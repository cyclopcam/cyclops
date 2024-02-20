package rf1

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/cyclopcam/cyclops/pkg/cgogo"
)

// #include "rf1.h"
import "C"

// File is used to read and write rf1 files
type File struct {
	Tracks []*Track
}

// Create a new rf1 file group writer.
// baseFilename is the base name of the file, eg "/home/user/recording-2024-01-01".
func NewFile(baseFilename string, tracks []*Track) (*File, error) {
	for _, track := range tracks {
		if !IsValidCodec(track.Codec) {
			return nil, ErrInvalidCodec
		}
	}
	f := &File{}

	for _, track := range tracks {
		idx, err := os.Create(TrackFilename(baseFilename, track.Name, FileTypeIndex))
		if err != nil {
			return nil, err
		}
		pkt, err := os.Create(TrackFilename(baseFilename, track.Name, FileTypePackets))
		if err != nil {
			idx.Close()
			return nil, err
		}

		if track.IsVideo {
			header := C.VideoIndexHeader{}
			header.TimeBase = C.uint64_t(EncodeTimeBase(track.TimeBase))
			cgogo.CopySlice(header.Magic[:], []byte(MagicVideoTrackBytes))
			cgogo.CopySlice(header.Codec[:], []byte(track.Codec))
			header.Width = C.uint16_t(track.Width)
			header.Height = C.uint16_t(track.Height)
			//if _, err := idx.Write(unsafe.Slice((*byte)(unsafe.Pointer(&header)), unsafe.Sizeof(header))); err != nil {
			if _, err := cgogo.WriteStruct(idx, &header); err != nil {
				idx.Close()
				return nil, err
			}
		} else {
			header := C.AudioIndexHeader{}
			header.TimeBase = C.uint64_t(EncodeTimeBase(track.TimeBase))
			cgogo.CopySlice(header.Magic[:], []byte(MagicAudioTrackBytes))
			cgogo.CopySlice(header.Codec[:], []byte(track.Codec))
			//if _, err := idx.Write(unsafe.Slice((*byte)(unsafe.Pointer(&header)), unsafe.Sizeof(header))); err != nil {
			if _, err := cgogo.WriteStruct(idx, &header); err != nil {
				idx.Close()
				return nil, err
			}
		}

		track.index = idx
		track.packets = pkt
		track.packetsPos = 0
		track.isWriting = true
		f.Tracks = append(f.Tracks, track)
	}

	return f, nil
}

// Open an rf1 file group for reading
func Open(baseFilename string) (*File, error) {
	// Scan for tracks
	//dir, _ := filepath.Split(baseFilename)
	//matches, err := filepath.Glob(TrackFilename(dir+"/*", "*", FileTypeIndex))
	matches, err := filepath.Glob(TrackFilename(baseFilename, "*", FileTypeIndex))
	if err != nil {
		return nil, err
	}

	tracks := make([]*Track, 0)
	for _, m := range matches {
		trackName := strings.TrimPrefix(m, baseFilename+"_")
		trackName = strings.TrimSuffix(trackName, ".rf1i")
		track, err := OpenTrack(baseFilename, trackName)
		if err != nil {
			if os.IsNotExist(err) {
				break
			}
			return nil, err
		}
		tracks = append(tracks, track)
	}
	if len(tracks) == 0 {
		return nil, os.ErrNotExist
	}
	return &File{Tracks: tracks}, nil
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
