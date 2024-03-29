#include <malloc.h>
#include <string.h>
#include <assert.h>
#include <stdio.h>
#include <stdlib.h>
#include <time.h>
#include "../misc.h"

// Test
// clang -g -O0 -o misc_test pkg/videox/test/misc_test.cpp pkg/videox/misc.cpp && ./misc_test

// Benchmark
// clang -O2 -o misc_test pkg/videox/test/misc_test.cpp pkg/videox/misc.cpp && ./misc_test

void VerifyEncodeAnnexB(const char* src, size_t srcLen, const char* expectDst, size_t dstLen, int expectR) {
	size_t   encodeBufSize    = dstLen;
	size_t   encodeBufSizeRef = srcLen * 3 / 2;
	uint8_t* dst              = (uint8_t*) malloc(encodeBufSize);
	uint8_t* dstRef           = (uint8_t*) malloc(encodeBufSizeRef);
	memset(dst, 255, encodeBufSize);
	memset(dstRef, 255, encodeBufSizeRef);

	printf("Testing encode/decode %d vs %d\n", (int) srcLen, (int) dstLen);

	int r = EncodeAnnexB(src, srcLen, dst, encodeBufSize);
	if (r != expectR) {
		printf("Expected %d, got %d\n", expectR, r);
	}
	assert(r == expectR);

	int rRef = EncodeAnnexB_Ref(src, srcLen, dstRef, encodeBufSizeRef);
	if (r != 0) {
		assert(rRef == r);
		assert(0 == memcmp(dst, dstRef, r));
	}

	if (r != 0) {
		assert(0 == memcmp(expectDst, dst, dstLen));

		// Verify decode
		void* src2 = malloc(r);
		memset(src2, 255, r);
		void* src2Ref = malloc(r);
		memset(src2Ref, 255, r);
		int r2 = DecodeAnnexB(dst, r, src2, r);
		assert(r2 == srcLen);
		int r2Ref = DecodeAnnexB_Ref(dst, r, src2Ref, r);
		assert(r2Ref == srcLen);

		assert(0 == memcmp(src, src2, srcLen));
		assert(0 == memcmp(src2, src2Ref, srcLen));

		free(src2);
		free(src2Ref);
	}

	free(dst);
	free(dstRef);
}

int VerifyRoundTrip(const char* src, size_t srcLen) {
	const int dstBufSize = 30;
	char      dst[dstBufSize];
	char      dstRef[dstBufSize];
	assert(srcLen * 2 < dstBufSize);
	//memset(dst, 255, dstBufSize);

	int r = EncodeAnnexB(src, srcLen, dst, dstBufSize);
	assert(r != 0);
	int rRef = EncodeAnnexB_Ref(src, srcLen, dstRef, dstBufSize);
	assert(rRef != 0);
	assert(0 == memcmp(dst, dstRef, r));

	// Verify decode
	char src2[dstBufSize];
	//memset(src2, 255, r);
	int r2 = DecodeAnnexB(dst, r, src2, r);
	assert(r2 == srcLen);

	assert(0 == memcmp(src, src2, srcLen));

	return r;
}

void VerifyDecodeAnnexB(const char* src, size_t srcLen, const char* expectDst, int expectDstLen) {
	void* dst = malloc(srcLen);
	memset(dst, 255, srcLen);
	void* dstRef = malloc(srcLen);
	memset(dstRef, 255, srcLen);

	printf("Testing decode %d vs %d\n", (int) srcLen, (int) expectDstLen);

	int r = DecodeAnnexB(src, srcLen, dst, srcLen);
	if (r != expectDstLen) {
		printf("Expected %d, got %d\n", expectDstLen, r);
	}
	assert(r == expectDstLen);
	if (r != 0)
		assert(0 == memcmp(expectDst, dst, expectDstLen));

	int r2 = DecodeAnnexB_Ref(src, srcLen, dstRef, srcLen);
	assert(r == r2);
	assert(memcmp(dst, dstRef, r) == 0);

	free(dst);
	free(dstRef);
}

void TestRandomMutations();
void Benchmark();

int main(int argc, char** argv) {
	VerifyEncodeAnnexB("", 0, "", 0, 0);
	VerifyEncodeAnnexB("\x00", 1, "\x00", 1, 1);
	VerifyEncodeAnnexB("\x00\x00", 2, "\x00\x00", 2, 2);
	VerifyEncodeAnnexB("\x00\x00\x04", 3, "\x00\x00\x04", 3, 3);
	VerifyEncodeAnnexB("\x00\x00\x04\x00", 4, "\x00\x00\x04\x00", 4, 4);
	VerifyEncodeAnnexB("\x00\x00\x01", 3, "", 0, 0);
	VerifyEncodeAnnexB("\x00\x00\x01", 3, "", 1, 0);
	VerifyEncodeAnnexB("\x00\x00\x01", 3, "", 2, 0);
	VerifyEncodeAnnexB("\x00\x00\x01", 3, "", 3, 0);
	VerifyEncodeAnnexB("\x00\x00\x01", 3, "\x00\x00\x03\x01", 4, 4);
	VerifyEncodeAnnexB("\x00\x00\x01\x88\x99", 5, "", 5, 0);
	VerifyEncodeAnnexB("\x00\x00\x01\x88\x99", 5, "\x00\x00\x03\x01\x88\x99", 6, 6);
	VerifyEncodeAnnexB("\x00\x00\x01\x00\x00\x02", 6, "\x00\x00\x03\x01\x00\x00\x03\x02", 8, 8);
	VerifyEncodeAnnexB("\x00\x00\x00\x00\x00\x00", 6, "\x00\x00\x03\x00\x00\x03\x00\x00", 8, 8);
	//VerifyEncodeAnnexB("\x01\x00\x00\x02", 4, "", 4, 0);
	VerifyEncodeAnnexB("\x01\x00\x00\x02", 4, "\x01\x00\x00\x03\x02", 5, 5);
	VerifyEncodeAnnexB("\x00\x00\x04", 3, "\x00\x00\x04", 3, 3);
	VerifyEncodeAnnexB("\x00\x00\x00\x04", 4, "\x00\x00\x03\x00\x04", 5, 5);
	VerifyEncodeAnnexB("\x01\x00\x01\x00", 4, "\x01\x00\x01\x00", 4, 4);
	VerifyEncodeAnnexB("\x00\x00\x03", 3, "\x00\x00\x03\x03", 4, 4);

	// Incorrect:
	//VerifyDecodeAnnexB("\x00\x00\x03\x00\x00\x03\x01", 7, "\x00\x00\x00\x00\x03\x01", 6); // ensure we don't "double dip" on the 00 after the 03
	// Correct:
	VerifyDecodeAnnexB("\x00\x00\x03\x00\x00\x03\x01", 7, "\x00\x00\x00\x00\x01", 5);

	VerifyDecodeAnnexB("\x00\x00\x03\x00", 4, "\x00\x00\x00", 3);
	VerifyDecodeAnnexB("\x00\x00\x00", 3, "\x00\x00\x00", 3);
	VerifyDecodeAnnexB("\x00\x00", 2, "\x00\x00", 2);
	VerifyDecodeAnnexB("\x00", 1, "\x00", 1);
	VerifyDecodeAnnexB("", 0, "", 0);

	TestRandomMutations();
	Benchmark();
}

void Benchmark() {
	printf("Benchmark speed\n");
	srand(0);
	int   iter    = 100;
	int   rawSize = 10 * 1024 * 1024;
	int   encSize = rawSize * 3 / 2;
	char* raw     = (char*) malloc(rawSize);
	char* enc     = (char*) malloc(encSize);
	//int   fillFactor = 2;  // 2 produces 40% escaping
	//int   fillFactor = 5;  // 5 produces 5% escaping
	int fillFactor = 20; // 20 produces 0.14% escaping
	for (int i = 0; i < rawSize; i++) {
		if (rand() % fillFactor == 0)
			raw[i] = 0;
		else if (rand() % fillFactor == 0)
			raw[i] = rand() % 5;
		else
			raw[i] = rand() % 256;
	}
	auto start         = clock();
	int  actualEncSize = 0;
	for (int i = 0; i < iter; i++) {
		int r = EncodeAnnexB(raw, rawSize, enc, encSize);
		assert(r != 0);
		actualEncSize = r;
	}
	auto end = clock();

	printf("Encode MB / second: %.0f\n", (double) rawSize * iter / (end - start) * CLOCKS_PER_SEC / 1024 / 1024);

	start = clock();
	for (int i = 0; i < iter; i++) {
		int r = DecodeAnnexB(enc, actualEncSize, raw, encSize); // we're lying about the raw decode buffer size, but we know it's OK
		assert(r != 0);
	}
	end = clock();

	printf("Decode MB / second: %.0f\n", (double) rawSize * iter / (end - start) * CLOCKS_PER_SEC / 1024 / 1024);

	free(raw);
	free(enc);
}

void TestRandomMutations() {
	printf("Testing random mutations\n");
	srand(0);
	const int maxSeqLen  = 10;
	int       nTotal     = 0;
	int       nEncoded   = 0;
	int       fillFactor = 2; // 2 produces 40% escaping
	//int fillFactor = 5;  // 5 produces 5% escaping
	//int fillFactor = 20; // 20 produces 0.14% escaping
	for (int seqLen = 1; seqLen <= maxSeqLen; seqLen++) {
		for (int iter = 0; iter < 1000000; iter++) {
			char seq[maxSeqLen];
			for (int i = 0; i < seqLen; i++) {
				// The only really interesting bytes are 0,1,2,3. 4-255 are all identical from an escaping point of view.
				if (rand() % fillFactor == 0)
					seq[i] = 0;
				else if (rand() % fillFactor == 0)
					seq[i] = rand() % 5;
				else
					seq[i] = rand() % 256;
			}
			int encodedLen = VerifyRoundTrip(seq, seqLen);
			if (encodedLen != seqLen)
				nEncoded++;
			nTotal++;
		}
	}
	printf("%d/%d random mutations ended up requiring escaping\n", nEncoded, nTotal);
	assert(nEncoded > 0);
}