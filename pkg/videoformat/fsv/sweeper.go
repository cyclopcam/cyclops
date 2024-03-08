package fsv

import (
	"time"

	"github.com/cyclopcam/cyclops/pkg/kibi"
)

func (a *Archive) StartSweeper() {
	a.sweepStop = make(chan bool)
	go a.sweeperThread()
}

func (a *Archive) StopSweeper() {
	close(a.sweepStop)
}

func (a *Archive) sweeperThread() {
	for {
		select {
		case <-a.sweepStop:
			return
		case <-time.After(a.settings.SweepInterval):
			a.sweepIfNecessary()
		}
	}
}

// Check if the archive is too large, and delete old files if necessary
func (a *Archive) sweepIfNecessary() {
	if a.settings.MaxArchiveSize <= 0 {
		return
	}
	totalSize := a.TotalSize()
	maxSize := (a.settings.MaxArchiveSize * 99) / 100
	targetSize := (a.settings.MaxArchiveSize * 98) / 100
	if totalSize > maxSize {
		a.sweep(totalSize, targetSize)
	}
}

func (a *Archive) sweep(totalSize, targetSize int64) {
	initialSize := totalSize

	// Keep deleting the oldest file, until we're within budget
	for totalSize > targetSize {
		a.streamsLock.Lock()
		var oldestStream *videoStream
		oldestFile := videoFileIndex{
			startTime: int64(1 << 62),
		}
		deleteStream := []string{}
		for _, stream := range a.streams {
			stream.contentLock.Lock()
			if len(stream.files) > 0 {
				if stream.files[0].startTime < oldestFile.startTime {
					oldestStream = stream
					oldestFile = stream.files[0]
				}
			} else {
				deleteStream = append(deleteStream, stream.name)
			}
			stream.contentLock.Unlock()
		}
		for _, del := range deleteStream {
			a.deleteEmptyStreamHaveLock(del)
		}
		a.streamsLock.Unlock()

		if oldestStream == nil {
			a.log.Errorf("Sweep failed to find any more files to delete. Total size: %v, target size: %v", totalSize, targetSize)
			break
		}

		totalSize -= oldestFile.size
		a.deleteOldestFile(oldestStream)
	}

	a.log.Infof("Sweep finished. Dropped size from %v to %v (%v deleted)", kibi.Bytes(initialSize), kibi.Bytes(totalSize), kibi.Bytes(initialSize-totalSize))
}

func (a *Archive) deleteOldestFile(stream *videoStream) {
	stream.contentLock.Lock()
	defer stream.contentLock.Unlock()

	if len(stream.files) == 0 {
		return
	}

	if err := a.formats[0].Delete(stream.files[0].filename, stream.files[0].tracks); err != nil {
		a.log.Errorf("Failed to delete video file %v: %v", stream.files[0].filename, err)
	}

	if cap(stream.files) > len(stream.files)*2 {
		// The slice's underlying array is growing large, so make a new array
		newFiles := make([]videoFileIndex, len(stream.files)-1)
		copy(newFiles, stream.files[1:])
		stream.files = newFiles
	} else {
		stream.files = stream.files[1:]
	}

	// Update startTime of stream
	if len(stream.files) > 0 {
		stream.startTime = time.UnixMilli(stream.files[0].startTime)
	} else if stream.current != nil {
		stream.startTime = stream.current.startTime
	} else {
		// We should maybe delete the stream
		a.log.Warnf("Stream %v is now empty", stream.name)
		stream.startTime = time.Time{}
		stream.endTime = time.Time{}
	}
}
