package videox

import "fmt"

type Codec string

const (
	CodecH264 Codec = "h264"
	CodecH265 Codec = "h265"
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
		return CodecH265, nil
	default:
		return "", fmt.Errorf("Unknown codec: %v", codec)
	}
}

// Return the string that FFMpeg uses to identify this codec
func (c Codec) ToFFmpeg() string {
	return string(c)
}
