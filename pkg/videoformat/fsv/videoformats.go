package fsv

import (
	"errors"
	"time"

	"github.com/cyclopcam/cyclops/pkg/videoformat/rf1"
)

var ErrTrackNotFound = errors.New("Track not found")

// A video file type must support the VideoFormat interface in order to be
// used by fsv.
type VideoFormat interface {
	IsVideoFile(filename string) bool
	Open(filename string) (VideoFile, error)
	Create(filename string) (VideoFile, error)
	Delete(filename string, tracks []string) error
}

// Metadata about a track
type Track struct {
	Name      string
	StartTime time.Time
	Duration  time.Duration
	Width     int // Only applicable to video tracks
	Height    int // Only applicable to video tracks
}

// VideoFile is the analog of VideoFormat, but this is an embodied handle that can be read from and written to
type VideoFile interface {
	Close() error
	ListTracks() []Track

	// Return true if the video file can still grow larger to accept the given packets.
	// Note that even if your video file is capable of storing terabytes of data in a single file,
	// you should arbitrarily cap the size at something smaller, because the sweeper deletes whole
	// video files. If your entire video history is stored in one or two files, then the rolling
	// recorder would need to delete massive chunks of history whenever it needs more space.
	HasCapacity(trackName string, packets []rf1.NALU) bool

	// Create a new video track in the file.
	// You must do this before writing packets to the track.
	CreateVideoTrack(trackName string, timeBase time.Time, codec string, width, height int) error

	Write(trackName string, packets []rf1.NALU) error
	Read(trackName string, startTime, endTime time.Time, flags ReadFlags) ([]rf1.NALU, error)

	// Total size of the video file(s)
	Size() (int64, error)
}

// Find the time of the last packet in the video file, from any track
func VideoFileMaxTrackEndTime(vf VideoFile) time.Time {
	maxT := time.Time{}
	for _, t := range vf.ListTracks() {
		tp := t.StartTime.Add(t.Duration)
		if tp.After(maxT) {
			maxT = tp
		}
	}
	return maxT
}

// Returns true if the file has a video track with the given name, width and height
func VideoFileHasVideoTrack(vf VideoFile, trackName string, width, height int) bool {
	for _, t := range vf.ListTracks() {
		if t.Name == trackName {
			return t.Width == width && t.Height == height
		}
	}
	return false
}
