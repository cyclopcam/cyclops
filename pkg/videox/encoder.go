package videox

// #cgo pkg-config: libavcodec libavformat libavutil libswscale
// #include "encoder.h"
// #include <stdio.h>
// #include <stdlib.h>
// #include <stdint.h>
import "C"
import (
	"time"
	"unsafe"
)

// Export some of the ffmpeg C pixel formats to Go
type AVPixelFormat int

const (
	AVPixelFormatYUV420P AVPixelFormat = C.AV_PIX_FMT_YUV420P
	AVPixelFormatRGB24   AVPixelFormat = C.AV_PIX_FMT_RGB24
)

func NumPlanes(pixelFormat AVPixelFormat) int {
	switch pixelFormat {
	case AVPixelFormatYUV420P:
		return 3
	case AVPixelFormatRGB24:
		return 1
	}
	panic("Unknown pixel format")
}

type VideoEncoderType int

const (
	VideoEncoderTypePackets     VideoEncoderType = C.EncoderTypePackets     // Sending pre-encoded packets/NALUs to the encoder
	VideoEncoderTypeImageFrames VideoEncoderType = C.EncoderTypeImageFrames // Sending image frames to the encoder
)

type VideoEncoder struct {
	enc              unsafe.Pointer
	InputPixelFormat AVPixelFormat
}

// NewVideoEncoder creates a new video encoder
// You must Close() a video encoder when you are done using it, otherwise you will leak ffmpeg objects
func NewVideoEncoder(codec, format, filename string, width, height int, pixelFormatIn, pixelFormatOut AVPixelFormat, encoderType VideoEncoderType, fps int) (*VideoEncoder, error) {
	// Populate EncoderParams
	cCodec := C.CString(codec)
	var params C.EncoderParams
	err := takeCError(C.MakeEncoderParams(cCodec, C.int(width), C.int(height), C.enum_AVPixelFormat(pixelFormatIn), C.enum_AVPixelFormat(pixelFormatOut), C.enum_EncoderType(encoderType), C.int(fps), &params))
	C.free(unsafe.Pointer(cCodec))
	if err != nil {
		return nil, err
	}

	cFormat := C.CString(format)
	cFilename := C.CString(filename)
	var encoder unsafe.Pointer
	err = takeCError(C.MakeEncoder(cFormat, cFilename, &params, &encoder))
	C.free(unsafe.Pointer(cFormat))
	C.free(unsafe.Pointer(cFilename))
	if err != nil {
		return nil, err
	}
	return &VideoEncoder{
		enc:              encoder,
		InputPixelFormat: pixelFormatIn,
	}, nil
}

func (v *VideoEncoder) Close() {
	if v.enc != nil {
		C.Encoder_Close(v.enc)
		v.enc = nil
	}
}

func (v *VideoEncoder) WriteNALU(dts, pts time.Duration, nalu NALU) error {
	if !nalu.PayloadIsAnnexB {
		// Encoding to Annex-B has a non-zero cost (615 MB/s on a Raspberry Pi 5, vs 4600 MB/s memcpy).
		nalu = nalu.AsAnnexB()
	}
	idts := C.int64_t(dts.Nanoseconds())
	ipts := C.int64_t(pts.Nanoseconds())
	return takeCError(C.Encoder_WriteNALU(v.enc, idts, ipts, C.int(nalu.StartCodeLen()), unsafe.Pointer(&nalu.Payload[0]), C.ulong(len(nalu.Payload))))
}

func (v *VideoEncoder) WritePacket(dts, pts time.Duration, packet *VideoPacket) error {
	idts := C.int64_t(dts.Nanoseconds())
	ipts := C.int64_t(pts.Nanoseconds())
	encoded := packet.EncodeToAnnexBPacket()
	isKeyFrame := C.int(0)
	if packet.HasIDR() {
		isKeyFrame = 1
	}
	return takeCError(C.Encoder_WritePacket(v.enc, idts, ipts, isKeyFrame, unsafe.Pointer(&encoded[0]), C.ulong(len(encoded))))
}

// Write an RGB (single plane) or YUV (3 planes) image to the encoder
func (v *VideoEncoder) WriteImage(pts time.Duration, data [][]uint8, stride []int) error {
	var frame *C.AVFrame
	if err := takeCError(C.Encoder_MakeFrameWriteable(v.enc, &frame)); err != nil {
		return err
	}
	for i := 0; i < NumPlanes(v.InputPixelFormat); i++ {
		frame.data[i] = (*C.uint8_t)(unsafe.Pointer(&data[i][0]))
		frame.linesize[i] = C.int(stride[i])
	}
	return takeCError(C.Encoder_WriteFrame(v.enc, C.int64_t(pts.Nanoseconds())))
}

func (v *VideoEncoder) WriteTrailer() error {
	return takeCError(C.Encoder_WriteTrailer(v.enc))
}
