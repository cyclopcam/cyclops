#include <string.h>
#include "bit.h"

// Set a range of bits
// This is more efficient than setting bits one by one, when 'len' spans multiple bytes.
void setbits(unsigned char* input, size_t i, size_t len) {
	size_t start          = i;
	size_t end            = i + len;
	size_t firstWholeByte = (start + 7) / 8;
	size_t lastWholeByte  = end / 8;
	if (firstWholeByte * 8 > start) {
		size_t startCap = firstWholeByte * 8 < end ? firstWholeByte * 8 : end;
		for (size_t i = start; i < startCap; i++) {
			setbit(input, i);
		}
	}

	if (lastWholeByte > firstWholeByte) {
		if (lastWholeByte - firstWholeByte > 4) {
			memset(input + firstWholeByte, 0xff, lastWholeByte - firstWholeByte);
		} else {
			for (size_t i = firstWholeByte; i < lastWholeByte; i++) {
				input[i] = 0xff;
			}
		}
	}

	if (lastWholeByte * 8 < end && firstWholeByte <= lastWholeByte) {
		size_t endCap = lastWholeByte * 8 > start ? lastWholeByte * 8 : start;
		for (size_t i = endCap; i < end; i++) {
			setbit(input, i);
		}
	}
}

// Compute the binary AND of 'a' and 'b', and return the number of bits set to 1.
size_t andbits(unsigned char* a, unsigned char* b, size_t bytesLength) {
	size_t total = 0;
	for (size_t i = 0; i < bytesLength; i++) {
		// Compute the AND of the corresponding bytes...
		unsigned char result = a[i] & b[i];
		// ... and add in the number of bits set to 1.
		total += __builtin_popcount((unsigned int) result);
	}
	return total;
}

void bitmap_fillrect(unsigned char* bitmap, int width, int x, int y, int w, int h) {
	if (width & 7) {
		return;
	}
	size_t         stride = width >> 3;
	unsigned char* p      = bitmap + y * stride;
	int            y2     = y + h;
	for (; y < y2; y++) {
		setbits(p, x, w);
		p += stride;
	}
}
