package fsv

import (
	"fmt"
	"path/filepath"
	"slices"
	"time"

	"github.com/cyclopcam/cyclops/pkg/videoformat/rf1"
)

// Usually this thread is not needed. The calls to Write() will end up causing the buffers
// to get flushed. However, if a caller stops calling Write() when there is still data in
// the write buffer, then that's where we come in. Without us, that data would sit in our
// buffer until the archive was closed.
func (a *Archive) writeBufferThread() {
	keepRunning := true
	for keepRunning {
		select {
		case <-a.shutdown:
			keepRunning = false
		case <-time.After(5 * time.Second):
			a.flushWriteBuffers(false)
		}
	}
	close(a.bufferWriterStopped)
}

// Write a payload to the archive.
// payload keys are track names.
// The payload must always include the exact same set of tracks, even if some of
// them have no new content to write. We use the set of tracks and their properties (eg width, height)
// to figure out when we need to close a file and open a new one. For example, if the user
// decides to enable HD recording, then the track composition would change. Such a change
// requires a new video file.
func (a *Archive) Write(streamName string, payload map[string]TrackPayload) error {
	var err error
	if a.isWriteBufferEnabled() {
		err = a.writeBuffered(streamName, payload)
	} else {
		err = a.writeOuter(streamName, payload)
	}
	if err != nil {
		a.log.Errorf("Error writing to stream %v: %v", streamName, err)
	}
	return err
}

func (a *Archive) isWriteBufferEnabled() bool {
	return a.staticSettings.MaxWriteBufferSize > 0 && a.staticSettings.MaxWriteBufferTime > 0
}

func (a *Archive) writeBuffered(streamName string, payload map[string]TrackPayload) error {
	stream, err := a.getOrCreateStream(streamName)
	if err != nil {
		return err
	}
	stream.contentLock.Lock()
	// Add to write buffer
	for track, packets := range payload {
		stream.writeBuffer[track] = append(stream.writeBuffer[track], packets)
		stream.writeBufferSize += int(totalPayloadBytes(packets.NALUs))
	}
	// Flush write buffer if necessary
	if a.mustFlushWriteBuffer(stream) {
		a.flushWriteBufferForStream(stream)
	}
	stream.contentLock.Unlock()
	return nil
}

// At this point, you must NOT be holding stream.contentLock.
func (a *Archive) writeOuter(streamName string, payload map[string]TrackPayload) error {
	stream, err := a.getOrCreateStream(streamName)
	if err != nil {
		return err
	}

	// This is a big lock, but there's no simple way around this. We don't want to introduce
	// multi-threaded access into our VideoFile interface - that would be insane.
	// I'm assuming that the write phase here will usually complete quickly, so that we don't
	// end up starving readers. Unless something bad is happening (eg running out of disk space),
	// writes here should complete very quickly, because they're just a copying of memory into
	// the disk cache.
	// Hmm AHEM! Writes do indeed become very "blocking" when writing to our
	// test HDD that is a USB external hard disk, NTFS formatted, attached to WSL.
	// And yes - I do have "Write Caching" enabled on the drive.
	// My workaround to this has been to drop frames inside VideoRecorder when it detects
	// that the channel from VideoRecorder to Archive is full.
	stream.contentLock.Lock()
	defer stream.contentLock.Unlock()

	return a.writeInner(stream, payload)
}

// At this point, you must be holding stream.contentLock.
func (a *Archive) writeInner(stream *videoStream, payload map[string]TrackPayload) error {
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

	// Ensure that the tracks in the video file are the same set of tracks that
	// the caller is trying to write. If the caller has altered the track composition,
	// then we create a new file.

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
		videoFilename := filepath.Join(a.streamDir(stream.name), fmt.Sprintf("%v", minPTSMicro/1000))
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

	for track, packets := range payload {
		startWrite := time.Now()

		// If any NALUs in 'packets' have already been written, remove them from the list
		afterSplice := a.splicePacketsBeforeWrite(stream, track, packets.NALUs)
		if len(afterSplice) == 0 {
			continue
		}

		// Write to file
		if err := stream.current.file.Write(track, afterSplice); err != nil {
			return fmt.Errorf("Error writing to video file %v: %v", stream.current.filename, err)
		}

		if !slices.Contains(stream.current.tracks, track) {
			stream.current.tracks = append(stream.current.tracks, track)
		}

		// Add the new packets to the list of recently written packets
		a.addPacketsToRecentWriteList(stream, track, afterSplice)

		// Record performance stats
		elapsed := time.Now().Sub(startWrite)
		a.bytesWrittenStat.AddSample(totalPayloadBytes(afterSplice))
		if a.firstWrite.IsZero() {
			a.firstWrite = startWrite
		}
		a.writeTimeStat.AddSample(elapsed)
	}

	stream.current.endTime = maxPTS

	if stream.startTime.IsZero() {
		stream.startTime = minPTS
	}
	stream.endTime = maxPTS

	// Write stats to log, if appropriate interval has elapsed
	a.AutoStatsToLog()

	return nil
}

func (a *Archive) mustFlushWriteBuffer(stream *videoStream) bool {
	now := time.Now()
	for _, buffer := range stream.writeBuffer {
		var age time.Duration
		if len(buffer) != 0 {
			age = now.Sub(buffer[0].NALUs[0].PTS)
		}
		if age > a.staticSettings.MaxWriteBufferTime || stream.writeBufferSize > a.staticSettings.MaxWriteBufferSize {
			return true
		}
	}
	return false
}

// If necessary, flush the write buffer for the stream.
// You must be holding the stream.contentLock before calling this function.
func (a *Archive) flushWriteBufferForStream(stream *videoStream) {
	for track, payloadList := range stream.writeBuffer {
		// Merge payloads together, so that we can reduce the number of OS write calls,
		// and also the number of calls to our 'write' function, which is quite involved.
		merged := payloadList[0]
		for i := 1; i <= len(payloadList); i++ {
			if i < len(payloadList) && merged.EqualStructure(&payloadList[i]) {
				merged.NALUs = append(merged.NALUs, payloadList[i].NALUs...)
			} else {
				if err := a.writeInner(stream, map[string]TrackPayload{track: merged}); err != nil {
					a.log.Errorf("Error flushing write buffer for stream %v (%v/%v): %v", stream.name, i, len(payloadList), err)
				}
				if i < len(payloadList) {
					merged = payloadList[i]
				}
			}
		}
	}
	stream.writeBuffer = map[string][]TrackPayload{}
	stream.writeBufferSize = 0
}

func (a *Archive) flushWriteBuffers(force bool) {
	a.streamsLock.Lock()
	streams := make([]*videoStream, 0, len(a.streams))
	for _, stream := range a.streams {
		streams = append(streams, stream)
	}
	a.streamsLock.Unlock()

	for _, stream := range streams {
		stream.contentLock.Lock()
		if force || a.mustFlushWriteBuffer(stream) {
			a.flushWriteBufferForStream(stream)
		}
		stream.contentLock.Unlock()
	}
}
