#include <stdio.h>
#include <string>
#include "decoder.h"
#include "encoder.h"
#include "annexb.h"
#include "h264ParseSPS.h"
#include "tsf.hpp"

// Step 1:
// cd pkg/videox

// Build and run:
// clang++    -O2 -fsanitize=address -std=c++17 -I. -I/usr/local/include -L/usr/local/lib -lavformat -lavcodec -lavutil -o test/decoder_test test/decoder_test.cpp decoder.cpp annexb.cpp common.cpp h264ParseSPS.cpp && ./test/decoder_test
// clang++ -g -O0 -fsanitize=address -std=c++17 -I. -I/usr/local/include -L/usr/local/lib -lavformat -lavcodec -lavutil -o test/decoder_test test/decoder_test.cpp decoder.cpp annexb.cpp common.cpp h264ParseSPS.cpp && ./test/decoder_test

// Debug build:
// clang++ -g -O0 -fsanitize=address -std=c++17 -I. -I/usr/local/include -L/usr/local/lib -lavformat -lavcodec -lavutil -o test/decoder_test test/decoder_test.cpp decoder.cpp annexb.cpp common.cpp h264ParseSPS.cpp

using namespace std;

string GetErr(char* e) {
	if (e == nullptr)
		return "";

	// SYNC-SPECIAL-FFMPEG-ERRORS
	if ((intptr_t) e == 1)
		return "EOF";
	else if ((intptr_t) e == 2)
		return "EAGAIN";

	string err = e;
	free(e);
	return err;
}

void AssertNoError(std::string err) {
	if (err != "") {
		tsf::print("Unexpected error: %v\n", err);
		assert(false);
	}
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
std::string EncodeAVCToAnnexB(bool escapeStartCodes, MyCodec codec, const void* src, size_t srcSize, std::string& out) {
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
		//DumpNALUHeader(codec, NALU{s + 4, length});
		*outP++ = 0;
		*outP++ = 0;
		*outP++ = 1;
		if (escapeStartCodes) {
			size_t written = EncodeAnnexB(s + 4, length, outP, outEnd - outP);
			if (written == 0)
				return "EncodeAnnexB out of space";
			outP += written;
		} else {
			if (outEnd - outP < length)
				return "out of space";
			memcpy(outP, s + 4, length);
			outP += length;
		}
		s += length + 4;
		srcSize -= length + 4;
	}
	out.resize(outP - (uint8_t*) out.data());
	return "";
}

void TestFile(std::string filename, int expectedFrameCount) {
	tsf::print("Testing %v\n", filename);

	string err;
	void*  decoder = nullptr;

	err = GetErr(MakeDecoder(filename.c_str(), nullptr, &decoder));

	tsf::print("phase 1\n");

	const char* codecName = "";
	int         width, height;
	Decoder_VideoInfo(decoder, &width, &height, &codecName);

	MyCodec codec = MyCodec::None;
	if (strcmp(codecName, "h264") == 0)
		codec = MyCodec::H264;
	else if (strcmp(codecName, "h265") == 0 || strcmp(codecName, "hevc") == 0)
		codec = MyCodec::H265;
	else
		assert(false);

	assert(width == 320);
	assert(height == 240);
	// Decode frames
	int     nframes = 0;
	int64_t ptsPrev = 0;
	while (true) {
		AVFrame* img;
		err = GetErr(Decoder_ReadAndReceiveFrame(decoder, &img));
		if (err == "EOF")
			break;
		nframes++;
		assert(IsFramePopulated(img, width, height));
		//int64_t ptsNano = Decoder_PTSNano(decoder, img->pts);
		//tsf::print("%18d %18d %18d %18d\n", ptsNano, ptsNano / 1000000, img->pts, img->pts - ptsPrev);
		//ptsPrev = img->pts;
	}
	// To get the true number of frames in a video, do this:
	// ffmpeg -i ../../testdata/tracking/0001-LD.mp4 -map 0:v:0 -c copy -f null - 2>&1 | grep "frame="
	//tsf::print("nframes %v\n", nframes);
	if (expectedFrameCount != 0)
		assert(nframes == expectedFrameCount);
	else
		assert(nframes != 0);
	Decoder_Close(decoder);

	// Repeat the same test, but this time we read raw packets out of the file,
	// and pass them into a 2nd decoder. The 2nd decoder is testing our streaming
	// API, which is what gets used when decoding live video from a camera.
	err = GetErr(MakeDecoder(filename.c_str(), nullptr, &decoder));
	AssertNoError(err);

	tsf::print("phase 2\n");

	void* decoder2 = nullptr;
	err            = GetErr(MakeDecoder(nullptr, codecName, &decoder2));
	AssertNoError(err);

	// There's a wrinkle here, which is that mp4 files store packets in AVC format,
	// which is length-prefix. If you look at the first 4 bytes of the first packet,
	// it is 00 00 00 22. The 22 is the length of the packet.
	// We need to convert these into annex-b format for our decoder. Apparently
	// it might be possible to tell the codec to accept length-prefixed packets,
	// but since annex-b is what I've had to deal with coming from cameras so far,
	// I'd rather stick with that for the unit test.
	// HOWEVER:
	// It looks like the data coming out of a .mp4 file is already escaped for annex-b.
	// For my h264 samples I suspect there were no escaped bytes, so I didn't notice.
	// But for h265, I needed to disable escaping to get the decoder to work.
	bool addEscapeBytes = false;

	// When doing this type of test, we need to decouple the frame extraction from
	// the decoding of packets. For my h264 tests, this wasn't necessary, but for
	// my h265 tests, I only get a frame out after the first 3 frames have gone in.

	nframes = 0;
	while (true) {
		void*   packet;
		size_t  packetSize;
		int64_t pts, dts;
		err = GetErr(Decoder_NextPacket(decoder, &packet, &packetSize, &pts, &dts));
		if (err == "EOF")
			break;
		string packetB;
		err = EncodeAVCToAnnexB(addEscapeBytes, codec, packet, packetSize, packetB);
		free(packet);
		AssertNoError(err);
		nframes++;
		err = GetErr(Decoder_OnlyDecodePacket(decoder2, packetB.data(), packetB.size()));
		if (err == "EAGAIN") {
			// no frame available yet
			continue;
		}

		while (true) {
			AVFrame* img;
			err = GetErr(Decoder_ReceiveFrame(decoder2, &img));
			if (err == "EAGAIN") {
				// no frame available yet
				break;
			} else if (err != "") {
				tsf::print("Decoder_DecodePacket failed: %v\n", err);
				assert(false);
			}
			assert(IsFramePopulated(img, width, height));
		}
	}
	if (expectedFrameCount != 0)
		assert(nframes == expectedFrameCount);
	else
		assert(nframes != 0);
	Decoder_Close(decoder);
	Decoder_Close(decoder2);

	tsf::print("decoder tests passed\n");
}

std::string DecodeAnnexBBuffer(const void* annexb, size_t size) {
	std::string out;
	out.resize(size);
	size_t outsize = DecodeAnnexB(annexb, size, out.data(), out.size());
	out.resize(outsize);
	return out;
}

void TestSPSDecode() {
	unsigned char h264_sps_320_240[22] = {
	    0x67, 0x4d, 0x40, 0x1e, 0x9a, 0x66, 0x0a, 0x0f,
	    0xff, 0x35, 0x01, 0x01, 0x01, 0x40, 0x00, 0x00,
	    0xfa, 0x00, 0x00, 0x13, 0x88, 0x01};
	std::string buf    = DecodeAnnexBBuffer(h264_sps_320_240, sizeof(h264_sps_320_240));
	int         width  = 0;
	int         height = 0;
	ParseH264SPS(buf.data(), buf.size(), &width, &height);
	assert(width == 320);
	assert(height == 240);

	unsigned char h265_sps_320_240[41] = {
	    0x42, 0x01, 0x01, 0x01, 0x60, 0x00, 0x00, 0x03,
	    0x00, 0x90, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03,
	    0x00, 0x3f, 0xa0, 0x0a, 0x08, 0x0f, 0x16, 0x59,
	    0x59, 0xa4, 0x93, 0x2b, 0x9a, 0x02, 0x00, 0x00,
	    0x03, 0x00, 0x02, 0x00, 0x00, 0x03, 0x00, 0x78,
	    0x10};
	buf = DecodeAnnexBBuffer(h265_sps_320_240, sizeof(h265_sps_320_240));

	width  = 0;
	height = 0;
	ParseH265SPS(buf.data(), buf.size(), &width, &height);
	assert(width == 320);
	assert(height == 240);
}

int main(int argc, char** argv) {
	void* decoder = nullptr;

	// Fail to open a non-existent file
	string err = GetErr(MakeDecoder("foo.mp4", nullptr, &decoder));
	assert(err.find("No such file") != string::npos);

	TestSPSDecode();
	TestFile("../../testdata/tracking/0001-LD.mp4", 64);
	TestFile("out-h265.mp4", 0);
}
