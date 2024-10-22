package accel

// accel contains some functions that we can write faster in C/C++ than in Go

// #cgo CPPFLAGS: -fopenmp -I${SRCDIR}/../../Simd/src
// #cgo LDFLAGS: -L${SRCDIR}/../../Simd/build -lSimd -lgomp
// #include <stdio.h>
// #include <stdlib.h>
// #include "accel.h"
import "C"
import "unsafe"

// CAVEAT!
// We are not paying attention to the difference between AV_PIX_FMT_YUV420P and AV_PIX_FMT_YUVJ420P.
// This may be the cause of colors looking washed out.

// void YUV420pToRGB(int width, int height, const uint8_t* y, const uint8_t* u, const uint8_t* v, int strideY, int strideU, int strideV, uint8_t* rgb, int strideRGB) {
func YUV420pToRGB(width, height int, y, u, v []byte, strideY, strideU, strideV, strideRGB int, rgb []byte) {
	C.YUV420pToRGB(C.int(width), C.int(height),
		(*C.uint8_t)(unsafe.Pointer(&y[0])), (*C.uint8_t)(unsafe.Pointer(&u[0])), (*C.uint8_t)(unsafe.Pointer(&v[0])),
		C.int(strideY), C.int(strideU), C.int(strideV),
		(*C.uint8_t)(unsafe.Pointer(&rgb[0])), C.int(strideRGB))
}
