#include "varint.h"

int varint_encode_uint32(unsigned int value, unsigned char* output) {
	unsigned char* p = output;
	while (value >= 0x80) {
		*p++ = (value & 0x7F) | 0x80;
		value >>= 7;
	}
	*p = value;
	return p - output + 1;
}

unsigned int varint_decode_uint32(const unsigned char* input, size_t input_size, size_t* len) {
	unsigned int value = 0;
	unsigned int shift = 0;
	size_t       _len  = 0;
	for (size_t i = 0; i < input_size; i++) {
		unsigned char byte = input[i];
		value |= (byte & 0x7F) << shift;
		shift += 7;
		_len += 1;
		if ((byte & 0x80) == 0) {
			*len = _len;
			return value;
		}
		if (i == 5) {
			// The value is too large to be represented as a uint.
			break;
		}
	}
	*len = _len;
	return -1;
}

int varint_encode_sint32(int value, unsigned char* output) {
	return varint_encode_uint32(zigzag_encode_int32(value), output);
}

int varint_decode_sint32(const unsigned char* input, size_t input_size, size_t* len) {
	unsigned int uvalue = varint_decode_uint32(input, input_size, len);
	if (uvalue == -1) {
		return -1;
	}
	return zigzag_decode_int32(uvalue);
}
