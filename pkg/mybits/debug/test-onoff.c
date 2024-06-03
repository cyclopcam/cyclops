#include <stdio.h>
#include <assert.h>
#include <string.h>
#include <malloc.h>
#include "../onoff.h"
#include "../bit.h"

// Test:
// cd pkg/mybits
// gcc -I. -o test-onoff debug/test-onoff.c onoff.c varint.c bit.c && ./test-onoff
//
// Debug:
// cd pkg/mybits
// gcc -g -O0 -I. -o test-onoff debug/test-onoff.c onoff.c varint.c bit.c

typedef struct {
	unsigned char* input;
	size_t         input_size_bits;
	unsigned char* output;
	size_t         output_size;
} Testcase;

void AssertMemEqual(const void* actual, const void* expected, size_t size) {
	if (memcmp(actual, expected, size) != 0) {
		printf("Expected: ");
		for (int i = 0; i < size; i++) {
			printf("%02x ", ((unsigned char*) expected)[i]);
		}
		printf("\n");
		printf("Actual:   ");
		for (int i = 0; i < size; i++) {
			printf("%02x ", ((unsigned char*) actual)[i]);
		}
		printf("\n");
		assert(0);
	}
}

void TestOnoff() {
	// Big cases are important to stress the varint encoding (specifically runs more than 127 bits)
	unsigned char* big1 = (unsigned char*) malloc(1000);
	memset(big1, 0, 1000);
	big1[0]   = 0xff;
	big1[999] = 0xff;

	unsigned char* big2 = (unsigned char*) malloc(200);
	memset(big2, 0, 200);
	for (int i = 3; i < 200; i++) {
		big2[i] = 0xff;
	}

	Testcase testcases[] = {
	    // Remember that within each byte, the bit patterns go left to right,
	    // so a run of 5 contiguous bits split across two bytes looks like:
	    // 11100000 00000011
	    //
	    // If a test case has a NULL output, then we don't verify it's output,
	    // but we still verify the encode/decode roundtrip.
	    // These known encoding cases are for encode_!
	    //{(unsigned char[]){0b00000000}, 8, (unsigned char[]){0x08}, 1},
	    //{(unsigned char[]){0b11111111}, 8, (unsigned char[]){0x00, 0x08}, 2},
	    //{(unsigned char[]){0b00111110}, 8, (unsigned char[]){0x01, 0x05, 0x02}, 3},
	    //{(unsigned char[]){0b11111000, 0b00000011}, 16, (unsigned char[]){0x03, 0x07, 0x06}, 3},
	    //{(unsigned char[]){0b00000101}, 4, (unsigned char[]){0x00, 0x01, 0x01, 0x01, 0x01}, 5},
	    //{(unsigned char[]){0b00000001}, 1, (unsigned char[]){0x00, 0x01}, 2},
	    //{(unsigned char[]){0b00000000}, 0, (unsigned char[]){0x00}, 1}, // empty buffer
	    {(unsigned char[]){0b00000000}, 8, NULL, 0},
	    {(unsigned char[]){0b11111111}, 8, NULL, 0},
	    {(unsigned char[]){0b00111110}, 8, NULL, 0},
	    {(unsigned char[]){0b11111000, 0b00000011}, 16, NULL, 0},
	    {(unsigned char[]){0b00000101}, 4, NULL, 0},
	    {(unsigned char[]){0b00000001}, 1, NULL, 0},
	    {(unsigned char[]){0b00000000}, 0, NULL, 0}, // empty buffer
	    {(unsigned char[]){0b00000001, 0b00000001, 0b00000000, 0b10000000}, 8 * 4, NULL, 0},
	    {(unsigned char[]){0x01, 0x1f, 0xff, 0x00, 0xff, 0xfe, 0xcd, 0x00, 0x00, 0xff}, 8 * 10, NULL, 0},
	    {(unsigned char[]){0x01, 0x00, 0x00, 0x00, 0xff, 0xff}, 8 * 6, NULL, 0},
	    {big1, 8 * 1000, NULL, 0},
	    {big2, 8 * 200, NULL, 0},
	};
	size_t ncases = sizeof(testcases) / sizeof(testcases[0]);

	for (int icase = 0; icase < ncases; icase++) {
		//icase       = 11;
		Testcase tc = testcases[icase];
		printf("Test case %d\n", icase);
		// Encode
		unsigned char actual_output[1000];
		size_t        actual_size = onoff_encode_3(tc.input, tc.input_size_bits, actual_output, sizeof(actual_output));
		if (tc.output != NULL) {
			assert(actual_size == tc.output_size);
			AssertMemEqual(actual_output, tc.output, tc.output_size);
			// Test encode with a buffer that is too small (it should fail)
			if (tc.output_size != 0) {
				unsigned char toosmall[100];
				size_t        fail_size = onoff_encode_3(tc.input, tc.input_size_bits, toosmall, tc.output_size - 1);
				assert(fail_size == -1);
			}
		}
		// Decode
		unsigned char actual_decode[2000];
		// fill with anything besides zero, so that we verify that our zero-fill functionality works inside the decoder
		memset(actual_decode, 0xcc, sizeof(actual_decode));
		size_t exact_original_raw_bytes = (tc.input_size_bits + 7) / 8;
		size_t decoded_bits             = onoff_decode_3(actual_output, actual_size, actual_decode, exact_original_raw_bytes);
		assert(decoded_bits == tc.input_size_bits);
		AssertMemEqual(actual_decode, tc.input, (tc.input_size_bits + 7) / 8);
		// Ensure decode fails if output buffer is too small
		if (exact_original_raw_bytes != 0) {
			decoded_bits = onoff_decode_3(actual_output, actual_size, actual_decode, exact_original_raw_bytes - 1);
			assert(decoded_bits == -1);
		}
	}

	free(big1);
	free(big2);
}

int main(int argc, char** argv) {
	TestOnoff();
	return 0;
}
