#ifndef VARINT_H
#define VARINT_H

#include <stddef.h>

int          varint_encode_uint(unsigned int value, unsigned char* output);
unsigned int varint_decode_uint(const unsigned char* input, size_t input_size, size_t* len);

#endif
