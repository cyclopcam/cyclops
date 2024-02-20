package rf1

import (
	"os"
	"path/filepath"
	"strings"
)

// #include "rf1.h"
import "C"

// File is used to read and write rf1 files
type File struct {
	BaseFilename string
	Tracks       []*Track
}

// Create a new rf1 file group writer.
// baseFilename is the base name of the file, eg "/home/user/recording-2024-01-01".
func NewFile(baseFilename string, tracks []*Track) (*File, error) {
	for _, track := range tracks {
		if !IsValidCodec(track.Codec) {
			return nil, ErrInvalidCodec
		}
	}
	f := &File{
		BaseFilename: baseFilename,
	}

	for _, track := range tracks {
		if err := track.CreateTrackFiles(baseFilename); err != nil {
			f.Close()
			return nil, err
		}
		f.Tracks = append(f.Tracks, track)
	}

	return f, nil
}

// Open an existing rf1 file group
func Open(baseFilename string, mode OpenMode) (*File, error) {
	// Scan for tracks
	//dir, _ := filepath.Split(baseFilename)
	//matches, err := filepath.Glob(TrackFilename(dir+"/*", "*", FileTypeIndex))
	matches, err := filepath.Glob(TrackFilename(baseFilename, "*", FileTypeIndex))
	if err != nil {
		return nil, err
	}
	f := &File{
		BaseFilename: baseFilename,
	}

	for _, m := range matches {
		trackName := strings.TrimPrefix(m, baseFilename+"_")
		trackName = strings.TrimSuffix(trackName, ".rf1i")
		track, err := OpenTrack(baseFilename, trackName, mode)
		if err != nil {
			f.Close()
			return nil, err
		}
		f.Tracks = append(f.Tracks, track)
	}
	if len(f.Tracks) == 0 {
		return nil, os.ErrNotExist
	}
	return f, nil
}

func (f *File) Close() error {
	var firstErr error
	for _, t := range f.Tracks {
		if t.index != nil {
			err := t.index.Close()
			if firstErr == nil && err != nil {
				firstErr = err
			}
			t.index = nil
		}
		if t.packets != nil {
			err := t.packets.Close()
			if firstErr == nil && err != nil {
				firstErr = err
			}
			t.packets = nil
		}
	}
	return firstErr
}
