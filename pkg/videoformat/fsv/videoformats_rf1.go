package fsv

import (
	"fmt"
	"os"
	"time"

	"github.com/cyclopcam/cyclops/pkg/videoformat/rf1"
)

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
	var firstError error
	for _, track := range tracks {
		err := os.Remove(rf1.TrackFilename(filename, track, rf1.FileTypeIndex))
		if firstError == nil && err != nil {
			firstError = err
		}
		err = os.Remove(rf1.TrackFilename(filename, track, rf1.FileTypePackets))
		if firstError == nil && err != nil {
			firstError = err
		}
	}
	return firstError
}

/////////////////////////////////////////////////////////////////////////////////

type VideoFileRF1 struct {
	File *rf1.File
}

func (v *VideoFileRF1) Close() error {
	return v.File.Close()
}

func (v *VideoFileRF1) ListTracks() map[string]Track {
	tracks := map[string]Track{}
	for _, t := range v.File.Tracks {
		tracks[t.Name] = Track{
			Name:      t.Name,
			StartTime: t.TimeBase,
			Duration:  t.Duration(),
			Codec:     t.Codec,
			Width:     t.Width,
			Height:    t.Height,
		}
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

func (v *VideoFileRF1) Read(trackName string, startTime, endTime time.Time, flags ReadFlags) ([]rf1.NALU, error) {
	var rf1Flags rf1.PacketReadFlags
	if flags&ReadFlagSeekBackToKeyFrame != 0 {
		rf1Flags |= rf1.PacketReadFlagSeekBackToKeyFrame
	}
	for _, track := range v.File.Tracks {
		if track.Name == trackName {
			return track.ReadAtTime(startTime.Sub(track.TimeBase), endTime.Sub(track.TimeBase), rf1Flags)
		}
	}
	return nil, fmt.Errorf("%w: '%v'", ErrTrackNotFound, trackName)
}

func (v *VideoFileRF1) Size() (int64, error) {
	sum := int64(0)
	for _, track := range v.File.Tracks {
		// Use logical (truncated) file sizes instead of the on-disk file size, which might
		// be substantially inflated due to anti-fragmentation pre-allocation.
		sum += track.PacketFileSize() + track.IndexFileSize()
		//f1, f2 := track.Filenames(v.File.BaseFilename)
		//s1, err := os.Stat(f1)
		//if err != nil {
		//	return 0, err
		//}
		//s2, err := os.Stat(f2)
		//if err != nil {
		//	return 0, err
		//}
		//sum += s1.Size() + s2.Size()
	}
	return sum, nil
}

/////////////////////////////////////////////////////////////////////////////////
