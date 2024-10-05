package rf1

import (
	"bytes"
	"fmt"
	"time"

	"github.com/cyclopcam/cyclops/pkg/cgogo"
)

// #include "rf1.h"
import "C"

const PrintAggregationStats = false

func (t *Track) WriteHeader() error {
	if t.indexCount > MaxIndexEntries {
		return fmt.Errorf("Too many index entries (%v). Maximum is %v", t.indexCount, MaxIndexEntries)
	}
	if t.Type == TrackTypeVideo {
		header := C.VideoIndexHeader{}
		header.TimeBase = C.uint64_t(EncodeTimeBase(t.TimeBase))
		header.IndexCount = C.uint16_t(t.indexCount)
		cgogo.CopySlice(header.Magic[:], []byte(MagicVideoTrackBytes))
		cgogo.CopySlice(header.Codec[:], []byte(t.Codec))
		header.Width = C.uint16_t(t.Width)
		header.Height = C.uint16_t(t.Height)
		if _, err := cgogo.WriteStructAt(t.index, &header, 0); err != nil {
			return err
		}
	} else if t.Type == TrackTypeAudio {
		header := C.AudioIndexHeader{}
		header.TimeBase = C.uint64_t(EncodeTimeBase(t.TimeBase))
		header.IndexCount = C.uint16_t(t.indexCount)
		cgogo.CopySlice(header.Magic[:], []byte(MagicAudioTrackBytes))
		cgogo.CopySlice(header.Codec[:], []byte(t.Codec))
		if _, err := cgogo.WriteStructAt(t.index, &header, 0); err != nil {
			return err
		}
	} else {
		return fmt.Errorf("Invalid track type: %v", t.Type)
	}
	return nil
}

func (t *Track) Close() error {
	var firstErr error

	if t.index != nil {
		if t.dirty {
			firstErr = t.WriteHeader()
			if err := t.index.Truncate(int64(IndexHeaderSize) + int64(t.indexCount+1)*8); firstErr == nil && err != nil {
				firstErr = err
			}
		}
		if err := t.index.Close(); firstErr == nil && err != nil {
			firstErr = err
		}
		t.index = nil
	}

	if t.packets != nil {
		if t.dirty {
			if err := t.packets.Truncate(t.packetsSize); firstErr == nil && err != nil {
				firstErr = err
			}
		}
		if err := t.packets.Close(); firstErr == nil && err != nil {
			firstErr = err
		}
		t.packets = nil
	}

	if firstErr == nil {
		t.dirty = false
	}

	return firstErr
}

// Write NALUs
func (t *Track) WriteNALUs(nalus []NALU) error {
	if !t.canWrite {
		return ErrReadOnly
	}
	if len(nalus) == 0 {
		return nil
	}

	packetBytes := int64(0)
	for _, nalu := range nalus {
		packetBytes += int64(len(nalu.Payload))
	}

	if !t.disablePreallocate && t.packetsSize+packetBytes > t.packetsPreallocSize {
		// Extend the size of the packets file by a large enough increment to achieve our non-fragmentation goal
		t.packetsPreallocSize = packetFilePreallocationSize(t.packetsSize+packetBytes, t.indexCount+len(nalus))
		if err := PreallocateFile(t.packets, t.packetsPreallocSize); err != nil {
			return err
		}
		//if err := t.packets.Truncate(t.packetsPreallocSize); err != nil {
		//	return err
		//}
	}

	if !t.disablePreallocate && (t.indexCount+len(nalus)+1)*8 > int(t.indexPreallocSize) {
		// Extend the size of the index file by a large enough increment to achieve our non-fragmentation goal
		// The maximum size of an index file is 512 KB, so we just pre-allocate the max amount.
		t.indexPreallocSize = (MaxIndexEntries + 1) * 8
		if err := PreallocateFile(t.index, int64(IndexHeaderSize)+t.indexPreallocSize); err != nil {
			return err
		}
	}

	index := []uint64{}

	// write packets

	if !t.disableWriteAggregate && len(nalus) > 1 && int(packetBytes)/len(nalus) < 1000 {
		// For small packets, aggregate data into larger OS writes, because inter-frames for
		// low res streams can be as small as 50 bytes each.
		// I have no idea what the right threshold is here.
		// My guess is that this is worth it, but I haven't measured CPU usage on buffer vs no-buffer.
		// My hunch is that buffering will be worth it for very small writes (eg 10 x 100 byte writes).
		// The extra cost that we're introducing here is an additional memcpy into bytes.Buffer, but
		// if we keep our buffer size very small, then all the in and out from the buffer will live in
		// L2 or even L1 cache, and so that cost will be worth it (vs the cost of many small OS writes).
		writeBuffer := bytes.Buffer{}
		writeBufferMax := 10 * 1024
		pos := t.packetsSize
		iLastWrite := 0
		for i := 0; i <= len(nalus); i++ {
			if i < len(nalus) {
				nalu := nalus[i]
				relativePTS := nalu.PTS.Sub(t.TimeBase)
				if relativePTS < t.duration {
					return fmt.Errorf("NALU occurs before the end of the track (%v < %v), '%v < %v'", relativePTS, t.duration, nalu.PTS, t.TimeBase.Add(t.duration))
				}
				t.duration = relativePTS
				writeBuffer.Write(nalu.Payload)
				index = append(index, MakeIndexNALU(EncodeTimeOffset(relativePTS), pos, nalu.Flags))
				pos += int64(len(nalu.Payload))
			}
			if i == len(nalus) || writeBuffer.Len() > writeBufferMax {
				if PrintAggregationStats {
					fmt.Printf("Writing %v bytes over %v packets in one go\n", writeBuffer.Len(), 1+i-iLastWrite) // Debug info for stats
				}
				n, err := t.packets.WriteAt(writeBuffer.Bytes(), t.packetsSize)
				t.packetsSize += int64(n)
				if err != nil {
					return err
				}
				writeBuffer.Truncate(0)
				iLastWrite = i
			}
		}
	} else {
		// Write without buffering
		if PrintAggregationStats {
			fmt.Printf("Writing %v bytes in %v packets directly\n", packetBytes, len(nalus))
		}
		for _, nalu := range nalus {
			relativePTS := nalu.PTS.Sub(t.TimeBase)
			if relativePTS < t.duration {
				return fmt.Errorf("NALU occurs before the end of the track (%v < %v), '%v < %v'", relativePTS, t.duration, nalu.PTS, t.TimeBase.Add(t.duration))
			}
			t.duration = relativePTS
			pos := t.packetsSize
			n, err := t.packets.WriteAt(nalu.Payload, pos)
			t.packetsSize += int64(n)
			if err != nil {
				return err
			}
			index = append(index, MakeIndexNALU(EncodeTimeOffset(relativePTS), pos, nalu.Flags))
		}
	}

	// add sentinel index entry
	index = append(index, MakeIndexNALU(0, t.packetsSize, 0))

	// write to index, overwriting the previous sentinel
	_, err := cgogo.WriteSliceAt(t.index, index, int64(IndexHeaderSize)+int64(t.indexCount)*8)

	t.indexCount += len(nalus)
	t.dirty = true
	return err
}

func (t *Track) HasCapacity(nNALU int, maxPTS time.Time, combinedPayloadBytes int) bool {
	if !t.canWrite {
		return false
	}
	if nNALU == 0 {
		return true
	}

	// Check if we have enough space in the packets file
	if t.packetsSize+int64(combinedPayloadBytes) > MaxPacketsFileSize {
		return false
	}

	// Check if we have enough time in the index file
	encodedTime := EncodePTSTime(maxPTS, t.TimeBase)
	if encodedTime > MaxEncodedPTS {
		return false
	}

	// Check if we have enough index entries
	if t.indexCount+nNALU+1 > MaxIndexEntries {
		return false
	}

	return true
}

// Given that we are about to write up to byte requiredSize, how large should we
// make the size of the file?
func packetFilePreallocationSize(requiredSize int64, totalNALUCount int) int64 {
	averageBytesPerNALU := requiredSize / int64(totalNALUCount)
	chunk := int64(0)
	if averageBytesPerNALU < 5000 {
		// Low res stream (At 320x240x10fps, 1/10 keyframe, average bytes per frame is about 1000, up to 4000 for windy days)
		// At time limit of 1024 seconds, we have 10*1024 = 10240 frames = 10240 KB = 10 MB
		// We choose 16MB to be the next power of 2 step above 10 MB.
		chunk = 16 * 1024 * 1024
	} else {
		// High res stream (At 1920x1080x10fps, 1/10 keyframe, average bytes per frame is about 150000)
		// At time limit of 1024 seconds, we have 10*1024 = 10240 frames = 1536000 KB = 1500 MB
		chunk = 64 * 1024 * 1024
	}
	// I don't know what a reasonable chunk size is for spinning disc HDDs.
	// 30 seconds of 1.5 MB/s video is 45 MB, so if we have to seek once to read a 30
	// second block, then that seems OK.
	// The loop below will increase the chunk size, but I'm skeptical that anything beyond 64MB is appropriate.
	//maxChunkSize := int64(128 * 1024 * 1024)
	//for chunk < requiredSize && chunk < maxChunkSize {
	//	chunk *= 2
	//}
	// return requiredSize rounded up to nearest chunk
	return (requiredSize + chunk - 1) / chunk * chunk
}

func nextPowerOf2(n int64) int64 {
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	n |= n >> 32
	n++
	return n
}
