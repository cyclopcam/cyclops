package videox

import (
	"fmt"

	"github.com/bluenviron/mediacommon/pkg/codecs/h264"
	"github.com/bluenviron/mediacommon/pkg/codecs/h265"
)

type Codec int

const (
	CodecUnknown Codec = iota
	CodecH264
	CodecH265
)

type AbstractNALUType int

const (
	AbstractNALUTypeOther         AbstractNALUType = iota // Any other NALU type
	AbstractNALUTypeEssentialMeta                         // SPS, PPS, VPS. Required before we can decode a frame.
	AbstractNALUTypeIDR                                   // Keyframe (Instantaneous Decoder Refresh)
	AbstractNALUTypeNonIDR                                // Visual frame, but not a keyframe
)

func ParseCodec(codec string) (Codec, error) {
	switch codec {
	case "h264":
		fallthrough
	case "H264":
		return CodecH264, nil
	case "h265":
		fallthrough
	case "H265":
		fallthrough
	case "hevc":
		return CodecH265, nil
	default:
		return CodecUnknown, fmt.Errorf("Unknown codec: %v", codec)
	}
}

// Return the string that FFMpeg uses to identify this codec
func (c Codec) ToFFmpeg() string {
	switch c {
	case CodecH264:
		return "h264"
	case CodecH265:
		return "hevc"
	default:
		return "unknown"
	}
}

func (c Codec) InternalName() string {
	switch c {
	case CodecH264:
		return "h264"
	case CodecH265:
		return "h265"
	default:
		return "unknown"
	}
}

func (c Codec) String() string {
	return c.InternalName()
}

func ReadNaluTypeH264(firstByte byte) h264.NALUType {
	return h264.NALUType(firstByte & 31)
}

func ReadNaluTypeH265(firstByte byte) h265.NALUType {
	return h265.NALUType((firstByte >> 1) & 63)
}

func H264ToAbstractType(firstByte byte) AbstractNALUType {
	switch ReadNaluTypeH264(firstByte) {
	case h264.NALUTypeNonIDR:
		return AbstractNALUTypeNonIDR
	case h264.NALUTypeIDR:
		return AbstractNALUTypeIDR
	case h264.NALUTypeSPS:
		fallthrough
	case h264.NALUTypePPS:
		return AbstractNALUTypeEssentialMeta
	default:
		return AbstractNALUTypeOther
	}
}

func H265ToAbstractType(firstByte byte) AbstractNALUType {
	t := ReadNaluTypeH265(firstByte)
	if (t >= 0 && t <= 9) || (t >= 16 && t <= 18) || (t == 21) {
		return AbstractNALUTypeNonIDR
	}

	switch t {
	case h265.NALUType_IDR_W_RADL:
		fallthrough
	case h265.NALUType_IDR_N_LP:
		return AbstractNALUTypeIDR
	case h265.NALUType_VPS_NUT:
		fallthrough
	case h265.NALUType_SPS_NUT:
		fallthrough
	case h265.NALUType_PPS_NUT:
		return AbstractNALUTypeEssentialMeta
	default:
		return AbstractNALUTypeOther
	}
}

func (t AbstractNALUType) IsVisual() bool {
	return t == AbstractNALUTypeNonIDR || t == AbstractNALUTypeIDR
}
