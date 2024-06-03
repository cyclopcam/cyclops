#ifndef ONOFF_H
#define ONOFF_H

#include <stddef.h>

size_t onoff_encode_max_output_size(size_t input_bit_size);
size_t onoff_encode_1(const unsigned char* input, size_t input_bit_size, unsigned char* output, size_t output_byte_size);
size_t onoff_decode_1(const unsigned char* input, size_t input_byte_size, unsigned char* output, size_t output_byte_size);

size_t onoff_encode_2(const unsigned char* input, size_t input_bit_size, unsigned char* output, size_t output_byte_size);

size_t onoff_encode_3(const unsigned char* input, size_t input_bit_size, unsigned char* output, size_t output_byte_size);
size_t onoff_decode_3(const unsigned char* input, size_t input_byte_size, unsigned char* output, size_t output_byte_size);

#endif // ONOFF_H