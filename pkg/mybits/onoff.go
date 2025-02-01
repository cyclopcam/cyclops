package mybits

import "errors"

// #include <stdint.h>
// #include <stddef.h>
// #include "onoff.h"
// #include "bit.h"
import "C"

var ErrOutOfSpace = errors.New("out of buffer space")

// Encode the given bit stream using our on/off encoding.
// Returns the number of bytes written into 'out'.
// If the resulting bit stream ends up being larger than 'out',
// then abort and return ErrOutOfSpace.
func EncodeOnoff(bits []byte, out []byte) (int, error) {
	return EncodeOnoff3(bits, out)
}

// Returns the number of BITS decoded.
// Returns ErrOutOfSpace if the decoded bit stream is larger than 'out'
func DecodeOnoff(enc []byte, out []byte) (int, error) {
	return DecodeOnoff3(enc, out)
}

// Return the maximum number of bytes required to encode an input of the given bit length
func MaxEncodedBytes(inputBitCount int) int {
	// 1 in case first bit is 'on', 4 for each additional bit, if the pattern is 1010101010101010...
	// plus 11, because it's the largest encoding of a 32-bit uint, with 4-bit nibbles.
	// The +11 is not a practical concern, but it allows our encoder to make strict guarantees.
	maxBits := 1 + 4*inputBitCount + 11
	return (maxBits + 7) / 8
}

// Final version
// Note that EncodeOnoff is a wrapper around this function.
func EncodeOnoff3(bits []byte, out []byte) (int, error) {
	if len(out) == 0 {
		return 0, ErrOutOfSpace
	}
	outputBytes := C.onoff_encode_3((*C.uint8_t)(&bits[0]), C.size_t(len(bits)*8), (*C.uint8_t)(&out[0]), C.size_t(len(out)))
	if outputBytes == C.size_t(^uintptr(0)) {
		return 0, ErrOutOfSpace
	}
	return int(outputBytes), nil
}

// Returns the number of BITS decoded.
// Returns ErrOutOfSpace if the decoded bit stream is larger than 'out'.
// Note that DecodeOnoff is a wrapper around this function.
func DecodeOnoff3(enc []byte, out []byte) (int, error) {
	if len(out) == 0 {
		return 0, ErrOutOfSpace
	}
	outputBits := C.onoff_decode_3((*C.uint8_t)(&enc[0]), C.size_t(len(enc)), (*C.uint8_t)(&out[0]), C.size_t(len(out)))
	if outputBits == C.size_t(^uintptr(0)) {
		return 0, ErrOutOfSpace
	}
	return int(outputBits), nil
}

// Fill a bitmap with a rectangle of 'on' bits.
// The bitmap's width must be a multiple of 8
func BitmapFillRect(bits []byte, width, x, y, w, h int) {
	C.bitmap_fillrect((*C.uint8_t)(&bits[0]), C.int(width), C.int(x), C.int(y), C.int(w), C.int(h))
}

// Compute the binary AND of the bitmaps 'a' and 'b', and return the number of 'on' bits.
func AndBitmaps(a, b []byte) int {
	if len(a) != len(b) {
		panic("Length of bitmaps must match")
	}
	return int(C.andbits((*C.uint8_t)(&a[0]), (*C.uint8_t)(&b[0]), C.size_t(len(a))))
}
