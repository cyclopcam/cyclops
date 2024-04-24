package fsv

import (
	"path/filepath"
	"time"

	"github.com/cyclopcam/cyclops/pkg/gen"
	"github.com/cyclopcam/cyclops/pkg/kibi"
)

func (a *Archive) startSweeper() {
	a.sweepStop = make(chan bool)
	a.sweeperStopped = make(chan bool)
	go a.sweeperThread()
}

// Stop the sweeper, and wait for it to exit
func (a *Archive) stopSweeper() {
	if a.sweepStop == nil || gen.IsChannelClosed(a.sweepStop) {
		a.log.Infof("Sweeper is already stopped")
	} else {
		a.log.Infof("Stopping sweeper")
		close(a.sweepStop)
		<-a.sweeperStopped
		a.log.Infof("Sweeper stopped")
	}
}

func (a *Archive) sweeperThread() {
	a.log.Infof("Sweeper thread starting")

	keepRunning := true
	for keepRunning {
		select {
		case <-a.sweepStop:
			keepRunning = false
		case <-time.After(a.staticSettings.SweepInterval):
			a.sweepIfNecessary()
		}
	}
	close(a.sweeperStopped)

	a.log.Infof("Sweeper thread exiting")
}

// Check if the archive is too large, and delete old files if necessary
func (a *Archive) sweepIfNecessary() {
	a.dynamicSettingsLock.Lock()
	maxArchiveSize := a.dynamicSettings.MaxArchiveSize
	a.dynamicSettingsLock.Unlock()

	if maxArchiveSize <= 0 {
		return
	}
	totalSize := a.TotalSize()
	maxSize := (maxArchiveSize * 99) / 100
	targetSize := (maxArchiveSize * 98) / 100
	if totalSize > maxSize {
		a.sweep(totalSize, targetSize)
	}
}

func (a *Archive) sweep(totalSize, targetSize int64) {
	initialSize := totalSize

	// Keep deleting the oldest file, until we're within budget
	for totalSize > targetSize {
		if gen.IsChannelClosed(a.sweepStop) {
			a.log.Infof("Sweep aborted because of shutdown request")
			break
		}

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

	a.log.Infof("Sweep finished. Dropped size from %v to %v (%v deleted)", kibi.FormatBytes(initialSize), kibi.FormatBytes(totalSize), kibi.FormatBytes(initialSize-totalSize))
}

func (a *Archive) deleteOldestFile(stream *videoStream) {
	stream.contentLock.Lock()
	defer stream.contentLock.Unlock()

	if len(stream.files) == 0 {
		return
	}

	absFilename := filepath.Join(a.streamDir(stream.name), stream.files[0].filename)
	a.log.Infof("Deleting oldest file %v from stream %v", absFilename, stream.name)

	if err := a.formats[0].Delete(absFilename, stream.files[0].tracks); err != nil {
		a.log.Errorf("Failed to delete video file %v: %v", absFilename, err)
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
		a.log.Warnf("Stream %v is now empty", stream.name)
		stream.startTime = time.Time{}
		stream.endTime = time.Time{}
	}
}
