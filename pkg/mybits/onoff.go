package mybits

import "errors"

// #include <stdint.h>
// #include <stddef.h>
// #include "onoff.h"
import "C"

var ErrOutOfSpace = errors.New("out of buffer space")

// Encode the given bit stream using out on/off encoding.
// Returns the number of bytes written into 'out'.
// If the resulting bit stream ends up being larger than 'out',
// then abort and return ErrOutOfSpace.
func EncodeOnoff(bits []byte, out []byte) (int, error) {
	if len(out) == 0 {
		return 0, ErrOutOfSpace
	}
	outputBytes := C.onoff_encode((*C.uint8_t)(&bits[0]), C.size_t(len(bits)*8), (*C.uint8_t)(&out[0]), C.size_t(len(out)))
	if outputBytes == C.size_t(^uintptr(0)) {
		return 0, ErrOutOfSpace
	}
	return int(outputBytes), nil
}

// Returns the number of BITS decoded.
// Returns ErrOutOfSpace if the decoded bit stream is larger than 'out'
func DecodeOnoff(enc []byte, out []byte) (int, error) {
	if len(out) == 0 {
		return 0, ErrOutOfSpace
	}
	outputBits := C.onoff_decode((*C.uint8_t)(&enc[0]), C.size_t(len(enc)), (*C.uint8_t)(&out[0]), C.size_t(len(out)))
	if outputBits == C.size_t(^uintptr(0)) {
		return 0, ErrOutOfSpace
	}
	return int(outputBits), nil
}
