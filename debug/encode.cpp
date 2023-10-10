#include <string.h>
#include <stdio.h>
#include <assert.h>
#include <string>
#include <algorithm>
#include "pkg/videox/encoder.h"
#include "pkg/videox/tsf.hpp"
#include "debug/glob.hpp"

// build ffmpeg:
// sudo apt install libx264-dev
// ./configure --enable-libx264 --enable-gpl
// or... for good debugging
// ./configure --enable-libx264 --enable-gpl --disable-optimizations
// make -j
//
// use build_encode to compile this file
// You must first build ffmpeg from source

//func ParseBinFilename(filename string) (packetNumber int, naluNumber int, timeNS int64) {
//	// filename example:
//	// 026-002.002599955555.raw
//	major := strings.Split(filename, ".")
//	a, b, _ := strings.Cut(major[0], "-")
//	packetNumber, _ = strconv.Atoi(a)
//	naluNumber, _ = strconv.Atoi(b)
//	timeNS, _ = strconv.ParseInt(major[1], 10, 64)
//	return
//}

using namespace std;

struct Packet {
	vector<std::string> NALUs;
	int64_t             PTS;
};

int main(int argc, char** argv) {
	tsf::print("Hello!\n");

	auto files = glob::glob("/home/ben/dev/cyclops/raw/*.raw");
	sort(files.begin(), files.end());

	char* err     = nullptr;
	void* encoder = MakeEncoder(&err, "mp4", "dump/test.mp4", 2048, 1536);
	if (err != nullptr) {
		tsf::print("Failed: %v\n", err);
		free(err);
		return 1;
	}
	int            lastSeq = -1;
	Packet         packet;
	vector<Packet> packets;
	for (auto path : files) {
		// 026-002.002599955555.raw
		auto    filename = path.filename().string();
		size_t  idash    = filename.find('-');
		size_t  idot1    = filename.find('.');
		size_t  idot2    = filename.find('.', idot1);
		int     seq      = atoi(filename.substr(0, idash).c_str());
		int     seqx     = atoi(filename.substr(idash + 1, idot1 - idash - 1).c_str());
		int64_t pts      = atoll(filename.substr(idot1 + 1, idot2 - idot1 - 1).c_str());
		//tsf::print("%v %v %v %v\n", filename, seq, seqx, pts);
		if (lastSeq != seq) {
			if (lastSeq != -1) {
				packets.push_back(packet);
			}
			packet     = Packet();
			packet.PTS = pts;
		}
		FILE* f = fopen(path.c_str(), "rb");
		assert(f);
		char        buf[4096];
		std::string nalu;
		while (true) {
			size_t n = fread(buf, 1, sizeof(buf), f);
			if (n == -1) {
				tsf::print("Error reading file\n");
				break;
			} else if (n == 0) {
				break;
			}
			nalu.append((const char*) buf, n);
		}
		packet.NALUs.push_back(nalu);
		fclose(f);
		lastSeq = seq;
	}
	packets.push_back(packet);

	for (auto packet : packets) {
		int64_t dts = packet.PTS;
		int64_t pts = packet.PTS + 1000;
		for (auto nalu : packet.NALUs) {
			Encoder_WriteNALU(&err, encoder, dts, pts, 0, nalu.data(), nalu.length());
			dts += 10000;
			pts += 10000;
			if (err != nullptr) {
				tsf::print("WriteNALU error: %v\n", err);
				free(err);
			}
		}
	}

	Encoder_WriteTrailer(&err, encoder);
	if (err != nullptr) {
		tsf::print("Encoder_WriteTrailer error: %v\n", err);
		free(err);
	}

	Encoder_Close(encoder);

	return 0;
}
