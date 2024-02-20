package rf1

import (
	"bytes"
	"fmt"
	"os"
	"unsafe"

	"github.com/cyclopcam/cyclops/pkg/cgogo"
)

// #include "rf1.h"
import "C"

const IndexHeaderSize = int(unsafe.Sizeof(C.CommonIndexHeader{}))

// Reader is used to read an rf1 file group
type Reader struct {
	Tracks []*Track
}

// Open an rf1 file group for reading
func Open(baseFilename string) (*Reader, error) {
	// Scan for tracks
	tracks := make([]*Track, 0)
	for i := 0; ; i++ {
		track, err := OpenTrack(baseFilename, i)
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
	return &Reader{Tracks: tracks}, nil
}

// Open a track for reading
func OpenTrack(baseFilename string, trackIdx int) (*Track, error) {
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

	idxFile, err = os.Open(TrackFilename(baseFilename, trackIdx, FileTypeIndex))
	if err != nil {
		return nil, err
	}
	pktFile, err = os.Open(TrackFilename(baseFilename, trackIdx, FileTypePackets))
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
	var isVideo bool
	if bytes.Equal(magic[:], []byte(MagicAudioTrackBytes)) {
		isVideo = false
	} else if bytes.Equal(magic[:], []byte(MagicVideoTrackBytes)) {
		isVideo = true
	} else {
		return nil, fmt.Errorf("Unrecognized magic bytes in index track: %02x %02x %02x %02x", magic[0], magic[1], magic[2], magic[3])
	}

	if !IsValidCodec(string(codec[:])) {
		return nil, fmt.Errorf("%w '%v'", ErrInvalidCodec, string(codec[:]))
	}

	idxStat, err := idxFile.Stat()
	if err != nil {
		return nil, err
	}
	indexCount := (int(idxStat.Size()) - IndexHeaderSize) / 8

	pktStat, err := pktFile.Stat()
	if err != nil {
		return nil, err
	}

	track := &Track{
		IsVideo:     isVideo,
		Codec:       string(codec[:]),
		TimeBase:    DecodeTimeBase(uint64(indexHead.TimeBase)),
		index:       idxFile,
		indexCount:  int(indexCount),
		packets:     pktFile,
		packetsSize: pktStat.Size(),
	}

	if isVideo {
		videoHead := C.VideoIndexHeader{}
		commonHeadBytes := unsafe.Slice((*byte)(unsafe.Pointer(&indexHead)), int(unsafe.Sizeof(indexHead)))
		videoHeadBytes := unsafe.Slice((*byte)(unsafe.Pointer(&videoHead)), int(unsafe.Sizeof(videoHead)))
		copy(videoHeadBytes, commonHeadBytes)
		track.Width = int(videoHead.Width)
		track.Height = int(videoHead.Height)
	}

	success = true
	return track, nil
}

// Returns the number of NALUs
func (t *Track) Count() int {
	return t.indexCount
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
	startByte := int64(IndexHeaderSize) + int64(startIdx)*8
	raw := make([]uint64, readCount)
	_, err := cgogo.ReadSliceAt(t.index, raw, startByte)
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
