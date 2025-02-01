#include <stdio.h>
#include <string.h>
#include <assert.h>
#include "../bit.h"

// Test:
// cd pkg/mybits
// gcc -I. -o test-bits debug/test-bits.c bit.c && ./test-bits

void TestPattern(unsigned char* buf, size_t bufsize, size_t start, size_t end) {
	memset(buf, 0, bufsize);
	setbits(buf, start, end - start);
	for (size_t i = 0; i < bufsize * 8; i++) {
		assert(getbit(buf, i) == (i >= start && i < end));
	}
}

void TestSetBits() {
	unsigned char buf[20];
	TestPattern(buf, 20, 0, 0);
	TestPattern(buf, 20, 0, 1);
	TestPattern(buf, 20, 0, 8);
	TestPattern(buf, 20, 0, 9);
	TestPattern(buf, 20, 0, 16);
	TestPattern(buf, 20, 0, 17);
	TestPattern(buf, 20, 1, 2);
	TestPattern(buf, 20, 1, 9);
	TestPattern(buf, 20, 1, 30);
	TestPattern(buf, 20, 7, 8);
	TestPattern(buf, 20, 7, 9);
	TestPattern(buf, 20, 8, 9);
	TestPattern(buf, 20, 9, 31);
	TestPattern(buf, 20, 9, 60);
}

void TestFillRectAndAND() {
	unsigned char buf1[100];
	unsigned char buf2[100];
	memset(buf1, 0, sizeof(buf1));
	memset(buf2, 0, sizeof(buf2));
	bitmap_fillrect(buf1, 32, 5, 11, 2, 2);

	// no overlap
	bitmap_fillrect(buf2, 32, 7, 11, 2, 2);
	assert(andbits(buf1, buf2, sizeof(buf1)) == 0);

	// some overlap
	bitmap_fillrect(buf2, 32, 6, 11, 2, 2);
	assert(andbits(buf1, buf2, sizeof(buf1)) == 2);
}

int main(int argc, char** argv) {
	TestSetBits();
	TestFillRectAndAND();
	return 0;
}