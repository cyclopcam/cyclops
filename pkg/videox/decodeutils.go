package videox

import (
	"errors"
	"fmt"
	"os"
	"time"
	"unsafe"

	"github.com/bluenviron/mediacommon/pkg/codecs/h264"
	"github.com/bmharper/cimg/v2"
	"github.com/cyclopcam/cyclops/pkg/accel"
)

// #cgo pkg-config: libavformat libavcodec libavutil libswscale
// #include <libavformat/avformat.h>
// #include <libavcodec/avcodec.h>
// #include <libavutil/imgutils.h>
import "C"

func IsVisualPacket(t h264.NALUType) bool {
	return int(t) >= 1 && int(t) <= 5
}

var ErrResourceTemporarilyUnavailable = errors.New("Resource temporarily unavailable") // common response from avcodec_receive_frame if a frame is not available

// Creates a decoder and attempts to decode a single IDR packet.
// This was built for extracting a thumbnail during a long recording.
// Obviously this is a bit expensive, because you're creating a decoder
// for just a single frame.
func DecodeSinglePacketToImage(codec Codec, packet *VideoPacket) (*cimg.Image, error) {
	decoder, err := NewVideoStreamDecoder2(codec)
	if err != nil {
		return nil, err
	}
	defer decoder.Close()
	frame, err := decoder.Decode(packet)
	if err != nil {
		return nil, err
	}
	return frame.Image.ToCImageRGB(), nil
}

// Decode the list of packets, and return the first image that successfully decodes
func DecodeFirstImageInPacketList(codec Codec, packets []*VideoPacket) (*cimg.Image, time.Time, error) {
	return DecodeClosestImageInPacketList(codec, packets, time.Time{}, nil, "")
}

// If true, report the decode FPS
const DebugVideoDecodeTimes = false

// Decode the list of packets, and return the decoded image who's presentation time is closest to targetTime.
// If targetTime is zero, then we return the first image coming out of the decoder.
// If cache is not nil, then we will insert/query the provided cache.
// videoCacheKey is the key for this video. We use {videoCacheKey-PTS} as the complete cache key.
func DecodeClosestImageInPacketList(codec Codec, packets []*VideoPacket, targetTime time.Time, cache *FrameCache, videoCacheKey string) (*cimg.Image, time.Time, error) {
	// First see if the frame is in the cache
	if cache != nil {
		frameCacheKey := cache.MakeKey(videoCacheKey, targetTime.UnixMilli())
		if img := cache.GetFrame(frameCacheKey); img != nil {
			//fmt.Printf("Cache hit for %v\n", frameCacheKey)
			return img.ToCImageRGB(), targetTime, nil
		}
	}

	startTime := time.Now()
	nFramesDecoded := 0
	decoder, err := NewVideoStreamDecoder2(codec)
	if err != nil {
		return nil, time.Time{}, err
	}
	defer decoder.Close()
	var bestImg *accel.YUVImage
	bestTime := time.Time{}
	bestDelta := time.Duration(1<<63 - 1)
	var firstError error
	for _, p := range packets {
		frame, err := decoder.DecodeDeepRef(p)
		if err != nil && firstError == nil {
			firstError = err
		}
		if frame != nil {
			nFramesDecoded++
			if cache != nil {
				frameCacheKey := cache.MakeKey(videoCacheKey, p.WallPTS.UnixMilli())
				cache.AddFrame(frameCacheKey, frame.Image.Clone())
			}
			timeDelta := time.Duration(0)
			if !targetTime.IsZero() {
				timeDelta = p.WallPTS.Sub(targetTime)
				if timeDelta < 0 {
					timeDelta = -timeDelta
				}
			}
			if timeDelta < bestDelta {
				bestTime = p.WallPTS
				bestDelta = timeDelta
				bestImg = frame.Image.Clone()
			}
			if p.WallPTS.After(targetTime) || targetTime.IsZero() || bestDelta == 0 {
				// No point decoding packets once we've passed our desired time, or if we're 100% on our desired time
				break
			}
		}
	}
	if DebugVideoDecodeTimes && nFramesDecoded != 0 {
		fmt.Printf("Decoded %v frames in %.3f seconds (%.1f FPS)\n", nFramesDecoded, time.Since(startTime).Seconds(), float64(nFramesDecoded)/time.Since(startTime).Seconds())
	}
	if bestImg != nil {
		return bestImg.ToCImageRGB(), bestTime, nil
	}
	if firstError == nil {
		firstError = fmt.Errorf("No image found")
	}
	return nil, time.Time{}, firstError
}

// Create a deep (and unsafe) reference to the YUV 420 frame from ffmpeg.
func makeYUV420ImageDeepUnsafeReference(frame *C.AVFrame) accel.YUVImage {
	if frame.format != C.AV_PIX_FMT_YUV420P && frame.format != C.AV_PIX_FMT_YUVJ420P {
		panic(fmt.Sprintf("Unsupported pixel format %v. Only YUV420p is supported.", frame.format))
	}
	width := int(frame.width)
	height := int(frame.height)
	strideY := int(frame.linesize[0])
	strideU := int(frame.linesize[1])
	strideV := int(frame.linesize[2])
	rawY := unsafe.Slice((*byte)(frame.data[0]), strideY*height)
	rawU := unsafe.Slice((*byte)(frame.data[1]), strideU*height/2)
	rawV := unsafe.Slice((*byte)(frame.data[2]), strideV*height/2)
	return accel.YUVImage{
		Width:  width,
		Height: height,
		Y:      rawY,
		U:      rawU,
		V:      rawV,
	}
}

func makeYUVImageCopy(frame *C.AVFrame) *accel.YUVImage {
	ref := makeYUV420ImageDeepUnsafeReference(frame)
	return ref.Clone()
}

func dumpYUV420pFrame(frame *C.AVFrame) {
	fmt.Printf("%v\n", frame)

	dump, _ := os.Create("testdata/yuv/dump.y")
	dump.Write(unsafe.Slice((*byte)(frame.data[0]), int(frame.linesize[0]*frame.height)))
	dump, _ = os.Create("testdata/yuv/dump.u")
	dump.Write(unsafe.Slice((*byte)(frame.data[1]), int(frame.linesize[1]*frame.height/2)))
	dump, _ = os.Create("testdata/yuv/dump.v")
	dump.Write(unsafe.Slice((*byte)(frame.data[2]), int(frame.linesize[2]*frame.height/2)))
}
