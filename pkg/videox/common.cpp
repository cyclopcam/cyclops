#include "common.h"

extern "C" {

// I can't figure out how to get AV_ERROR_MAX_STRING_SIZE into Go code.. so we need this extra malloc
// Note that this means you must free() the result.
char* GetAvErrorStr(int averr) {
	char msg[AV_ERROR_MAX_STRING_SIZE] = {0};
	av_make_error_string(msg, AV_ERROR_MAX_STRING_SIZE, averr);
	return strdup(msg);
}
} // extern "C"

MyCodec GetMyCodec(AVCodecID codecId) {
	switch (codecId) {
	case AV_CODEC_ID_H264:
		return MyCodec::H264;
	case AV_CODEC_ID_HEVC:
		return MyCodec::H265;
	default:
		return MyCodec::None;
	}
}

void FindNALUsAnnexB(const void* packet, size_t packetSize, std::vector<NALU>& nalus) {
	const uint8_t* in = (const uint8_t*) packet;
	size_t         i  = 0;

	for (; i < packetSize - 2; i++) {
		if (in[i] == 0 && in[i + 1] == 0 && in[i + 2] == 1) {
			nalus.push_back(NALU{in + i + 3, 0});
		}
	}
	// add terminal
	nalus.push_back(NALU{in + packetSize, 0});

	for (size_t k = 0; k < nalus.size() - 1; k++) {
		nalus[k].Size = (uint8_t*) nalus[k + 1].Data - (uint8_t*) nalus[k].Data;
	}

	// remove terminal
	nalus.pop_back();
}

// Split into 4-byte length-prefixed NALUs.
// Returns false on error.
bool FindNALUsAvcc(const void* packet, size_t packetSize, std::vector<NALU>& nalus) {
	const uint8_t* in = (const uint8_t*) packet;
	size_t         i  = 0;

	while (i < packetSize - 4) {
		NALU n;
		n.Size = (in[i] << 24) | (in[i + 1] << 16) | (in[i + 2] << 8) | in[i + 3];
		n.Data = in + i + 4;
		if (i + 4 + n.Size > packetSize) {
			return false; // Not enough data
		}
		nalus.push_back(n);
		i += 4 + n.Size;
	}

	if (i != packetSize) {
		return false;
	}

	return true;
}