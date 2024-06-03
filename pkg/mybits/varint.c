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

void varint_encode_4b_uint32(unsigned int value, unsigned char* output, size_t* _i) {
	size_t i = *_i;
	while (value >= 8) {
		unsigned char nibble = (value & 7) | 8;
		if ((i & 1) == 0) {
			output[i >> 1] = nibble;
		} else {
			output[i >> 1] |= nibble << 4;
		}
		value >>= 3;
		i++;
	}
	unsigned char nibble = value & 7;
	if ((i & 1) == 0) {
		output[i >> 1] = nibble;
	} else {
		output[i >> 1] |= nibble << 4;
	}
	i++;
	*_i = i;
}

unsigned int varint_decode_4b_uint32(const unsigned char* input, size_t input_size, size_t* _i) {
	unsigned int value     = 0;
	unsigned int shift     = 0;
	size_t       initial_i = *_i;
	size_t       i         = initial_i;
	while (i < input_size) {
		unsigned char nibble = input[i >> 1];
		if ((i & 1) == 1) {
			nibble >>= 4;
		}
		i++;
		value |= (nibble & 7) << shift;
		shift += 3;
		if ((nibble & 8) == 0) {
			//*_i = i;
			//return value;
			break;
		}
		if (i - initial_i == 11) {
			// The value is too large to be represented as a uint.
			break;
		}
	}
	*_i = i;
	return value;
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
