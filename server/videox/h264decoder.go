package videox

import (
	"errors"
	"fmt"
	"image"
	"unsafe"

	"github.com/aler9/gortsplib/pkg/h264"
)

// #cgo pkg-config: libavcodec libavutil libswscale
// #include <libavcodec/avcodec.h>
// #include <libavutil/imgutils.h>
// #include <libswscale/swscale.h>
// #include "encoder.h"
import "C"

func frameData(frame *C.AVFrame) **C.uint8_t {
	return (**C.uint8_t)(unsafe.Pointer(&frame.data[0]))
}

func frameLineSize(frame *C.AVFrame) *C.int {
	return (*C.int)(unsafe.Pointer(&frame.linesize[0]))
}

func IsVisualPacket(t h264.NALUType) bool {
	return int(t) >= 1 && int(t) <= 5
}

var ErrResourceTemporarilyUnavailable = errors.New("Resource temporarily unavailable") // common response from avcodec_receive_frame if a frame is not available

func WrapAvErr(err C.int) error {
	if err == -11 {
		return ErrResourceTemporarilyUnavailable
	}
	//char msg[AV_ERROR_MAX_STRING_SIZE] = {0};
	//av_make_error_string(msg, AV_ERROR_MAX_STRING_SIZE, e);
	//return msg;
	//C.av_make_error_string()
	return errors.New(C.GoString(C.GetAvErrorStr(err)))
}

// H264Decoder is a wrapper around ffmpeg's H264 decoder.
type H264Decoder struct {
	codecCtx    *C.AVCodecContext
	srcFrame    *C.AVFrame
	swsCtx      *C.struct_SwsContext
	dstFrame    *C.AVFrame
	dstFramePtr []uint8
}

// NewH264Decoder allocates a new H264Decoder.
func NewH264Decoder() (*H264Decoder, error) {
	// I tried this on Rpi4, to make sure I'm getting hardware decode.. but:
	// This doesn't work.. I just get "avcodec_receive_frame error Resource temporarily unavailable"
	// Perhaps it could work, I didn't try harder.
	//codecName := C.CString("h264_v4l2m2m")
	//defer C.free(unsafe.Pointer(codecName))
	//codec := C.avcodec_find_decoder_by_name(codecName)

	codec := C.avcodec_find_decoder(C.AV_CODEC_ID_H264)
	if codec == nil {
		return nil, fmt.Errorf("avcodec_find_decoder() failed")
	}

	codecCtx := C.avcodec_alloc_context3(codec)
	if codecCtx == nil {
		return nil, fmt.Errorf("avcodec_alloc_context3() failed")
	}

	res := C.avcodec_open2(codecCtx, codec, nil)
	if res < 0 {
		C.avcodec_close(codecCtx)
		return nil, fmt.Errorf("avcodec_open2() failed: %v", res)
	}

	srcFrame := C.av_frame_alloc()
	if srcFrame == nil {
		C.avcodec_close(codecCtx)
		return nil, fmt.Errorf("av_frame_alloc() failed")
	}

	return &H264Decoder{
		codecCtx: codecCtx,
		srcFrame: srcFrame,
	}, nil
}

// close closes the decoder.
func (d *H264Decoder) Close() {
	if d.dstFrame != nil {
		C.av_frame_free(&d.dstFrame)
	}

	if d.swsCtx != nil {
		C.sws_freeContext(d.swsCtx)
	}

	C.av_frame_free(&d.srcFrame)
	C.avcodec_close(d.codecCtx)
}

// Send the packet to the decoder, but don't retrieve the next frame
// This is of dubious value... I wanted to use it for decoding SPS only.. but turns out
// you must decode the first frame before avcodec context will have the width/height.
// So I ended up getting a dedicated SPS parser...
//func (d *H264Decoder) DecodeAndDiscard(nalu NALU) error {
//	return d.sendPacket(nalu)
//}

// WARNING: The image returned is only valid while the decoder is still alive,
// and it will be clobbered by the subsequent Decode()
func (d *H264Decoder) Decode(packet *DecodedPacket) (image.Image, error) {
	if err := d.sendPacket(packet.EncodeToAnnexBPacket()); err != nil {
		// sendPacket failure is not fatal
		// We should log it or something.
		// But it occurs normally during start of a stream, before first IDR has been seen.
	}

	//if !IsVisualPacket(nalu.Type()) {
	//	// avcodec_receive_frame will return an error if we try to decode a frame when
	//	// sending a non-visual NALU
	//	return nil, nil
	//}

	// receive frame if available
	res := C.avcodec_receive_frame(d.codecCtx, d.srcFrame)
	if res < 0 {
		// This special code path should no longer be necessary. We were decoding before SPS+PPS,
		// and we were trying to extract a frame when sending future SPS+PPS.. so both those paths
		// are now removed, and any error should be a genuine error.
		//err := WrapAvErr(res)
		//if err == ErrResourceTemporarilyUnavailable {
		//	// is this missing SPS + PPS prefixed to IDR?
		//	return nil, nil
		//}
		return nil, fmt.Errorf("avcodec_receive_frame error %w", WrapAvErr(res))
	}

	// if frame size has changed, allocate needed objects
	if d.dstFrame == nil || d.dstFrame.width != d.srcFrame.width || d.dstFrame.height != d.srcFrame.height {
		if d.dstFrame != nil {
			C.av_frame_free(&d.dstFrame)
		}

		if d.swsCtx != nil {
			C.sws_freeContext(d.swsCtx)
		}

		d.dstFrame = C.av_frame_alloc()
		d.dstFrame.format = C.AV_PIX_FMT_RGBA
		d.dstFrame.width = d.srcFrame.width
		d.dstFrame.height = d.srcFrame.height
		d.dstFrame.color_range = C.AVCOL_RANGE_JPEG
		res = C.av_frame_get_buffer(d.dstFrame, 1)
		if res < 0 {
			return nil, fmt.Errorf("av_frame_get_buffer() error %v", res)
		}

		d.swsCtx = C.sws_getContext(d.srcFrame.width, d.srcFrame.height, C.AV_PIX_FMT_YUV420P,
			d.dstFrame.width, d.dstFrame.height, (int32)(d.dstFrame.format), C.SWS_BILINEAR, nil, nil, nil)
		if d.swsCtx == nil {
			return nil, fmt.Errorf("sws_getContext() error")
		}

		dstFrameSize := C.av_image_get_buffer_size((int32)(d.dstFrame.format), d.dstFrame.width, d.dstFrame.height, 1)
		d.dstFramePtr = (*[1 << 30]uint8)(unsafe.Pointer(d.dstFrame.data[0]))[:dstFrameSize:dstFrameSize]
	}

	// convert frame from YUV420 to RGB
	res = C.sws_scale(d.swsCtx, frameData(d.srcFrame), frameLineSize(d.srcFrame),
		0, d.srcFrame.height, frameData(d.dstFrame), frameLineSize(d.dstFrame))
	if res < 0 {
		return nil, fmt.Errorf("sws_scale() error %v", res)
	}

	//fmt.Printf("Got frame %v x %v -> %v x %v\n", d.srcFrame.width, d.srcFrame.height, d.dstFrame.width, d.dstFrame.height)

	// embed frame into an image.Image
	return &image.RGBA{
		Pix:    d.dstFramePtr,
		Stride: 4 * (int)(d.dstFrame.width),
		Rect: image.Rectangle{
			Max: image.Point{(int)(d.dstFrame.width), (int)(d.dstFrame.height)},
		},
	}, nil
}

func (d *H264Decoder) Width() int {
	return int(d.codecCtx.width)
}

func (d *H264Decoder) Height() int {
	return int(d.codecCtx.height)
}

func (d *H264Decoder) sendPacket(packet []byte) error {
	//if nalu.PrefixLen == 0 {
	//	nalu = nalu.CloneWithPrefix()
	//}

	// The following doesn't work, because we're storing a Go pointer inside a C struct,
	// and then passing that C struct to a C function. So to fix this, we need to define
	// a helper function in C.
	// d.avPacket.data = (*C.uint8_t)(unsafe.Pointer(&nalu.Payload[0]))
	// This is why I created AvCodecSendPacket.
	// Using a Go struct for the C struct is anyway not future compatible, because the avcodec
	// API is deprecating the fixed struct size, and rather moving to requiring the use of
	// an alloc function to create a packet.

	//res := C.AvCodecSendPacket(d.codecCtx, unsafe.Pointer(&nalu.Payload[0]), C.ulong(len(nalu.Payload)))
	res := C.AvCodecSendPacket(d.codecCtx, unsafe.Pointer(&packet[0]), C.ulong(len(packet)))
	if res < 0 {
		return fmt.Errorf("avcodec_send_packet failed: %v", res)
	}
	return nil
}

// Creates a decoder and attempts to decode a single IDR packet.
// This was built for extracting a thumbnail during a long recording.
// Obviously this is quite expensive, because you're creating a decoder
// for just a single frame.
func DecodeSinglePacketToImage(packet *DecodedPacket) (image.Image, error) {
	decoder, err := NewH264Decoder()
	if err != nil {
		return nil, err
	}
	defer decoder.Close()
	img, err := decoder.Decode(packet)
	if err != nil {
		return nil, err
	}
	return cloneImage(img), nil
}
