package rf1

import (
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/cyclopcam/cyclops/pkg/cgogo"
)

// #include "rf1.h"
import "C"

// Writer is used to create rf1 files
type Writer struct {
	Tracks []*Track
}

// Track is one track of audio or video
type Track struct {
	IsVideo  bool      // else Audio
	Name     string    // Name of track - becomes part of filename
	TimeBase time.Time // All PTS times are relative to this
	Codec    string    // eg "H264"
	Width    int       // Only applicable to video
	Height   int       // Only applicable to video

	isWriting   bool     // else reading
	index       *os.File // Index file
	indexCount  int      // Number of index entries in file. Read once, when opening the track, and only used when reading
	packets     *os.File // Packets file
	packetsPos  int64    // End of last written byte inside packets file. Used when writing.
	packetsSize int64    // Size of packets file in bytes. Read once, when opening the track, and only used when reading
}

func MakeVideoTrack(name string, timeBase time.Time, codec string, width, height int) (*Track, error) {
	if !IsValidCodec(codec) {
		return nil, ErrInvalidCodec
	}
	if width < 1 || height < 1 {
		return nil, fmt.Errorf("Invalid video width/height (%v, %v)", width, height)
	}
	if !IsValidTrackName(name) {
		return nil, fmt.Errorf("Invalid track name: %v", name)
	}
	return &Track{
		IsVideo:  true,
		Name:     name,
		TimeBase: timeBase,
		Codec:    codec,
		Width:    width,
		Height:   height,
	}, nil
}

// Return true if the given name is a valid track name.
// Track names become part of filenames, so we impose restrictions on them.
func IsValidTrackName(name string) bool {
	if strings.ContainsAny(name, "/.\\#!@%^&*?<>|()") {
		return false
	}
	if path.Clean(name) != name {
		return false
	}
	return true
}

// Create a new rf1 file group writer.
// baseFilename is the base name of the file, eg "/home/user/recording-2024-01-01".
func NewWriter(baseFilename string, tracks []*Track) (*Writer, error) {
	for _, track := range tracks {
		if !IsValidCodec(track.Codec) {
			return nil, ErrInvalidCodec
		}
	}
	w := &Writer{}

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
		w.Tracks = append(w.Tracks, track)
	}

	return w, nil
}

func (w *Writer) Close() error {
	var firstErr error
	for _, t := range w.Tracks {
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

// Write NALUs
func (t *Track) WriteNALUs(nalus []NALU) error {
	index := []uint64{}

	// write packets
	for _, nalu := range nalus {
		pos := t.packetsPos
		n, err := t.packets.Write(nalu.Payload)
		t.packetsPos += int64(n)
		if err != nil {
			return err
		}
		index = append(index, MakeIndexNALU(EncodePTSTime(nalu.PTS, t.TimeBase), pos, nalu.Flags))
	}

	// write to index
	_, err := cgogo.WriteSlice(t.index, index)
	return err
}
