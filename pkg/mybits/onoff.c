#include <stddef.h>
#include <string.h>
#include "varint.h"
#include "bit.h"

// OnOff encodes a stream of bits into a sequence of numbers,
// where every number specifies the length of the run of bits, either 0 or 1.
// You can think of this as RLE compression, but applied to a stream of words
// who's size is 1 bit. And because the stream is binary, we don't need to
// tell the decoder whether the next run is 0s or 1s, because that changes
// with every token.
// Some input/output examples:
// 0001111 -> 3,4
// 0000000 -> 7
// 0000001 -> 6,1
// 1001000 -> 0,1,2,1,3
// The initial state of the encoder is '0', so if the first bit is '1',
// the first number that we output will be 0, to switch the state from '0' to '1'.
// The numbers that we emit are varints.
//
// onoff_encode_2 was an abandoned experiment where each 8-bit symbol would either
// be a run of 1s or 0s, or it would be a run of raw bytes. This ended up providing
// worse compression on average, than onoff_encode_1.
//
// With onoff_encode_3, we discard the idea from version 2 of encoding raw bytes,
// and our fundamental change here is to encode the run lengths as 4-bit symbols
// instead of 8 bit symbols. This means that long runs take more symbols, but we
// also pay less for short runs. This ends up paying off in practice, and reduce
// the average encoded size on our dataset. It also reduces the maximum encoded
// size on our dataset.

// Return the maximum number of bits required to encode an input of the given bit length
size_t onoff_encode_3_max_output_size(size_t input_bit_size) {
	// 1 in case first bit is 'on', 4 for each additional bit, if the pattern is 1010101010101010...
	return 1 + 4 * input_bit_size;
}

// Version 3.
// This version uses 4-bit nibbles instead of byte-based variants.
// Returns the number of bytes of output, or -1 if the output buffer is too small.
size_t onoff_encode_3(const unsigned char* input, size_t input_bit_size, unsigned char* output, size_t output_byte_size) {
	size_t        s            = 0;
	size_t        i            = 0;
	size_t        j            = 0;
	int           state        = 0; // assume the first run is 0s
	unsigned char varintbuf[5] = {0};
	for (; i <= input_bit_size; i++) {
		if (i == input_bit_size || getbit(input, i) != state) {
			if (j + 11 > output_byte_size) {
				return -1;
			}
			varint_encode_4b_uint32(i - s, output, &j);
			state = !state;
			s     = i;
		}
	}
	return (j + 1) / 2;
}

// Version 3
// Returns the number of BITS written to the output buffer.
// Returns -1 if the output buffer is not large enough.
size_t onoff_decode_3(const unsigned char* input, size_t input_byte_size, unsigned char* output, size_t output_byte_size) {
	size_t i                 = 0;
	size_t j                 = 0;
	size_t zeroed            = 0; // high-water mark of the byte that we have zeroed up to
	int    state             = 0; // must match initial state in onoff_encode
	size_t input_nibble_size = input_byte_size * 2;
	while (i < input_nibble_size) {
		unsigned int nbits = varint_decode_4b_uint32(input, input_nibble_size, &i);

		// zero out new bytes before writing (or not writing) bits into them
		size_t top = (j + nbits + 7) / 8;
		if (top > output_byte_size) {
			return -1;
		}
		if (top - zeroed > 4) {
			memset(output + zeroed, 0, top - zeroed);
		} else {
			for (size_t zero = zeroed; zero < top; zero++) {
				output[zero] = 0;
			}
		}
		zeroed = top;

		if (state) {
			setbits(output, j, nbits);
		}
		j += nbits;
		state = !state;
	}
	return j;
}
