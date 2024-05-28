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

size_t onoff_encode_max_output_size(size_t input_bit_size) {
	// 1 in case first bit is 'on', 8 for each additional bit, if the pattern is 1010101010101010...
	return 1 + 8 * input_bit_size;
}

// Note that input size is specified in BITS, not bytes.
// The encoded stream is a sequence of varints, where each varint is the length of a run of 0s or 1s.
// Our initial state is 1, so the first varint is the length of the first run of 0s. This will be
// zero if the first bit is a 1.
// Returns the number of bytes written to the output buffer.
// Returns -1 if the output buffer is not large enough.
size_t onoff_encode(const unsigned char* input, size_t input_bit_size, unsigned char* output, size_t output_byte_size) {
	size_t        s     = 0;
	size_t        i     = 0;
	size_t        j     = 0;
	int           state = 0; // assume the first run is 0s (if true, this saves us 1 byte)
	unsigned char varintbuf[5];
	for (; i <= input_bit_size; i++) {
		if (i == input_bit_size || getbit(input, i) != state) {
			int len = varint_encode_uint(i - s, varintbuf);
			if (j + len > output_byte_size) {
				return -1;
			}
			for (int k = 0; k < len; k++) {
				output[j++] = varintbuf[k];
			}
			state = !state;
			s     = i;
		}
	}
	return j;
}

// Returns the number of BITS written to the output buffer.
// Returns -1 if the output buffer is not large enough.
size_t onoff_decode(const unsigned char* input, size_t input_byte_size, unsigned char* output, size_t output_byte_size) {
	size_t i      = 0;
	size_t j      = 0;
	size_t zeroed = 0; // high-water mark of the byte that we have zeroed up to
	int    state  = 0; // must match initial state in onoff_encode
	while (i < input_byte_size) {
		size_t       varintlen = 1;
		unsigned int nbits     = varint_decode_uint(input + i, input_byte_size - i, &varintlen);
		if (nbits == -1) {
			return -1;
		}
		i += varintlen;

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
