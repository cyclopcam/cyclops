package rf1

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	MagicAudioTrackBytes = "rf1a" // must be 4 bytes long
	MagicVideoTrackBytes = "rf1v" // must be 4 bytes long
	CodecH264            = "h264" // must be 4 bytes long, and present in IsValidCodec()
	CodecH265            = "h265" // must be 4 bytes long, and present in IsValidCodec()
)

var ErrInvalidCodec = errors.New("invalid codec")

const MaxPacketsFileSize = 1<<30 - 1 // 1 GB
const MaxDuration = 1024 * time.Second
const MaxEncodedPTS = 1<<22 - 1
const MaxIndexEntries = 1<<16 - 1

type FileType int

const (
	FileTypeIndex FileType = iota
	FileTypePackets
)

// Flags of a NALU in the track index
type IndexNALUFlags uint32

// We have 12 bits for flags, so maximum flag value is 1 << 11 = 2048
const (
	IndexNALUFlagKeyFrame      IndexNALUFlags = 1 // Key frame
	IndexNALUFlagEssentialMeta IndexNALUFlags = 2 // Essential metadata, required to initialize the decoder (eg SPS+PPS NALUs in h264 / VPS+SPS+PPS NALUs h265)
	IndexNALUFlagAnnexB        IndexNALUFlags = 4 // Packet has Annex-B "emulation prevention bytes" and start codes
)

type NALU struct {
	PTS      time.Time
	Flags    IndexNALUFlags
	Position int64 // Position in packets file. Only used when reading
	Length   int64 // Only used when reading (logically this is equal to len(Payload), but Payload might be nil)
	Payload  []byte
}

func (n *NALU) IsKeyFrame() bool {
	return n.Flags&IndexNALUFlagKeyFrame != 0
}

// Encode an offset-based time to units of 1/4096 of a second
func EncodePTSTime(t time.Time, timeBase time.Time) int64 {
	return EncodeTimeOffset(t.Sub(timeBase))
}

// Decode an int64 time of 1/4096 of a second to a time.Time
func DecodePTSTime(t int64, timeBase time.Time) time.Time {
	return timeBase.Add(DecodeTimeOffset(t))
}

// Encode a time.Duration to units of 1/4096 of a second
func EncodeTimeOffset(t time.Duration) int64 {
	return int64(t * 4096 / time.Second)
}

// Decode an int64 time of 1/4096 of a second to a time.Duration
func DecodeTimeOffset(t int64) time.Duration {
	return time.Duration(t) * time.Second / 4096
}

// Encode a time to a microsecond Unix epoch encoding
func EncodeTimeBase(t time.Time) uint64 {
	return uint64(t.UnixMicro())
}

// Decode a microsecond Unix epoch encoding to a time
func DecodeTimeBase(t uint64) time.Time {
	return time.UnixMicro(int64(t)).UTC()
}

// Assumes little endian
func MakeIndexNALU(pts int64, location int64, flags IndexNALUFlags) uint64 {
	if pts < 0 || pts >= 1<<22 {
		panic("pts out of range")
	}
	if location < 0 || location >= 1<<30 {
		panic("location out of range")
	}
	if flags < 0 || flags >= 1<<12 {
		panic("flags out of range")
	}
	return uint64(pts)<<42 | uint64(location)<<12 | uint64(flags)&0xfff
}

// Assumes little endian
func SplitIndexNALU(p uint64) (pts int64, location int64, flags IndexNALUFlags) {
	pts = int64(p >> 42)
	location = int64((p >> 12) & (1<<30 - 1))
	flags = IndexNALUFlags(p & 0xfff)
	return
}

func MakeIndexSentinel(location int64) uint64 {
	return MakeIndexNALU(0, 0, 0)
}

func SplitIndexNALUEncodedTimeOnly(p uint64) int64 {
	return int64(p >> 42)
}

func SplitIndexNALUTimeOnly(p uint64) time.Duration {
	return DecodeTimeOffset(int64(p >> 42))
}

func SplitIndexNALULocationOnly(p uint64) int64 {
	return int64((p >> 12) & (1<<30 - 1))
}

func SplitIndexNALUFlagsOnly(p uint64) IndexNALUFlags {
	return IndexNALUFlags(p & 0xfff)
}

func IsValidCodec(c string) bool {
	return len(c) == 4 && (c == CodecH264 || c == CodecH265)
}

func Extension(fileType FileType) string {
	switch fileType {
	case FileTypeIndex:
		return "rf1i"
	case FileTypePackets:
		return "rf1p"
	default:
		panic("Invalid fileType")
	}
}

func TrackFilename(baseFilename string, trackName string, fileType FileType) string {
	return fmt.Sprintf("%v_%v.%v", baseFilename, trackName, Extension(fileType))
}

func IsVideoFile(filename string) bool {
	return strings.HasSuffix(filename, ".rf1i")
}
