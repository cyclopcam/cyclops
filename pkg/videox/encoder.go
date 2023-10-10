package videox

// #cgo pkg-config: libavcodec libavformat libavutil libswscale
// #include "encoder.h"
// #include <stdio.h>
// #include <stdlib.h>
// #include <stdint.h>
import "C"
import (
	"errors"
	"io"
	"time"
	"unsafe"
)

// Consume C error, by turning it into a Go error.
// Free memory used by the C string
func takeCErr(err *C.char) error {
	if err == nil {
		return nil
	}
	msg := C.GoString(err)
	C.free(unsafe.Pointer(err))
	if msg == "EOF" {
		return io.EOF
	}
	return errors.New(msg)
}

type VideoEncoder struct {
	enc unsafe.Pointer
}

// NewVideoEncoder creates a new video encoder
// You must Close() a video encoder when you are done using it, otherwise you will leak ffmpeg objects
func NewVideoEncoder(format, filename string, width, height int) (*VideoEncoder, error) {
	var cerr *C.char
	cFormat := C.CString(format)
	cFilename := C.CString(filename)
	e := C.MakeEncoder(&cerr, cFormat, cFilename, C.int(width), C.int(height))
	C.free(unsafe.Pointer(cFormat))
	C.free(unsafe.Pointer(cFilename))
	err := takeCErr(cerr)
	if err != nil {
		return nil, err
	}
	return &VideoEncoder{
		enc: e,
	}, nil
}

func (v *VideoEncoder) Close() {
	if v.enc != nil {
		C.Encoder_Close(v.enc)
		v.enc = nil
	}
}

func (v *VideoEncoder) WriteNALU(dts, pts time.Duration, nalu NALU) error {
	idts := C.int64_t(dts.Nanoseconds())
	ipts := C.int64_t(pts.Nanoseconds())
	var cerr *C.char
	C.Encoder_WriteNALU(&cerr, v.enc, idts, ipts, C.int(nalu.PrefixLen), unsafe.Pointer(&nalu.Payload[0]), C.ulong(len(nalu.Payload)))
	if err := takeCErr(cerr); err != nil {
		return err
	}
	return nil
}

func (v *VideoEncoder) WritePacket(dts, pts time.Duration, packet *DecodedPacket) error {
	idts := C.int64_t(dts.Nanoseconds())
	ipts := C.int64_t(pts.Nanoseconds())
	encoded := packet.EncodeToAnnexBPacket()
	isKeyFrame := C.int(0)
	if packet.HasIDR() {
		isKeyFrame = 1
	}
	var cerr *C.char
	C.Encoder_WritePacket(&cerr, v.enc, idts, ipts, isKeyFrame, unsafe.Pointer(&encoded[0]), C.ulong(len(encoded)))
	if err := takeCErr(cerr); err != nil {
		return err
	}
	return nil
}

func (v *VideoEncoder) WriteTrailer() error {
	var cerr *C.char
	C.Encoder_WriteTrailer(&cerr, v.enc)
	return takeCErr(cerr)
}
