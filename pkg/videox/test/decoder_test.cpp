#include <stdio.h>
#include <string>
#include "decoder2.h"
#include "annexb.h"
#include "tsf.hpp"

// Step 1:
// cd pkg/videox

// Build and run:
// clang++    -O2 -fsanitize=address -std=c++17 -I. -I/usr/local/include -L/usr/local/lib -lavformat -lavcodec -lavutil -o test/decoder_test test/decoder_test.cpp decoder2.cpp annexb.cpp && ./test/decoder_test
// clang++ -g -O0 -fsanitize=address -std=c++17 -I. -I/usr/local/include -L/usr/local/lib -lavformat -lavcodec -lavutil -o test/decoder_test test/decoder_test.cpp decoder2.cpp annexb.cpp && ./test/decoder_test

// Debug build:
// clang++ -g -O0 -fsanitize=address -std=c++17 -I. -I/usr/local/include -L/usr/local/lib -lavformat -lavcodec -lavutil -o test/decoder_test test/decoder_test.cpp decoder2.cpp annexb.cpp

using namespace std;

string GetErr(char* e) {
	if (e == nullptr)
		return "";
	string err = e;
	free(e);
	return err;
}

bool IsFramePopulated(AVFrame* img, int expectWidth, int expectHeight) {
	if (img->width != expectWidth)
		return false;
	if (img->height != expectHeight)
		return false;
	if (img->format != AV_PIX_FMT_YUV420P)
		return false;
	//if (img.U == nullptr)
	//	return false;
	//if (img.V == nullptr)
	//	return false;
	//if (img.YStride < img.Width)
	//	return false;
	//if (img.UStride < img.Width / 2)
	//	return false;
	//if (img.VStride < img.Width / 2)
	//	return false;
	return true;
}

// Encode a packet in AVC format (length prefix) to Annex-B format.
// Assume length prefixes are 4 bytes.
std::string EncodeAVCToAnnexB(const void* src, size_t srcSize, std::string& out) {
	const uint8_t* s = (const uint8_t*) src;
	out.resize(8 + srcSize * 110 / 100); // something fishy if we're growing by more than 8 + 10% bytes
	uint8_t* outP   = (uint8_t*) out.data();
	uint8_t* outEnd = outP + out.size();
	while (srcSize) {
		if (srcSize < 4)
			return "packet size too small";
		size_t length = (s[0] << 24) | (s[1] << 16) | (s[2] << 8) | s[3];
		if (srcSize < length + 4)
			return "packet size with payload too small";
		*outP++        = 0;
		*outP++        = 0;
		*outP++        = 0;
		*outP++        = 1;
		size_t written = EncodeAnnexB(s + 4, length, outP, outEnd - outP);
		if (written == 0)
			return "EncodeAnnexB out of space";
		outP += written;
		s += length + 4;
		srcSize -= length + 4;
	}
	out.resize(outP - (uint8_t*) out.data());
	return "";
}

int main(int argc, char** argv) {
	string err;
	//printf("Hello\n");
	void* decoder = nullptr;

	// Fail to open a non-existent file
	err = GetErr(MakeDecoder("foo.mp4", nullptr, &decoder));
	//tsf::print("%v\n", err);
	assert(err.find("No such file") != string::npos);

	// Open a real file
	err = GetErr(MakeDecoder("../../testdata/tracking/0001-LD.mp4", nullptr, &decoder));
	assert(err == "");
	int width, height;
	Decoder_VideoSize(decoder, &width, &height);
	assert(width == 320);
	assert(height == 240);
	// Decode frames
	int nframes = 0;
	while (true) {
		AVFrame* img;
		err = GetErr(Decoder_NextFrame(decoder, &img));
		if (err == "EOF")
			break;
		nframes++;
		assert(IsFramePopulated(img, width, height));
	}
	// To get the true number of frames in a video, do this:
	// ffmpeg -i ../../testdata/tracking/0001-LD.mp4 -map 0:v:0 -c copy -f null - 2>&1 | grep "frame="
	//tsf::print("nframes %v\n", nframes);
	assert(nframes == 64);
	Decoder_Close(decoder);

	// Repeat the same test, but this time we read raw packets out of the file,
	// and pass them into a 2nd decoder. The 2nd decoder is testing our streaming
	// API, which is what gets used when decoding live video from a camera.
	err = GetErr(MakeDecoder("../../testdata/tracking/0001-LD.mp4", nullptr, &decoder));
	assert(err == "");

	void* decoder2 = nullptr;
	err            = GetErr(MakeDecoder(nullptr, "h264", &decoder2));
	assert(err == "");

	// There's a wrinkle here, which is that mp4 files store packets in AVC format,
	// which is length-prefix. If you look at the first 4 bytes of the first packet,
	// it is 00 00 00 22. The 22 is the length of the packet.
	// We need to convert these into annex-b format for our decoder. Apparently
	// it might be possible to tell the codec to accept length-prefixed packets,
	// but since annex-b is what I've had to deal with coming from cameras so far,
	// I'd rather stick with that for the unit test.

	nframes = 0;
	while (true) {
		void*  packet;
		size_t packetSize;
		err = GetErr(Decoder_NextPacket(decoder, &packet, &packetSize));
		if (err == "EOF")
			break;
		string packetB;
		err = EncodeAVCToAnnexB(packet, packetSize, packetB);
		free(packet);
		assert(err == "");
		nframes++;
		AVFrame* img;
		err = GetErr(Decoder_DecodePacket(decoder2, packetB.data(), packetB.size(), &img));
		assert(IsFramePopulated(img, width, height));
	}
	assert(nframes == 64);
	Decoder_Close(decoder);
	Decoder_Close(decoder2);

	tsf::print("decoder tests passed\n");
	return 0;
}