package fsv

import (
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"github.com/cyclopcam/cyclops/pkg/videoformat/rf1"
)

// Read packets from the archive.
// The map that is returned contains the tracks that were requested.
// If no packets are found, we return an empty map and a nil error.
func (a *Archive) Read(streamName string, trackNames []string, startTime, endTime time.Time) (map[string][]rf1.NALU, error) {
	a.streamsLock.Lock()
	stream := a.streams[streamName]
	a.streamsLock.Unlock()
	if stream == nil {
		return nil, fmt.Errorf("Stream not found: %v", streamName)
	}
	// Do a binary search inside the stream.files to find the file that contains the requested time period.
	// We'll use the file's start time as the key for the binary search.

	// Concrete example to aid in the logic here:
	// 110_video
	// 123_video
	// 136_video
	// 170_video

	// We find the first video with a start time AFTER 'startTime', and then
	// we reverse back by one.

	tracks := map[string][]rf1.NALU{}
	totalBytes := 0
	maxBytesPerRead := a.staticSettings.MaxBytesPerRead

	// Read the tracks from vf, and append them to our result set
	readFromVideoFile := func(filename string, vf VideoFile) error {
		for _, trackName := range trackNames {
			packets, err := vf.Read(trackName, startTime, endTime)
			if err != nil {
				return fmt.Errorf("Error reading track %v from video file %v: %v", trackName, filename, err)
			}
			tracks[trackName] = append(tracks[trackName], packets...)
			for _, p := range packets {
				totalBytes += len(p.Payload)
			}
		}
		if totalBytes > maxBytesPerRead {
			return fmt.Errorf("Read limit exceeded: %v bytes", maxBytesPerRead)
		}
		return nil
	}

	// Minimize the amount of time that we need to hold stream.contentLock.
	// The crucial thing to note here is that we only need the lock for the
	// the "stream.files" slice and "stream.current". So we make our calculations
	// on those objects, and then we can release the lock. When we go to read
	// from the files, we'll open the video files independently, thereby
	// relying on OS/filesystem concurrency.
	stream.contentLock.Lock()
	startIdx := sort.Search(len(stream.files), func(i int) bool {
		return stream.files[i].startTime >= startTime.UnixMilli()
	}) - 1
	startIdx = max(startIdx, 0)
	endIdx := sort.Search(len(stream.files), func(i int) bool {
		return stream.files[i].startTime >= endTime.UnixMilli()
	})
	indexFiles := stream.files[startIdx:endIdx]

	// We need to be conservative in our decision of whether to flush our write buffers. If the Read() is requesting
	// a portion of time that is close to the present, then it's very likely that we have buffered the writes that
	// the reader is interested in. In such a case, it's even likely that stream.current is nil. So if there is any
	// possibility that the reader is interested in buffered data, then we must first flush those buffers.
	if DoTimeRangesOverlap(stream.writeBufferMinPTS, stream.writeBufferMaxPTS, startTime, endTime) {
		a.flushWriteBufferForStream(stream)
	}

	var useCurrent *videoFile
	if stream.current != nil && DoTimeRangesOverlap(stream.current.startTime, stream.current.endTime, startTime, endTime) {
		useCurrent = stream.current
	}
	stream.contentLock.Unlock()

	// In this section, we have zero locks, so here during our most IO-heavy phase,
	// we have no concurrency problems. Multiple threads could be reading here
	// at the same time.
	for _, file := range indexFiles {
		if file.startTime > endTime.UnixMilli() {
			break
		}
		videoFilename := filepath.Join(a.streamDir(streamName), file.filename)
		videoFile, err := stream.format.Open(videoFilename)
		if err != nil {
			return nil, fmt.Errorf("Error opening video file %v: %v", videoFilename, err)
		}
		defer videoFile.Close()
		if err := readFromVideoFile(videoFilename, videoFile); err != nil {
			return nil, err
		}
	}

	// Here we need to take the contentLock again, before attempting to read from 'current'.
	// We need to manage two scenarios here:
	// 1. Current is still open
	// 2. Current has been closed
	// It is tempting to always reopen a new handle to 'current', but our rf1 files aren't
	// guaranteed to be in a consistent state if they're still being written to
	// (i.e. index could be written before payload). Because of this, we always use
	// our open handle for 'current'.
	if useCurrent != nil {
		stream.contentLock.Lock()
		defer stream.contentLock.Unlock()

		// If we have any buffered writes, flush them now
		a.flushWriteBufferForStream(stream)

		if useCurrent == stream.current {
			// Current is still the same open handle that we found at the start of the Read()
			if err := readFromVideoFile(stream.current.filename, stream.current.file); err != nil {
				return nil, err
			}
		} else {
			// Current got retired, so we need to open it from disk.
			videoFile, err := stream.format.Open(useCurrent.filename)
			if err != nil {
				return nil, fmt.Errorf("Error opening video file %v: %v", useCurrent.filename, err)
			}
			defer videoFile.Close()
			if err := readFromVideoFile(useCurrent.filename, videoFile); err != nil {
				return nil, err
			}
		}
	}

	return tracks, nil
}
