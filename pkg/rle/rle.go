package rle

// Package rle provides a simple run-length encoding (RLE) implementation.

// #include <stdint.h>
// #include <stddef.h>
// #include "rle.h"
import "C"
import (
	"errors"
	"unsafe"
)

var ErrNotEnoughSpace = errors.New("Not enough space in decompression buffer")

func Compress(data []byte) []byte {
	if len(data) == 0 {
		return nil
	}
	outputSize := int(C.rle_compress_max_output_size(C.size_t(len(data))))
	output := make([]byte, outputSize)
	outputSize = int(C.rle_compress((*C.uint8_t)(unsafe.Pointer(&data[0])), C.size_t(len(data)), (*C.uint8_t)(unsafe.Pointer(&output[0]))))
	return output[:outputSize]
}

// Upon success, returns (decompressed size, nil).
// Upon failure, returns (0, error).
func Decompress(compressed []byte, decompressed []byte) (int, error) {
	if len(compressed) == 0 {
		return 0, nil
	}
	if len(decompressed) == 0 {
		return 0, ErrNotEnoughSpace
	}
	outputSize := int(C.rle_decompress((*C.uint8_t)(unsafe.Pointer(&compressed[0])), C.size_t(len(compressed)),
		(*C.uint8_t)(unsafe.Pointer(&decompressed[0])), C.size_t(len(decompressed))))
	if outputSize == -1 {
		return 0, ErrNotEnoughSpace
	}
	return outputSize, nil
}
