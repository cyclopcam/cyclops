package rf1

import (
	"fmt"
	"sort"
	"time"

	"github.com/cyclopcam/cyclops/pkg/cgogo"
)

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

// Read a range of uint64 index entries, either from the file, or from our in-memory cache.
// At present out caching strategy is simply to read the entire index the first time
// we need any of it, and keep the whole thing in memory. The index is so small compared to
// the payload, that it's not clear that anything else makes sense.
func (t *Track) readRawIndex(startIdx, endIdx int) ([]uint64, error) {
	if startIdx < 0 || endIdx <= startIdx || endIdx > t.indexCount+1 {
		return nil, fmt.Errorf("Invalid readRawIndex startIdx, endIdx: %v, %v (number of indices: %v)", startIdx, endIdx, t.indexCount+1)
	}
	if len(t.indexCache) != t.indexCount+1 {
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
			raw := make([]uint64, t.indexCount+1)
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
	// We read up to an including endIdx, so that we know the length of the last NALU request.
	// It is always possible to read one beyond the end, because of our sentinel NALU that is always present,
	// even in an empty file.
	readCount := 1 + endIdx - startIdx
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
	// Chop off the final NALU (or sentinel) that we artificially added in for Length computation
	nalus = nalus[:len(nalus)-1]
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

	// Work in encoded time so that we don't need to worry about rounding/precision issues.
	// In other words, if somebody writes packets at time T, and then requests to read
	// back packets from time T onwards, we are guaranteed here to always return the first packet,
	// and not skip it because of a rounding/precision error.
	startTimeEncoded := EncodeTimeOffset(startTime)
	endTimeEncoded := EncodeTimeOffset(endTime)

	// Find the first packet that is at or after startTime
	startIdx := sort.Search(len(rawIdx), func(i int) bool {
		return SplitIndexNALUEncodedTimeOnly(rawIdx[i]) >= startTimeEncoded
	})

	// Find the first packet that is after endTime
	endIdx := sort.Search(len(rawIdx), func(i int) bool {
		return SplitIndexNALUEncodedTimeOnly(rawIdx[i]) > endTimeEncoded
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
