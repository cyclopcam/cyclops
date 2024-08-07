#include <stddef.h>
#include <string.h>
#include "varint.h"
#include "bit.h"

//////////////////////////////////////////////////////////////////////////////
//////////////////////////////////////////////////////////////////////////////
// This was experimental code. Not used.
//////////////////////////////////////////////////////////////////////////////
//////////////////////////////////////////////////////////////////////////////

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
