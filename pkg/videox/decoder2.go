package videox

// #cgo pkg-config: libavcodec libavformat libavutil libswscale
// #include "decoder2.h"
// #include <stdio.h>
// #include <stdlib.h>
// #include <stdint.h>
import "C"
import (
	"errors"
	"io"
	"unsafe"

	"github.com/cyclopcam/cyclops/pkg/accel"
)

// This will replace VideoDecoder once it's finished
type VideoDecoder2 struct {
	decoder unsafe.Pointer
}

func takeCError(err *C.char) error {
	if err == nil {
		return nil
	}
	e := errors.New(C.GoString(err))
	C.free(unsafe.Pointer(err))
	if e.Error() == "EOF" {
		return io.EOF
	}
	return e
}

// Create a new decoder that you will feed with packets
func NewVideoStreamDecoder2(codec Codec) (*VideoDecoder2, error) {
	d := &VideoDecoder2{}
	codecC := C.CString(codec.ToFFmpeg())
	err := takeCError(C.MakeDecoder(nil, codecC, &d.decoder))
	C.free(unsafe.Pointer(codecC))
	if err != nil {
		return nil, err
	}
	return d, nil
}

// Create a new decoder that will decode a file
func NewVideoFileDecoder2(filename string) (*VideoDecoder2, error) {
	d := &VideoDecoder2{}
	filenameC := C.CString(filename)
	err := takeCError(C.MakeDecoder(filenameC, nil, &d.decoder))
	C.free(unsafe.Pointer(filenameC))
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (d *VideoDecoder2) Close() {
	if d.decoder != nil {
		C.Decoder_Close(d.decoder)
		d.decoder = nil
	}
}

func (d *VideoDecoder2) Width() int {
	var width C.int
	var height C.int
	C.Decoder_VideoSize(d.decoder, &width, &height)
	return int(width)
}

func (d *VideoDecoder2) Height() int {
	var width C.int
	var height C.int
	C.Decoder_VideoSize(d.decoder, &width, &height)
	return int(height)
}

// NextFrame reads the next frame from a file and returns a copy of the YUV image.
func (d *VideoDecoder2) NextFrame() (*accel.YUVImage, error) {
	img, err := d.NextFrameDeepRef()
	if err != nil {
		return nil, err
	}
	return img.Clone(), nil
}

// NextFrameDeepRef will read the next frame from a file and return a deep
// reference into the libavcodec decoded image buffer.
// The next call to NextFrame/NextFrameDeepRef will invalidate that image.
func (d *VideoDecoder2) NextFrameDeepRef() (*accel.YUVImage, error) {
	var frame *C.AVFrame
	err := takeCError(C.Decoder_NextFrame(d.decoder, &frame))
	if err != nil {
		return nil, err
	}
	img := makeYUV420ImageDeepUnsafeReference(frame)
	return &img, nil
}

// Decode the packet and return a copy of the YUV image.
// This is used when decoding a stream (not a file).
func (d *VideoDecoder2) Decode(packet *VideoPacket) (*accel.YUVImage, error) {
	img, err := d.DecodeDeepRef(packet)
	if err != nil {
		return nil, err
	}
	return img.Clone(), nil
}

// WARNING: The image returned is only valid while the decoder is still alive,
// and it will be clobbered by the subsequent DecodeDeepRef/Decode().
// The pixels in the returned image are not a garbage-collected Go slice.
// They point directly into the libavcodec decode buffer.
// That's why the function name has the "DeepRef" suffix.
func (d *VideoDecoder2) DecodeDeepRef(packet *VideoPacket) (*accel.YUVImage, error) {
	var frame *C.AVFrame
	encoded := packet.EncodeToAnnexBPacket()
	err := takeCError(C.Decoder_DecodePacket(d.decoder, unsafe.Pointer(&encoded[0]), C.size_t(len(encoded)), &frame))
	if err != nil {
		return nil, err
	}
	img := makeYUV420ImageDeepUnsafeReference(frame)
	return &img, nil
}
