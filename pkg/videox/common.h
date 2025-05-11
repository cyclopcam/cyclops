#ifndef _VIDEOX_COMMON_H
#define _VIDEOX_COMMON_H

#ifdef __cplusplus
extern "C" {
#endif

#include <libavcodec/avcodec.h>
#include <libavformat/avformat.h>
#include <libavformat/avio.h>
#include <libavutil/imgutils.h>
#include <libavutil/pixfmt.h>
#include <libavutil/opt.h>
#include <libswscale/swscale.h>

typedef struct NALU {
	const void* Data;
	size_t      Size;
} NALU;

char* GetAvErrorStr(int averr);

#ifdef __cplusplus
}
#endif

// C++ internal functions (not exposed to Go)
#ifdef __cplusplus
#include <vector>

enum class MyCodec {
	None,
	H264,
	H265,
};
MyCodec GetMyCodec(AVCodecID codecId);

enum class H264NALUTypes {
	// From nalutype.go in gortsplib
	Unknown                       = 0,
	NonIDR                        = 1,
	DataPartitionA                = 2,
	DataPartitionB                = 3,
	DataPartitionC                = 4,
	IDR                           = 5,
	SEI                           = 6,
	SPS                           = 7,
	PPS                           = 8,
	AccessUnitDelimiter           = 9,
	EndOfSequence                 = 10,
	EndOfStream                   = 11,
	FillerData                    = 12,
	SPSExtension                  = 13,
	Prefix                        = 14,
	SubsetSPS                     = 15,
	Reserved16                    = 16,
	Reserved17                    = 17,
	Reserved18                    = 18,
	SliceLayerWithoutPartitioning = 19,
	SliceExtension                = 20,
	SliceExtensionDepth           = 21,
	Reserved22                    = 22,
	Reserved23                    = 23,
};

// From github.com/bluenviron/mediacommon/blob/main/pkg/codecs/h265/nalu_type.go
enum class H265NALUTypes {
	TRAIL_N        = 0,
	TRAIL_R        = 1,
	TSA_N          = 2,
	TSA_R          = 3,
	STSA_N         = 4,
	STSA_R         = 5,
	RADL_N         = 6,
	RADL_R         = 7,
	RASL_N         = 8,
	RASL_R         = 9,
	RSV_VCL_N10    = 10,
	RSV_VCL_N12    = 12,
	RSV_VCL_N14    = 14,
	RSV_VCL_R11    = 11,
	RSV_VCL_R13    = 13,
	RSV_VCL_R15    = 15,
	BLA_W_LP       = 16,
	BLA_W_RADL     = 17,
	BLA_N_LP       = 18,
	IDR_W_RADL     = 19,
	IDR_N_LP       = 20,
	CRA_NUT        = 21,
	RSV_IRAP_VCL22 = 22,
	RSV_IRAP_VCL23 = 23,
	VPS_NUT        = 32,
	SPS_NUT        = 33,
	PPS_NUT        = 34,
	AUD_NUT        = 35,
	EOS_NUT        = 36,
	EOB_NUT        = 37,
	FD_NUT         = 38,
	PREFIX_SEI_NUT = 39,
	SUFFIX_SEI_NUT = 40,

	// additional NALU types for RTP/H265
	AggregationUnit   = 48,
	FragmentationUnit = 49,
	PACI              = 50,
};

inline H264NALUTypes GetH264NALUType(const uint8_t* buf) {
	return (H264NALUTypes) (buf[0] & 31);
}

inline H265NALUTypes GetH265NALUType(const uint8_t* buf) {
	return (H265NALUTypes) ((buf[0] >> 1) & 63);
}

inline bool IsVisualPacket(H264NALUTypes t) {
	return (int) t >= 1 && (int) t <= 5;
}

inline bool IsVisualPacket(H265NALUTypes t) {
	return (int) t >= 0 && (int) t <= 31;
}

inline bool IsIDR(H264NALUTypes t) {
	return t == H264NALUTypes::IDR;
}

inline bool IsIDR(H265NALUTypes t) {
	return t == H265NALUTypes::IDR_N_LP || t == H265NALUTypes::IDR_W_RADL;
}

inline bool IsEssentialMeta(H264NALUTypes t) {
	return t == H264NALUTypes::SPS || t == H264NALUTypes::PPS || t == H264NALUTypes::SEI;
}

inline bool IsEssentialMeta(H265NALUTypes t) {
	return t == H265NALUTypes::VPS_NUT || t == H265NALUTypes::SPS_NUT || t == H265NALUTypes::PPS_NUT || t == H265NALUTypes::PREFIX_SEI_NUT;
}

inline bool IsEssentialMeta(MyCodec codec, const uint8_t* buf) {
	switch (codec) {
	case MyCodec::None: return false;
	case MyCodec::H264: return IsEssentialMeta(GetH264NALUType(buf));
	case MyCodec::H265: return IsEssentialMeta(GetH265NALUType(buf));
	}
	return false;
}

inline bool IsIDR(MyCodec codec, const uint8_t* buf) {
	switch (codec) {
	case MyCodec::None: return false;
	case MyCodec::H264: return IsIDR(GetH264NALUType(buf));
	case MyCodec::H265: return IsIDR(GetH265NALUType(buf));
	}
	return false;
}

inline bool IsVisualPacket(MyCodec codec, const uint8_t* buf) {
	switch (codec) {
	case MyCodec::None: return false;
	case MyCodec::H264: return IsVisualPacket(GetH264NALUType(buf));
	case MyCodec::H265: return IsVisualPacket(GetH265NALUType(buf));
	}
	return false;
}

void FindNALUsAnnexB(const void* packet, size_t packetSize, std::vector<NALU>& nalus);
bool FindNALUsAvcc(const void* packet, size_t packetSize, std::vector<NALU>& nalus);
void DumpNALUHeader(MyCodec codec, const NALU& nalu);
#endif

#endif // _VIDEOX_COMMON_H