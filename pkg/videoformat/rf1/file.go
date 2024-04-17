package rf1

import (
	"fmt"
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
// tracks may be empty/nil
func Create(baseFilename string, tracks []*Track) (*File, error) {
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
// filename may be either a base filename such as `/foo/bar/myvideo` or
// a concrete track filename such as `/foo/bar/myvideo_mytrack.rf1i`
func Open(filename string, mode OpenMode) (*File, error) {
	// Scan for tracks
	baseFilename := filename
	if strings.HasSuffix(filename, Extension(FileTypeIndex)) {
		hasTrackSeparator := false
		baseFilename = strings.TrimSuffix(filename, Extension(FileTypeIndex))
		baseFilename, _, hasTrackSeparator = strings.Cut(baseFilename, "_")
		if !hasTrackSeparator {
			return nil, fmt.Errorf("Invalid filename (no track name specified): %v", filename)
		}
	}
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

// Create the structure for the new empty track on disk
func (f *File) AddTrack(track *Track) error {
	if err := track.CreateTrackFiles(f.BaseFilename); err != nil {
		return err
	}
	f.Tracks = append(f.Tracks, track)
	return nil
}

func (f *File) Close() error {
	var firstErr error
	for _, t := range f.Tracks {
		if err := t.Close(); firstErr == nil && err != nil {
			firstErr = err
		}
	}
	return firstErr
}
