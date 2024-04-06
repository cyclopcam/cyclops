package fsv

import (
	"fmt"
	"path/filepath"
	"slices"
	"time"

	"github.com/cyclopcam/cyclops/pkg/videoformat/rf1"
)

// Write a payload to the archive.
// payload keys are track names.
// The payload must always include the exact same set of tracks, even if some of
// them have no new content to write. We use the set of tracks and their properties (eg width, height)
// to figure out when we need to close a file and open a new one. For example, if the user
// decides to enable HD recording, then the track composition would change. Such a change
// requires a new video file.
func (a *Archive) Write(streamName string, payload map[string]TrackPayload) error {
	err := a.write(streamName, payload)
	if err != nil {
		a.log.Errorf("Error writing to stream %v: %v", streamName, err)
	}
	return err
}

func (a *Archive) write(streamName string, payload map[string]TrackPayload) error {
	for track, payload := range payload {
		if payload.TrackType != rf1.TrackTypeVideo {
			return fmt.Errorf("Only video tracks have been implemented. Track %v has type: %v", track, payload.TrackType)
		}
	}

	// Find the earliest packet time.
	// We'll use this if we need to create a new video file.
	hasPackets := false
	minPTSMicro := int64(1<<63 - 1)
	maxPTSMicro := int64(0)
	for _, packets := range payload {
		if len(packets.NALUs) != 0 {
			hasPackets = true
			minPTSMicro = min(minPTSMicro, packets.NALUs[0].PTS.UnixMicro())
			maxPTSMicro = max(maxPTSMicro, packets.NALUs[len(packets.NALUs)-1].PTS.UnixMicro())
		}
	}
	if !hasPackets {
		// If we don't have any packets to write, then we can't create a new video file.
		// Since there are zero packets, this function call is anyway a no-op,
		// so no harm in just returning immediately.
		return nil
	}
	minPTS := time.UnixMicro(minPTSMicro)
	maxPTS := time.UnixMicro(maxPTSMicro)

	stream, err := a.getOrCreateStream(streamName)
	if err != nil {
		return err
	}

	// Ensure that the tracks in the video file are the same set of tracks that
	// the caller is trying to write. If the caller has altered the track composition,
	// then we create a new file.

	// This is a big lock, but there's no simple way around this. We don't want to introduce
	// multi-threaded access into our VideoFile interface - that would be insane.
	// I'm assuming that the write phase here will usually complete quickly, so that we don't
	// end up starving readers. Unless something bad is happening (eg running out of disk space),
	// writes here should complete very quickly, because they're just a copying of memory into
	// the disk cache.
	stream.contentLock.Lock()
	defer stream.contentLock.Unlock()

	if stream.current != nil {
		mustCloseReason := "" // If not empty, then we close
		for trackName, packets := range payload {
			if !VideoFileHasVideoTrack(stream.current.file, trackName, packets.VideoWidth, packets.VideoHeight) {
				mustCloseReason = fmt.Sprintf("Track %v does not exist or has different dimensions", trackName)
				break
			}
			if !stream.current.file.HasCapacity(trackName, packets.NALUs) {
				mustCloseReason = fmt.Sprintf("Insufficient capacity in for track %v", trackName)
				break
			}
			if len(packets.NALUs) > 0 {
				endPTS := packets.NALUs[len(packets.NALUs)-1].PTS
				duration := endPTS.Sub(stream.current.startTime)
				if duration > a.maxVideoFileDuration {
					mustCloseReason = fmt.Sprintf("File has reached max duration %v", a.maxVideoFileDuration)
					break
				}
			}
		}

		if mustCloseReason != "" {
			a.log.Infof("Closing video file %v: %v", stream.current.filename, mustCloseReason)
			currentSize, err := stream.current.file.Size()
			if err != nil {
				return fmt.Errorf("Error getting size of video file %v: %v", stream.current.filename, err)
			}
			// Add to index
			stream.files = append(stream.files, videoFileIndex{
				filename:  filepath.Base(stream.current.filename),
				startTime: stream.current.startTime.UnixMilli(),
				size:      currentSize,
				tracks:    stream.current.tracks,
			})
			err = stream.current.file.Close()
			if err != nil {
				a.log.Errorf("Error closing video file %v: %v", stream.current.filename, err)
			}
			stream.current = nil
		}
	}

	if stream.current == nil {
		// Create a new video file

		// But first, see if we've run out of disk space, and need to recycle some old files.

		// Filename has resolution of one millisecond.
		// I can't see a scenario where you will start/stop recording within 1ms.
		//
		// At present, unix time is 1708584695, which is 10 digits. We'd like to preserve
		// lexicographic ordering. Do we need to use 11 digits? Unix time will only roll over
		// to 11 digits on 2286-11-20 17:46:40. The world is going to look very different 262
		// years from now. Probably not worth thinking about.
		videoFilename := filepath.Join(a.streamDir(streamName), fmt.Sprintf("%v", minPTSMicro/1000))
		a.log.Infof("Creating new video file %v", videoFilename)
		file, err := stream.format.Create(videoFilename)
		if err != nil {
			return fmt.Errorf("Error creating video file %v: %v", videoFilename, err)
		}
		for track, payload := range payload {
			if err := file.CreateVideoTrack(track, minPTS, payload.Codec, payload.VideoWidth, payload.VideoHeight); err != nil {
				file.Close()
				return fmt.Errorf("Error creating video track %v in %v: %v", track, videoFilename, err)
			}
		}

		stream.current = &videoFile{
			filename:  videoFilename,
			file:      file,
			startTime: minPTS,
			endTime:   minPTS, // We haven't written to the stream yet, so start = end. We'll update endTime further down in this function.
		}
	}

	if minPTS.Before(stream.current.endTime) {
		// TODO:
		// Instead of discarding the packets, delete the prefix that overlaps.
		// This is a legitimate condition that occurs in the following circumstance:
		// 1. We detect something and start recording.
		// 2. Timeout lapses and we stop recording
		// 3. Another thing is detected, and we start recording. We include 15 seconds of history.
		// That 15 seconds of history overlaps with the previous recording.
		// This is not a bug.
		// We should take the first IDR of the new payload, and walk back in history to see if we can find
		// it in the video file. If we find it, then splice the new payload into the old payload.
		// If we don't find it, then I'm not really sure what to do.
		return fmt.Errorf("Video payload %v starts before the end of the current video file %v. This would cause non-contiguous frames.", minPTS, stream.current.endTime)
	}

	for track, packets := range payload {
		if err := stream.current.file.Write(track, packets.NALUs); err != nil {
			return fmt.Errorf("Error writing to video file %v: %v", stream.current.filename, err)
		}
		if !slices.Contains(stream.current.tracks, track) {
			stream.current.tracks = append(stream.current.tracks, track)
		}
	}

	stream.current.endTime = maxPTS

	if stream.startTime.IsZero() {
		stream.startTime = minPTS
	}
	stream.endTime = maxPTS

	return nil
}
