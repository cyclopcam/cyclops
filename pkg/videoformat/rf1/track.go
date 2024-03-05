package rf1

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
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
	indexCount  int           // Number of index entries in file
	packets     *os.File      // Packets file
	packetsSize int64         // Size of packets file in bytes
	duration    time.Duration // Duration of track
	indexCache  []uint64      // Cache of index entries
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

	if t.Type == TrackTypeVideo {
		header := C.VideoIndexHeader{}
		header.TimeBase = C.uint64_t(EncodeTimeBase(t.TimeBase))
		cgogo.CopySlice(header.Magic[:], []byte(MagicVideoTrackBytes))
		cgogo.CopySlice(header.Codec[:], []byte(t.Codec))
		header.Width = C.uint16_t(t.Width)
		header.Height = C.uint16_t(t.Height)
		if _, err := cgogo.WriteStruct(idx, &header); err != nil {
			idx.Close()
			return err
		}
	} else if t.Type == TrackTypeAudio {
		header := C.AudioIndexHeader{}
		header.TimeBase = C.uint64_t(EncodeTimeBase(t.TimeBase))
		cgogo.CopySlice(header.Magic[:], []byte(MagicAudioTrackBytes))
		cgogo.CopySlice(header.Codec[:], []byte(t.Codec))
		if _, err := cgogo.WriteStruct(idx, &header); err != nil {
			idx.Close()
			return err
		}
	} else {
		return fmt.Errorf("Invalid track type: %v", t.Type)
	}

	t.canWrite = true
	t.index = idx
	t.indexCount = 0
	t.packets = pkt
	t.packetsSize = 0
	t.duration = 0
	t.indexCache = nil

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

	idxFile, err = os.OpenFile(TrackFilename(baseFilename, trackName, FileTypeIndex), flag, 0666)
	if err != nil {
		return nil, err
	}
	pktFile, err = os.OpenFile(TrackFilename(baseFilename, trackName, FileTypePackets), flag, 0666)
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

	// Seek to end of files, so that we can continue to write.
	// But more importantly in most cases - this gives us the file size.
	// I don't see opening of an existing file and appending to it as a common use case.
	idxSize, err := idxFile.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, err
	}
	indexCount := (int(idxSize) - IndexHeaderSize) / 8

	pktSize, err := pktFile.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, err
	}

	track := &Track{
		canWrite:    mode == OpenModeReadWrite,
		Type:        trackType,
		Name:        trackName,
		Codec:       string(codec[:]),
		TimeBase:    DecodeTimeBase(uint64(indexHead.TimeBase)),
		index:       idxFile,
		indexCount:  indexCount,
		packets:     pktFile,
		packetsSize: pktSize,
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

// Use the PTS of the last NALU in the index to figure out the duration of the track
func (t *Track) readDuration() (time.Duration, error) {
	if t.indexCount == 0 {
		return 0, nil
	}
	nalus, err := t.ReadIndex(t.indexCount-1, t.indexCount)
	if err != nil {
		return 0, err
	}
	return nalus[0].PTS.Sub(t.TimeBase), nil
}

// Read a range of packet uint64 index entries, either from the file, or from our in-memory cache.
// At present out caching strategy is simply to read the entire index the first time
// we need any of it, and keep the whole thing in memory. The index is so small compared to
// the payload, that it's not clear that anything else makes sense.
func (t *Track) readRawIndex(startIdx, endIdx int) ([]uint64, error) {
	if startIdx < 0 || endIdx <= startIdx || endIdx > t.indexCount {
		return nil, fmt.Errorf("Invalid startIdx, endIdx: %v, %v (number of indices: %v)", startIdx, endIdx, t.indexCount)
	}
	if len(t.indexCache) != t.indexCount {
		// Special exceptions for when we don't cache the entire index:
		// 1. When reading the final index entry.
		//    This is used to determine the duration of the track, and does not necessary imply
		//    subsequent reading.
		doNotCache := startIdx == t.indexCount-1
		if doNotCache {
			// Read only what was requested
			raw := make([]uint64, endIdx-startIdx)
			_, err := cgogo.ReadSliceAt(t.index, raw, int64(IndexHeaderSize)+int64(startIdx)*8)
			if err != nil {
				return nil, err
			}
			return raw, nil
		} else {
			// Read the whole index into the cache
			raw := make([]uint64, t.indexCount)
			_, err := cgogo.ReadSliceAt(t.index, raw, int64(IndexHeaderSize))
			if err != nil {
				return nil, err
			}
			t.indexCache = raw
		}
	}
	return t.indexCache[startIdx:endIdx], nil
}

// Read NALU index in the range [startIdx, endIdx).
func (t *Track) ReadIndex(startIdx, endIdx int) ([]NALU, error) {
	if startIdx < 0 || endIdx <= startIdx || endIdx > t.indexCount {
		return nil, fmt.Errorf("Invalid startIdx, endIdx: %v, %v (number of indices: %v)", startIdx, endIdx, t.indexCount)
	}
	// If plusOne is true, then we will read one more NALU than the user requested.
	// If plusOne is false, we're reading up to the end of the file, so we need to
	// use the packets file size to determine the size of the final NALU.
	plusOne := endIdx < t.indexCount
	readCount := endIdx - startIdx
	if plusOne {
		readCount++
	}
	raw, err := t.readRawIndex(startIdx, startIdx+readCount)
	if err != nil {
		return nil, err
	}
	nalus := make([]NALU, readCount)
	for i, r := range raw {
		pts, location, flags := SplitIndexNALU(r)
		nalus[i].PTS = DecodePTSTime(pts, t.TimeBase)
		nalus[i].Flags = flags
		nalus[i].Position = int64(location)
	}
	for i := 0; i < len(nalus)-1; i++ {
		nalus[i].Length = nalus[i+1].Position - nalus[i].Position
	}
	if plusOne {
		// Chop off the final NALU that we artifically added in for Length computation
		nalus = nalus[:len(nalus)-1]
	} else {
		// Compute the length of the final NALU by using the size of the packets file
		nalus[len(nalus)-1].Length = t.packetsSize - nalus[len(nalus)-1].Position
	}
	return nalus, nil
}

// Read payloads for the given NALUs
func (t *Track) ReadPayload(nalus []NALU) error {
	if len(nalus) == 0 {
		return nil
	}
	// Read in contiguous chunks
	maxChunkSize := int64(1024 * 1024)
	startByte := nalus[0].Position
	endByte := nalus[0].Position + nalus[0].Length
	startIdx := 0
	for i := 1; i <= len(nalus); i++ {
		if i == len(nalus) || nalus[i].Position != endByte || endByte+nalus[i].Length-startByte >= maxChunkSize {
			// Read the chunk
			buffer := make([]byte, endByte-startByte)
			if _, err := t.packets.ReadAt(buffer, startByte); err != nil {
				return err
			}
			// Divide it up
			for j := startIdx; j < i; j++ {
				relativePos := nalus[j].Position - startByte
				nalus[j].Payload = buffer[relativePos : relativePos+nalus[j].Length]
			}
			if i < len(nalus) {
				// Start the next chunk
				startByte = nalus[i].Position
				endByte = nalus[i].Position + nalus[i].Length
				startIdx = i
			}
		} else {
			endByte = nalus[i].Position + nalus[i].Length
		}
	}
	return nil
}

// Read NALUs with payload by specifying time instead of packet indices
func (t *Track) ReadAtTime(startTime, endTime time.Duration) ([]NALU, error) {
	rawIdx, err := t.readRawIndex(0, t.indexCount)
	if err != nil {
		return nil, err
	}

	// Find the first packet that is after startTime
	startIdx := sort.Search(len(rawIdx), func(i int) bool {
		return SplitIndexNALUTimeOnly(rawIdx[i]) >= startTime
	})

	// Find the first packet that is after endTime
	endIdx := sort.Search(len(rawIdx), func(i int) bool {
		return SplitIndexNALUTimeOnly(rawIdx[i]) > endTime
	})

	if endIdx-startIdx == 0 {
		// Empty search
		return nil, nil
	}

	nalus, err := t.ReadIndex(startIdx, endIdx)
	if err != nil {
		return nil, err
	}
	if err := t.ReadPayload(nalus); err != nil {
		return nil, err
	}
	return nalus, nil
}

// Write NALUs
func (t *Track) WriteNALUs(nalus []NALU) error {
	if !t.canWrite {
		return ErrReadOnly
	}
	if len(nalus) == 0 {
		return nil
	}
	index := []uint64{}

	// write packets
	for _, nalu := range nalus {
		relativePTS := nalu.PTS.Sub(t.TimeBase)
		if relativePTS < t.duration {
			return fmt.Errorf("NALU occurs before the end of the track (%v < %v), '%v < %v'", relativePTS, t.duration, nalu.PTS, t.TimeBase.Add(t.duration))
		}
		t.duration = relativePTS
		pos := t.packetsSize
		n, err := t.packets.Write(nalu.Payload)
		t.packetsSize += int64(n)
		if err != nil {
			return err
		}
		index = append(index, MakeIndexNALU(EncodeTimeOffset(relativePTS), pos, nalu.Flags))
	}

	// write to index
	_, err := cgogo.WriteSlice(t.index, index)
	t.indexCount += len(index)
	return err
}

func (t *Track) HasCapacity(nalus []NALU) bool {
	if !t.canWrite {
		return false
	}
	if len(nalus) == 0 {
		return true
	}
	// Check if we have enough space in the packets file
	packetBytes := int64(0)
	for _, nalu := range nalus {
		packetBytes += int64(len(nalu.Payload))
	}
	if t.packetsSize+packetBytes > MaxPacketsFileSize {
		return false
	}
	// Check if we have enough time in the index file
	encodedTime := EncodePTSTime(nalus[len(nalus)-1].PTS, t.TimeBase)
	if encodedTime > MaxEncodedPTS {
		return false
	}

	return true
}
