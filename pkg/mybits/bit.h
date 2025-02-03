#ifndef BIT_H
#define BIT_H

#include <stddef.h>

inline static int getbit(const unsigned char* input, size_t i) {
	return (input[i / 8] >> (i % 8)) & 1;
}

inline static void setbit(unsigned char* input, size_t i) {
	input[i / 8] |= 1 << (i % 8);
}

void   setbits(unsigned char* input, size_t i, size_t len);
size_t andbits_count(unsigned char* a, unsigned char* b, size_t bytesLength);
int    andbits_nonzero(unsigned char* a, unsigned char* b, size_t bytesLength);

void bitmap_fillrect(unsigned char* bitmap, int width, int x, int y, int w, int h);

#endif // BIT_H