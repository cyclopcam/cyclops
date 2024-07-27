#ifndef ONOFF_ARCHIVE_H
#define ONOFF_ARCHIVE_H

//////////////////////////////////////////////////////////////////////////////
//////////////////////////////////////////////////////////////////////////////
// This was experimental code. Not used.
//////////////////////////////////////////////////////////////////////////////
//////////////////////////////////////////////////////////////////////////////

#include <stddef.h>

size_t onoff_encode_1(const unsigned char* input, size_t input_bit_size, unsigned char* output, size_t output_byte_size);
size_t onoff_decode_1(const unsigned char* input, size_t input_byte_size, unsigned char* output, size_t output_byte_size);

size_t onoff_encode_2(const unsigned char* input, size_t input_bit_size, unsigned char* output, size_t output_byte_size);

#endif // ONOFF_ARCHIVE_H