package fsv

import (
	"errors"
	"fmt"
	"os"
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

// VideoFile is the analog of VideoFormat, but an embodied handle that can be read from and written to
type VideoFile interface {
	Close() error
	ListTracks() []Track
	HasCapacity(trackName string, packets []rf1.NALU) bool
	CreateVideoTrack(trackName string, timeBase time.Time, codec string, width, height int) error
	Write(trackName string, packets []rf1.NALU) error
	Read(trackName string, startTime, endTime time.Time) ([]rf1.NALU, error)
	Size() (int64, error) // Total size of the video file(s)
}

type VideoFormatRF1 struct {
}

func (f *VideoFormatRF1) IsVideoFile(filename string) bool {
	return rf1.IsVideoFile(filename)
}

func (f *VideoFormatRF1) Open(filename string) (VideoFile, error) {
	vf, err := rf1.Open(filename, rf1.OpenModeReadOnly)
	if err != nil {
		return nil, err
	}
	return &VideoFileRF1{File: vf}, nil
}

func (f *VideoFormatRF1) Create(filename string) (VideoFile, error) {
	vf, err := rf1.Create(filename, nil)
	if err != nil {
		return nil, err
	}
	return &VideoFileRF1{File: vf}, nil
}

func (f *VideoFormatRF1) Delete(filename string, tracks []string) error {
	for _, track := range tracks {
		err := os.Remove(rf1.TrackFilename(filename, track, rf1.FileTypeIndex))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

/////////////////////////////////////////////////////////////////////////////////

type VideoFileRF1 struct {
	File *rf1.File
}

func (v *VideoFileRF1) Close() error {
	return v.File.Close()
}

func (v *VideoFileRF1) ListTracks() []Track {
	tracks := make([]Track, 0, len(v.File.Tracks))
	for _, t := range v.File.Tracks {
		tracks = append(tracks, Track{
			Name:      t.Name,
			StartTime: t.TimeBase,
			Duration:  t.Duration(),
			Width:     t.Width,
			Height:    t.Height,
		})
	}
	return tracks
}

func (v *VideoFileRF1) HasCapacity(trackName string, packets []rf1.NALU) bool {
	for _, track := range v.File.Tracks {
		if track.Name == trackName {
			return track.HasCapacity(packets)
		}
	}
	// Caller will likely try to create a new file, so it's OK not to return an error, but just return false
	return false
}

func (v *VideoFileRF1) CreateVideoTrack(trackName string, timeBase time.Time, codec string, width, height int) error {
	t, err := rf1.MakeVideoTrack(trackName, timeBase, codec, width, height)
	if err != nil {
		return err
	}
	return v.File.AddTrack(t)
}

func (v *VideoFileRF1) Write(trackName string, packets []rf1.NALU) error {
	for _, track := range v.File.Tracks {
		if track.Name == trackName {
			return track.WriteNALUs(packets)
		}
	}
	return fmt.Errorf("%w: '%v'", ErrTrackNotFound, trackName)
}

func (v *VideoFileRF1) Read(trackName string, startTime, endTime time.Time) ([]rf1.NALU, error) {
	for _, track := range v.File.Tracks {
		if track.Name == trackName {
			return track.ReadAtTime(startTime.Sub(track.TimeBase), endTime.Sub(track.TimeBase))
		}
	}
	return nil, fmt.Errorf("%w: '%v'", ErrTrackNotFound, trackName)
}

func (v *VideoFileRF1) Size() (int64, error) {
	sum := int64(0)
	for _, track := range v.File.Tracks {
		f1, f2 := track.Filenames(v.File.BaseFilename)
		s1, err := os.Stat(f1)
		if err != nil {
			return 0, err
		}
		s2, err := os.Stat(f2)
		if err != nil {
			return 0, err
		}
		sum += s1.Size() + s2.Size()
	}
	return sum, nil
}

/////////////////////////////////////////////////////////////////////////////////

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

func VideoFileHasVideoTrack(vf VideoFile, trackName string, width, height int) bool {
	for _, t := range vf.ListTracks() {
		if t.Name == trackName {
			return t.Width == width && t.Height == height
		}
	}
	return false
}
