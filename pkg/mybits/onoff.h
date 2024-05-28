#ifndef ONOFF_H
#define ONOFF_H

#include <stddef.h>

size_t onoff_encode_max_output_size(size_t input_bit_size);
size_t onoff_encode(const unsigned char* input, size_t input_bit_size, unsigned char* output, size_t output_byte_size);
size_t onoff_decode(const unsigned char* input, size_t input_byte_size, unsigned char* output, size_t output_byte_size);

#endif // ONOFF_H