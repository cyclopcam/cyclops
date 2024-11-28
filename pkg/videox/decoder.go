package videox

// #cgo pkg-config: libavcodec libavformat libavutil libswscale
// #include "decoder.h"
// #include <stdio.h>
// #include <stdlib.h>
// #include <stdint.h>
import "C"
import (
	"errors"
	"io"
	"time"
	"unsafe"

	"github.com/cyclopcam/cyclops/pkg/accel"
)

// VideoDecoder is a wrapper around ffmpeg, for decoding videos
type VideoDecoder struct {
	decoder unsafe.Pointer
}

// A decoded frame
type Frame struct {
	Image *accel.YUVImage // Image (might be a deep reference into ffmpeg memory)
	PTS   int64           // Presentation time in native time units. Use VideoDecoder.FrameTimeToDuration() to convert to a time.Duration
}

// Return a deep clone of the frame (new image memory)
func (f *Frame) DeepClone() *Frame {
	return &Frame{
		Image: f.Image.Clone(),
		PTS:   f.PTS,
	}
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
func NewVideoStreamDecoder(codec Codec) (*VideoDecoder, error) {
	d := &VideoDecoder{}
	codecC := C.CString(codec.ToFFmpeg())
	err := takeCError(C.MakeDecoder(nil, codecC, &d.decoder))
	C.free(unsafe.Pointer(codecC))
	if err != nil {
		return nil, err
	}
	return d, nil
}

// Create a new decoder that will decode a file
func NewVideoFileDecoder(filename string) (*VideoDecoder, error) {
	d := &VideoDecoder{}
	filenameC := C.CString(filename)
	err := takeCError(C.MakeDecoder(filenameC, nil, &d.decoder))
	C.free(unsafe.Pointer(filenameC))
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (d *VideoDecoder) Close() {
	if d.decoder != nil {
		C.Decoder_Close(d.decoder)
		d.decoder = nil
	}
}

func (d *VideoDecoder) Width() int {
	var width C.int
	var height C.int
	C.Decoder_VideoSize(d.decoder, &width, &height)
	return int(width)
}

func (d *VideoDecoder) Height() int {
	var width C.int
	var height C.int
	C.Decoder_VideoSize(d.decoder, &width, &height)
	return int(height)
}

// NextFrame reads the next frame from a file and returns a copy of the YUV image.
func (d *VideoDecoder) NextFrame() (*Frame, error) {
	img, err := d.NextFrameDeepRef()
	if err != nil {
		return nil, err
	}
	return img.DeepClone(), nil
}

// NextFrameDeepRef will read the next frame from a file and return a deep
// reference into the libavcodec decoded image buffer.
// The next call to NextFrame/NextFrameDeepRef will invalidate that image.
func (d *VideoDecoder) NextFrameDeepRef() (*Frame, error) {
	var frame *C.AVFrame
	err := takeCError(C.Decoder_NextFrame(d.decoder, &frame))
	if err != nil {
		return nil, err
	}
	img := makeYUV420ImageDeepUnsafeReference(frame)
	return &Frame{
		Image: &img,
		PTS:   int64(frame.pts),
	}, nil
}

// Decode the packet and return a copy of the YUV image.
// This is used when decoding a stream (not a file).
func (d *VideoDecoder) Decode(packet *VideoPacket) (*Frame, error) {
	frame, err := d.DecodeDeepRef(packet)
	if err != nil {
		return nil, err
	}
	return frame.DeepClone(), nil
}

// WARNING: The image returned is only valid while the decoder is still alive,
// and it will be clobbered by the subsequent DecodeDeepRef/Decode().
// The pixels in the returned image are not a garbage-collected Go slice.
// They point directly into the libavcodec decode buffer.
// That's why the function name has the "DeepRef" suffix.
func (d *VideoDecoder) DecodeDeepRef(packet *VideoPacket) (*Frame, error) {
	var frame *C.AVFrame
	encoded := packet.EncodeToAnnexBPacket()
	err := takeCError(C.Decoder_DecodePacket(d.decoder, unsafe.Pointer(&encoded[0]), C.size_t(len(encoded)), &frame))
	if err != nil {
		return nil, err
	}
	img := makeYUV420ImageDeepUnsafeReference(frame)
	return &Frame{
		Image: &img,
		PTS:   int64(frame.pts),
	}, nil
}

// Convert a native frame time to a time.Duration
func (d *VideoDecoder) FrameTimeToDuration(pts int64) time.Duration {
	return time.Duration(C.int64_t(C.Decoder_PTSNano(d.decoder, C.int64_t(pts))))
}
