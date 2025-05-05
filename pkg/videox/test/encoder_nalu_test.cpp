#include <stdio.h>
#include <assert.h>
#include <stdint.h>
#include <malloc.h>
#include "encoder.h"
#include "decoder.h"
#include "tsf.hpp"

// This test writes NALUs to a file, which is an accurate simulation
// of what we do generally do in Cyclops, when there is no need to
// re-encode.
// This test reads from 'out.mp4', and rewrites it to 'out2.mp4'.
// You can generate out.mp4 using encoder_test. Or you could just use
// any other mp4 file.

// Debug build:
// clang++ -g -O0 -fsanitize=address -std=c++17 -I. -I/usr/local/include -L/usr/local/lib -lavformat -lavcodec -lavutil -lswscale -o test/encoder_nalu_test test/encoder_nalu_test.cpp encoder.cpp decoder.cpp common.cpp

void Check(char* e) {
	if (e == nullptr)
		return;
	printf("Error: %s\n", e);
	assert(false);
}

int main(int argc, char** argv) {
	void* decoder = nullptr;
	Check(MakeDecoder("out.mp4", "h264", &decoder));
	int width = 0, height = 0;
	Decoder_VideoSize(decoder, &width, &height);

	EncoderParams encoderParams;
	MakeEncoderParams("h264", width, height, AVPixelFormat::AV_PIX_FMT_YUV420P, AVPixelFormat::AV_PIX_FMT_YUV420P, EncoderType::EncoderTypePackets, 30, &encoderParams);
	void* encoder = nullptr;
	Check(MakeEncoder("mp4", "out2.mp4", &encoderParams, &encoder));

	int packetIdx = 0;
	while (true) {
		void*   packet     = nullptr;
		size_t  packetSize = 0;
		int64_t pts        = 0;
		int64_t dts        = 0;
		char*   err        = Decoder_NextPacket(decoder, &packet, &packetSize, &pts, &dts);
		if (err != nullptr) {
			if (strcmp(err, "EOF") == 0) {
				free(err);
				break;
			}
			Check(err);
		}
		//tsf::print("Packet %v, dts %v, pts %v\n", packetIdx, dts, pts);
		std::vector<NALU> nalus;
		FindNALUsAvcc(packet, packetSize, nalus);
		int64_t dtsNano = Decoder_PTSNano(decoder, dts);
		int64_t ptsNano = Decoder_PTSNano(decoder, pts);

		for (const auto& nalu : nalus) {
			Check(Encoder_WriteNALU(encoder, dtsNano, ptsNano, 0, nalu.Data, nalu.Size));
		}

		free(packet);
		packetIdx++;
	}
	Check(Encoder_WriteTrailer(encoder));

	Decoder_Close(decoder);
	Encoder_Close(encoder);
}
