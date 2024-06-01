#ifndef VARINT_H
#define VARINT_H

#include <stddef.h>
#include <stdint.h>

static inline unsigned int zigzag_encode_int32(int value) {
	return (value << 1) ^ (value >> 31);
}

static inline int zigzag_decode_int32(unsigned int value) {
	return (value >> 1) ^ -(value & 1);
}

int          varint_encode_uint32(unsigned int value, unsigned char* output);
unsigned int varint_decode_uint32(const unsigned char* input, size_t input_size, size_t* len);

// The signed functions do zigzag encoding

int varint_encode_sint32(int value, unsigned char* output);
int varint_decode_sint32(const unsigned char* input, size_t input_size, size_t* len);

#endif
