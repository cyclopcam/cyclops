#include "rle.h"

// Compress into chunks.
// Each chunk is 0..127 bytes long.
// The first byte of the chunk (excluding the high bit) specifies the length N.
// The high bit of the first byte of the chunk specifies whether this is a run-length encoded chunk, or a raw chunk.
// A run-length encoded chunk is followed by a single byte that is repeated N times.
// A raw chunk is followed by N bytes of raw data.

#define MAX_CHUNK_SIZE 127

size_t rle_compress_max_output_size(size_t input_size) {
	// Figure out the number of chunks we'd need to represent 100% raw data.
	// Each chunk has 1 byte of header.
	// So our final answer is the number of chunks + the number of raw bytes.
	return ((input_size + MAX_CHUNK_SIZE - 1) / MAX_CHUNK_SIZE) + input_size;
}

size_t rle_compress(const unsigned char* input, size_t input_size, unsigned char* output) {
	size_t i, j = 0;
	for (i = 0; i < input_size;) {
		size_t run_length = 1;
		while (i + run_length < input_size && input[i] == input[i + run_length] && run_length < MAX_CHUNK_SIZE) {
			run_length++;
		}

		if (run_length > 1) {
			output[j++] = 0x80 | (unsigned char) run_length;
			output[j++] = input[i];
			i += run_length;
		} else {
			size_t raw_length = 0;
			size_t raw_start  = j++;
			while (i + raw_length < input_size && raw_length < MAX_CHUNK_SIZE &&
			       (i + raw_length + 1 >= input_size || input[i + raw_length] != input[i + raw_length + 1])) {
				output[j++] = input[i + raw_length++];
			}
			output[raw_start] = (unsigned char) raw_length;
			i += raw_length;
		}
	}
	return j;
}

size_t rle_decompress(const unsigned char* input, size_t input_size, unsigned char* output, size_t output_size) {
	size_t i, j = 0;
	for (i = 0; i < input_size;) {
		unsigned char header = input[i++];
		size_t        count  = (header & 0x7F);

		if (header & 0x80) {
			// RLE chunk
			if (j + count > output_size) {
				// Buffer overflow
				return -1;
			}
			unsigned char value = input[i++];
			for (size_t k = 0; k < count; k++) {
				output[j++] = value;
			}
		} else {
			// Raw chunk
			if (j + count > output_size) {
				// Buffer overflow
				return -1;
			}
			for (size_t k = 0; k < count; k++) {
				output[j++] = input[i++];
			}
		}
	}
	return j;
}
