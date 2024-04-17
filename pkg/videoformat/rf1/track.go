package rf1

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"
	"unsafe"

	"github.com/cyclopcam/cyclops/pkg/cgogo"
)

// #include "rf1.h"
import "C"

const IndexHeaderSize = int(unsafe.Sizeof(C.CommonIndexHeader{}))

var ErrReadOnly = fmt.Errorf("track is read-only")

type TrackType int

const (
	TrackTypeVideo TrackType = iota
	TrackTypeAudio
)

type OpenMode int

const (
	OpenModeReadOnly OpenMode = iota
	OpenModeReadWrite
)

// Track is one track of audio or video
type Track struct {
	Type     TrackType // Audio or Video
	Name     string    // Name of track - becomes part of filename
	TimeBase time.Time // All PTS times are relative to this
	Codec    string    // eg "H264"
	Width    int       // Only applicable to video
	Height   int       // Only applicable to video

	canWrite    bool          // True if opened with write ability
	index       *os.File      // Index file
	indexCount  int           // Number of index entries in file, excluding the sentinel
	dirty       bool          // True if we need to write our index header and truncate files on Close()
	packets     *os.File      // Packets file
	packetsSize int64         // Size of packets file in bytes (real used space, ignoring pre-allocated space)
	duration    time.Duration // Duration of track
	indexCache  []uint64      // Cache of all index entries, including sentinel

	disablePreallocate  bool  // Disable preallocate of space in index and packet files to avoid fragmentation
	indexPreallocSize   int64 // Size that we have pre-extended index file to (zero if no extension)
	packetsPreallocSize int64 // Size that we have pre-extended packets file to (zero if no extension)
}

// Create a new track definition, but do not write anything to disk, or associate the track with a file.
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
		canWrite: true,
		Type:     TrackTypeVideo,
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

// Return [index file, packet file] paths
func (t *Track) Filenames(baseFilename string) (string, string) {
	return TrackFilename(baseFilename, t.Name, FileTypeIndex), TrackFilename(baseFilename, t.Name, FileTypePackets)
}

// Returns the truncated size of the packet file
func (t *Track) PacketFileSize() int64 {
	return t.packetsSize
}

// Returns the truncated size of the index file
func (t *Track) IndexFileSize() int64 {
	return int64(IndexHeaderSize) + int64(t.indexCount+1)*8
}

// Create new track files on disk
// You will usually not call this function directly. Instead, it is called
// for you when using [File.AddTrack].
func (t *Track) CreateTrackFiles(baseFilename string) error {
	idx, err := os.Create(TrackFilename(baseFilename, t.Name, FileTypeIndex))
	if err != nil {
		return err
	}
	pkt, err := os.Create(TrackFilename(baseFilename, t.Name, FileTypePackets))
	if err != nil {
		idx.Close()
		return err
	}

	t.canWrite = true
	t.index = idx
	t.indexCount = 0
	t.packets = pkt
	t.packetsSize = 0
	t.duration = 0
	t.indexCache = nil

	if err := t.WriteHeader(); err != nil {
		t.Close()
		return err
	}

	// We always have a sentinel entry, even if the file is empty.
	// This simplifies the code and data structures.
	sentinel := []uint64{MakeIndexSentinel(0)}
	cgogo.WriteSliceAt(t.index, sentinel, int64(IndexHeaderSize))

	return nil
}

// Open a track for reading/writing
// If OpenMode is OpenModeReadOnly, then we open the files with O_RDONLY.
// If OpenMode is OpenModeReadWrite is true, and we can't open the file with O_RDWR, then the function fails.
func OpenTrack(baseFilename string, trackName string, mode OpenMode) (*Track, error) {
	var idxFile *os.File
	var pktFile *os.File
	var err error
	success := false
	defer func() {
		if !success {
			if idxFile != nil {
				idxFile.Close()
			}
			if pktFile != nil {
				pktFile.Close()
			}
		}
	}()

	flag := os.O_RDONLY
	if mode == OpenModeReadWrite {
		flag = os.O_RDWR
	}

	idxFile, err = os.OpenFile(TrackFilename(baseFilename, trackName, FileTypeIndex), flag, 0660)
	if err != nil {
		return nil, err
	}
	pktFile, err = os.OpenFile(TrackFilename(baseFilename, trackName, FileTypePackets), flag, 0660)
	if err != nil {
		return nil, err
	}

	indexHead := C.CommonIndexHeader{}
	if _, err = cgogo.ReadStruct(idxFile, &indexHead); err != nil {
		return nil, err
	}
	magic := [4]byte{}
	codec := [4]byte{}
	cgogo.CopySlice(magic[:], indexHead.Magic[:])
	cgogo.CopySlice(codec[:], indexHead.Codec[:])
	trackType := TrackTypeVideo
	if bytes.Equal(magic[:], []byte(MagicAudioTrackBytes)) {
		trackType = TrackTypeAudio
	} else if bytes.Equal(magic[:], []byte(MagicVideoTrackBytes)) {
		trackType = TrackTypeVideo
	} else {
		return nil, fmt.Errorf("Unrecognized magic bytes in index track: %02x %02x %02x %02x", magic[0], magic[1], magic[2], magic[3])
	}

	if !IsValidCodec(string(codec[:])) {
		return nil, fmt.Errorf("%w '%v'", ErrInvalidCodec, string(codec[:]))
	}

	realIndexFileSize, err := idxFile.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, err
	}

	indexCount := int(indexHead.IndexCount)
	sentinelPos := int64(0)

	var indexCache []uint64
	if indexCount == 0 {
		// If the index count is zero, then we need to scan index entries to find the last non-zero entry.
		// This happens if the file was not closed properly (eg hardware/OS/program crash).
		indexCache, err = findAllNonZeroIndexEntries(idxFile)
		if err != nil {
			return nil, err
		}
		if len(indexCache) == 0 {
			indexCache = []uint64{MakeIndexSentinel(0)}
		}
		// indexCache includs the sentinel, but indexCount excludes the sentinel
		indexCount = max(0, len(indexCache)-1)
		sentinelPos = SplitIndexNALULocationOnly(indexCache[len(indexCache)-1])
	} else {
		// Read only the sentinel
		sentinel := []uint64{0}
		if _, err := cgogo.ReadSliceAt(idxFile, sentinel, int64(IndexHeaderSize)+int64(indexCount)*8); err != nil {
			return nil, fmt.Errorf("Error reading sentinel: %w", err)
		}
		sentinelPos = SplitIndexNALULocationOnly(sentinel[0])
	}

	realPacketFileSize, err := pktFile.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, err
	}

	// Limit the packet file size to the last index entry (sentinel).
	// This is useful when the packet file did not get truncated (eg system crashed).
	pktBytes := min(realPacketFileSize, sentinelPos)

	// The following deals with the case where the packet file was unnaturally truncated,
	// and the index file has entries pointing into non-existent regions of the packet file.
	// We truncate such index entries.
	// I don't have the energy to test this now, so rather leaving it out.
	//if pktSize < sentinelPos {
	//	i := len(indexCache) - 1
	//	for ; i >= 0; i-- {
	//		if pktSize <= SplitIndexNALULocationOnly(indexCache[i]) {
	//			break
	//		}
	//	}
	//	// i is the last valid index entry
	//	i = max(i, 0)
	//	if i == 0 {
	//		// empty
	//		indexCache = []uint64{MakeIndexSentinel(0)}
	//	} else {
	//		indexCache = indexCache[:i]
	//	}
	//	indexCount = len(indexCache) - 1
	//}

	track := &Track{
		canWrite:            mode == OpenModeReadWrite,
		Type:                trackType,
		Name:                trackName,
		Codec:               string(codec[:]),
		TimeBase:            DecodeTimeBase(uint64(indexHead.TimeBase)),
		index:               idxFile,
		indexCount:          indexCount,
		indexCache:          indexCache,
		packets:             pktFile,
		packetsSize:         pktBytes,
		indexPreallocSize:   realIndexFileSize,
		packetsPreallocSize: realPacketFileSize,
	}

	if trackType == TrackTypeVideo {
		videoHead := C.VideoIndexHeader{}
		commonHeadBytes := unsafe.Slice((*byte)(unsafe.Pointer(&indexHead)), int(unsafe.Sizeof(indexHead)))
		videoHeadBytes := unsafe.Slice((*byte)(unsafe.Pointer(&videoHead)), int(unsafe.Sizeof(videoHead)))
		copy(videoHeadBytes, commonHeadBytes)
		track.Width = int(videoHead.Width)
		track.Height = int(videoHead.Height)
	}

	track.duration, err = track.readDuration()
	if err != nil {
		return nil, err
	}

	success = true
	return track, nil
}

// Returns the number of NALUs
func (t *Track) Count() int {
	return t.indexCount
}

// Returns the duration of the track
func (t *Track) Duration() time.Duration {
	return t.duration
}

// Return the valid index entries, including the sentinel
func findAllNonZeroIndexEntries(idxFile *os.File) ([]uint64, error) {
	idxSize, err := idxFile.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, err
	}

	// Maximum number of index entries
	maxCount := (int(idxSize) - IndexHeaderSize) / 8

	// Read the whole index
	raw := make([]uint64, maxCount)
	_, err = cgogo.ReadSliceAt(idxFile, raw, int64(IndexHeaderSize))
	if err != nil {
		return nil, err
	}

	// Find the first zero index entry.
	firstZeroIdx := 0
	if maxCount >= 2 && raw[0] == 0 && raw[1] != 0 {
		// We make special allowance for packet[0] to be all zeroes.
		// This is allowable for the first packet, but not for any others.
		// The first packet will have a Location field of zero, likely
		// a Time (PTS) field of zero, and possibly zero flags.
		firstZeroIdx = 1
	}
	for ; firstZeroIdx < maxCount; firstZeroIdx++ {
		if raw[firstZeroIdx] == 0 {
			break
		}
	}

	return raw[:firstZeroIdx], nil
}
