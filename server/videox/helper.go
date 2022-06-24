package videox

// #cgo pkg-config: libavcodec libavformat libavutil libswscale
// #include "helper.h"
// #include <stdio.h>
// #include <stdlib.h>
// #include <stdint.h>
import "C"
import (
	"errors"
	"io"
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

func NewVideoEncoder() (*VideoEncoder, error) {
	var cerr *C.char
	cFormat := C.CString("mp4")
	cFilename := C.CString("dump/test.mp4")
	e := C.MakeEncoder(&cerr, cFormat, cFilename, 2048, 1536)
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
