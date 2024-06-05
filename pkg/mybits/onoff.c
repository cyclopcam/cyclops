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
// With onoff_encode_3, we get rid of the ability to encode raw bytes, but we
// encode the run lengths as 4-bit symbols. This means that long runs take more
// symbols, but we also pay less for short runs. This ends up paying off in practice,
// and reduce the average encoded size on our dataset. It also reduces the maximum
// encoded size on our dataset.

size_t onoff_encode_max_output_size(size_t input_bit_size) {
	// 1 in case first bit is 'on', 8 for each additional bit, if the pattern is 1010101010101010...
	return 1 + 8 * input_bit_size;
}

// Return 1 if the byte is one of the following 16 bit patterns:
// 11111111
// 01111111
// 00111111
// 00011111
// 00001111
// 00000111
// 00000011
// 00000001
// 00000000
// 10000000
// 11000000
// 11100000
// 11110000
// 11111000
// 11111100
// 11111110
int is_contiguous_bit_pattern(unsigned char v) {
	return (v & (v + 1)) == 0 || (~v & (~v + 1)) == 0;
}

// Note that input size is specified in BITS, not bytes.
// The encoded stream is a sequence of varints, where each varint is the length of a run of 0s or 1s.
// Our initial state is 1, so the first varint is the length of the first run of 0s. This will be
// zero if the first bit is a 1.
// Returns the number of bytes written to the output buffer.
// Returns -1 if the output buffer is not large enough.
// NOTE: This is no longer used, in favour of onoff_encode_3
size_t onoff_encode_1(const unsigned char* input, size_t input_bit_size, unsigned char* output, size_t output_byte_size) {
	size_t        s            = 0;
	size_t        i            = 0;
	size_t        j            = 0;
	int           state        = 0; // assume the first run is 0s (if true, this saves us 1 byte)
	unsigned char varintbuf[5] = {0};
	for (; i <= input_bit_size; i++) {
		if (i == input_bit_size || getbit(input, i) != state) {
			int len = varint_encode_uint32(i - s, varintbuf);
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

// Positive (or zero) varints encode on/off bits.
// Negative varints encode runs of raw bytes
// NOTE: I abandoned this because it gives worse average performance on my test set.
size_t onoff_encode_2(const unsigned char* input, size_t input_bit_size, unsigned char* output, size_t output_byte_size) {
	size_t in_pos          = 0;
	size_t out_pos         = 0;
	size_t input_byte_size = (input_bit_size + 7) / 8;
	int    onoff_state     = 0; // assume the first run is 0s
	while (in_pos < input_byte_size) {
		// Look ahead in the byte stream to figure out if we should encode the next N bytes
		// as on/off or as raw. Our criteria is simple:
		// Bytes valid for on/off encoding must either be 0x00, 0xff, or a switch between
		// those two (eg 11100000, 00000011, etc).
		int    contig = is_contiguous_bit_pattern(input[in_pos]);
		size_t pos    = in_pos + 1;
		for (; pos <= input_byte_size; pos++) {
			if (pos == input_byte_size || is_contiguous_bit_pattern(input[pos]) != contig) {
				break;
			}
		}
		size_t run_length = pos - in_pos;
		if (run_length >= 3 && contig) {
			// encode on/off
			size_t        i            = in_pos * 8;         // i is bit position
			size_t        s            = i;                  // bit position of start of run
			size_t        stop         = i + run_length * 8; // bit position where we must stop
			unsigned char varintbuf[5] = {0};
			for (; i <= stop; i++) {
				if (i == stop || getbit(input, i) != onoff_state) {
					int len = varint_encode_sint32((int) (i - s), varintbuf); // positive values (and zero) encode on/off runs
					if (out_pos + len > output_byte_size) {
						return -1;
					}
					for (int k = 0; k < len; k++) {
						output[out_pos++] = varintbuf[k];
					}
					onoff_state = !onoff_state;
					s           = i;
				}
			}
		} else {
			// encode raw
			unsigned char varintbuf[5] = {0};
			int           len          = varint_encode_sint32((int) -run_length, output + out_pos);
			if (out_pos + len > output_byte_size) {
				return -1;
			}
			for (int k = 0; k < len; k++) {
				output[out_pos++] = varintbuf[k];
			}
			memcpy(output + out_pos, input + in_pos, run_length);
			out_pos += run_length;
		}
		in_pos = pos;
	}
	return out_pos;
}

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

// Returns the number of BITS written to the output buffer.
// Returns -1 if the output buffer is not large enough.
size_t onoff_decode_1(const unsigned char* input, size_t input_byte_size, unsigned char* output, size_t output_byte_size) {
	size_t i      = 0;
	size_t j      = 0;
	size_t zeroed = 0; // high-water mark of the byte that we have zeroed up to
	int    state  = 0; // must match initial state in onoff_encode
	while (i < input_byte_size) {
		size_t       varintlen = 1;
		unsigned int nbits     = varint_decode_uint32(input + i, input_byte_size - i, &varintlen);
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
