#include <stdio.h>
#include <assert.h>
#include <stdint.h>
#include <malloc.h>
#include "encoder.h"

// This test writes image frames to a file.

// Step 1:
// cd pkg/videox

// Build and run:
// clang++    -O2 -fsanitize=address -std=c++17 -I. -I/usr/local/include -L/usr/local/lib -lavformat -lavcodec -lavutil -lswscale -o test/encoder_test test/encoder_test.cpp encoder.cpp common.cpp && test/encoder_test
// clang++ -g -O0 -fsanitize=address -std=c++17 -I. -I/usr/local/include -L/usr/local/lib -lavformat -lavcodec -lavutil -lswscale -o test/encoder_test test/encoder_test.cpp encoder.cpp common.cpp && test/encoder_test

// Debug build:
// clang++ -g -O0 -fsanitize=address -std=c++17 -I. -I/usr/local/include -L/usr/local/lib -lavformat -lavcodec -lavutil -lswscale -o test/encoder_test test/encoder_test.cpp encoder.cpp common.cpp

// Generate an RGB frame that varies over time
void GenerateFrame(uint8_t* buf, int stride, int frameIdx, int width, int height) {
	int red   = int((100 + frameIdx * 0.7)) & 255;
	int green = int((50 + frameIdx * 1.1)) & 255;
	int blue  = int((frameIdx * 1.7)) & 255;
	int rectR = 100;
	int rectG = 200;
	int rectB = 50;
	int rectW = 20;
	int rectH = 20;
	int x1    = (frameIdx * 1) % width;
	int y1    = (frameIdx * 2) % height;
	int x2    = x1 + rectW;
	int y2    = y1 + rectH;
	for (int y = 0; y < height; y++) {
		uint8_t* p = buf + y * stride;
		for (int x = 0; x < width; x++) {
			int r = red;
			int g = green;
			int b = blue;
			if (x >= x1 && x < x2 && y >= y1 && y < y2) {
				r = rectR;
				g = rectG;
				b = rectB;
			}
			p[0] = r;
			p[1] = g;
			p[2] = b;
			p += 3;
		}
	}
}

void Check(char* e) {
	if (e == nullptr)
		return;
	printf("Error: %s\n", e);
	assert(false);
}

void TestCodec(const char* codecName) {
	int width  = 320;
	int height = 240;

	// To see a list of available encoders
	// ffmpeg -encoders | grep 264

	char filename[256];
	sprintf(filename, "out-%s.mp4", codecName);

	EncoderParams params;
	int           fps = 60;
	Check(MakeEncoderParams(codecName, width, height, AV_PIX_FMT_RGB24, AV_PIX_FMT_YUV420P, EncoderTypeImageFrames, fps, &params));
	void* encoder = nullptr;
	Check(MakeEncoder(nullptr, filename, &params, &encoder));

	for (int frameIdx = 0; frameIdx < 500; frameIdx++) {
		int64_t  ptsNano = (int64_t) frameIdx * 1000000000 / fps;
		AVFrame* frame   = nullptr;
		Check(Encoder_MakeFrameWriteable(encoder, &frame));
		GenerateFrame(frame->data[0], frame->linesize[0], frameIdx, width, height);
		Check(Encoder_WriteFrame(encoder, ptsNano));
	}
	Check(Encoder_WriteTrailer(encoder));
	Encoder_Close(encoder);

	// To really complete this test, we should run some ffprobe commands and verify their outputs.

	// Some examples:

	// ffprobe -v error -count_frames -select_streams v:0 -show_entries stream=nb_read_frames -of default=nokey=1:noprint_wrappers=1 out.mp4
	// [frame rate]

	// ffprobe -v error -count_frames -select_streams v:0 -show_entries stream=nb_read_frames -of default=nokey=1:noprint_wrappers=1 out.mp4
	// [frame count]
}

int main(int argc, char** argv) {
	TestCodec("h264");
	TestCodec("h265");
	return 0;
}