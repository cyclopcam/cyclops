#ifndef RLE_H
#define RLE_H

#include <stddef.h>

size_t rle_compress_max_output_size(size_t input_size);
size_t rle_compress(const unsigned char* input, size_t input_size, unsigned char* output);
size_t rle_decompress(const unsigned char* input, size_t input_size, unsigned char* output, size_t output_size);

#endif // RLE_H
