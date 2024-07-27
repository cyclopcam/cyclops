#include <stdint.h>
#include <stdio.h>
#include <sys/time.h>

#define CHAR_MAX 255

#include "Simd/SimdLib.hpp"
#include "Simd/SimdMotion.hpp"

// g++ -O2 -fopenmp -I../../Simd/src -o motiondetect motiondetect.cpp -L../../Simd/build -lSimd -lgomp && ./motiondetect 320 240

//typedef Simd::View<Simd::Allocator> View;

int64_t timeInMilliseconds(void) {
	struct timeval tv;

	gettimeofday(&tv, NULL);
	return (((int64_t) tv.tv_sec) * 1000) + (tv.tv_usec / 1000);
}

double SecondsSince(int64_t ms) {
	return (timeInMilliseconds() - ms) / 1000.0;
}

void BenchmarkMotionDetect(int width, int height) {
	uint8_t* bg     = (uint8_t*) malloc(width * height);
	uint8_t* frame1 = (uint8_t*) malloc(width * height);
	uint8_t* frame2 = (uint8_t*) malloc(width * height);
	uint8_t* frame3 = (uint8_t*) malloc(width * height);

	//using namespace Simd;
	using namespace Simd::Motion;
	Detector detector;

	int64_t start;
	int     nframes = 2000;

	for (int i = 0; i < nframes + 1; i++) {
		if (i == 1) {
			start = timeInMilliseconds();
		}
		uint8_t* in;
		if (i % 3 == 0) {
			in = frame1;
		} else if (i % 3 == 1) {
			in = frame2;
		} else {
			in = frame3;
		}
		Metadata metadata;
		View     v(width, height, View::Gray8, in);
		Frame    input(v, false, i * 0.1);
		detector.NextFrame(v, metadata);
	}

	double elapsed = SecondsSince(start);
	printf("Time per %d x %d frame: %.3f ms\n", width, height, 1000 * elapsed / nframes);

	free(bg);
	free(frame1);
	free(frame2);
	free(frame3);
}

int main(int argc, char** argv) {
	if (argc < 3) {
		printf("Usage: %s <width> <height>\n", argv[0]);
		return 1;
	}
	int width  = atoi(argv[1]);
	int height = atoi(argv[2]);

	BenchmarkMotionDetect(width, height);
}
