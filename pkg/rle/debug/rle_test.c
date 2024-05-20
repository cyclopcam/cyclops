/*

build & run:

cd server/videodb
gcc -g  -o rle_test debug/rle_test.c rle.c && ./rle_test

When finished:

rm rle_test

*/

#include <assert.h>
#include <stdio.h>
#include <string.h>
#include "../rle.h"

void test_rle() {
	const char* test_cases[] = {
	    "",                       // Empty input
	    "abcdef",                 // No repetition
	    "aaaaaa",                 // All same character
	    "aaaabbbbccccaaaabb",     // Mixed characters
	    "abacabadabacaba",        // Short runs and single characters
	    "a",                      // Single character
	    "abbbbbbbbbbbbbbbbbbbbb", // Long repetition
	    "ababababababababababab", // Alternating characters
	    "aabccdee",               // Mixed with short repetitions
	    "aabbccddeeffgghhiijjkk"  // Multiple short runs
	};
	const size_t num_test_cases = sizeof(test_cases) / sizeof(test_cases[0]);

	for (size_t i = 0; i < num_test_cases; i++) {
		const unsigned char* original      = (const unsigned char*) test_cases[i];
		size_t               original_size = strlen((const char*) original);

		unsigned char compressed[256];
		size_t        compressed_size = rle_compress(original, original_size, compressed);

		unsigned char decompressed[256];
		int           decompressed_size = rle_decompress(compressed, compressed_size, decompressed, sizeof(decompressed));

		assert(decompressed_size == (int) original_size);
		assert(memcmp(original, decompressed, original_size) == 0);
	}

	// Test cases to stress buffer overflow
	{
		const unsigned char* original      = (const unsigned char*) "aaaaaa";
		size_t               original_size = strlen((const char*) original);

		unsigned char compressed[256];
		size_t        compressed_size = rle_compress(original, original_size, compressed);

		unsigned char decompressed[100];
		assert(-1 == rle_decompress(compressed, compressed_size, decompressed, 0));
		assert(-1 == rle_decompress(compressed, compressed_size, decompressed, 5));
		assert(6 == rle_decompress(compressed, compressed_size, decompressed, 6));
		assert(6 == rle_decompress(compressed, compressed_size, decompressed, 7));
		assert(original_size == 6);
	}
	{
		const unsigned char* original      = (const unsigned char*) "abacabadabacaba";
		size_t               original_size = strlen((const char*) original);

		unsigned char compressed[256];
		size_t        compressed_size = rle_compress(original, original_size, compressed);

		unsigned char decompressed[100];
		assert(-1 == rle_decompress(compressed, compressed_size, decompressed, 14));
		assert(15 == rle_decompress(compressed, compressed_size, decompressed, 15));
		assert(15 == rle_decompress(compressed, compressed_size, decompressed, 16));
		assert(original_size == 15);
	}

	// Test larger buffers
	{
		unsigned char original[1024];
		unsigned char decompressed[1024];
		unsigned char compressed[2000];
		for (int sample = 0; sample < 2; sample++) {
			if (sample == 0) {
				for (int i = 0; i < 1024; i++)
					original[i] = i;
			} else if (sample == 1) {
				for (int i = 0; i < 1024; i++)
					original[i] = i / 8;
			}
			size_t compressed_size = rle_compress(original, sizeof(original), compressed);
			assert(compressed_size <= rle_compress_max_output_size(sizeof(original)));
			size_t decompressed_size = rle_decompress(compressed, compressed_size, decompressed, sizeof(decompressed));
			assert(decompressed_size == sizeof(original));
			//printf("Sample %d, %d -> %d (max %d)\n", sample, (int) sizeof(original), (int) compressed_size, (int) rle_compress_max_output_size(sizeof(original)));
		}
	}

	// Test rle_compress_max_output_size
	{
		assert(rle_compress_max_output_size(0) == 0);
		assert(rle_compress_max_output_size(1) == 2);
		assert(rle_compress_max_output_size(2) == 3);
		assert(rle_compress_max_output_size(126) == 127);
		assert(rle_compress_max_output_size(127) == 128);
		assert(rle_compress_max_output_size(128) == 130); // 2 chunks of 127 bytes each

		unsigned char compressed[256];
		size_t        compressed_size = rle_compress("a", 1, compressed);
		assert(compressed_size == rle_compress_max_output_size(1));
	}

	printf("All tests passed.\n");
}

int main() {
	test_rle();
	return 0;
}
