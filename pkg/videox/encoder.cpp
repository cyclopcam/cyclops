//extern "C" {
//#include <libavcodec/avcodec.h>
//#include <libavformat/avformat.h>
//#include <libavformat/avio.h>
//}

#include <stdint.h>
#include "encoder.h"
#include "tsf.hpp"

// I don't yet know why, but this is the only way I can get ffmpeg to produce a valid
// mp4 file. The first packet we send it must be SPS + PPS + Keyframe.
// It is not sufficient to merely send SPS, then PPS, then Keyframe.
// I suspect this is something to do with the fact that MP4 stores this information not in the stream,
// but inside a once-off header in the file. However, I can't find an explicit ffmpeg
// API call to "write SPS + PPS". Perhaps this is just idomatic... or perhaps it's
// a hack that just works. But whatever the case, it's the first magic combination that
// I could find which just worked.

struct Encoder {
	AVFormatContext* OutFormatCtx = nullptr;
	AVOutputFormat*  Format       = nullptr;
	AVCodec*         Codec        = nullptr;
	AVCodecContext*  CodecCtx     = nullptr;
	AVStream*        OutStream    = nullptr;

	//bool SeenSPS = false;
	//bool SeenPPS = false;

	bool        SentHeader = false;
	std::string SPS;
	std::string PPS;
};

struct EncoderCleanup {
	Encoder* E;
	EncoderCleanup(Encoder* e) {
		E = e;
	}
	~EncoderCleanup() {
		if (!E)
			return;
		if (E->OutFormatCtx)
			avformat_free_context(E->OutFormatCtx);
		if (E->CodecCtx)
			avcodec_free_context(&E->CodecCtx);
		delete E;
	}
};

#define RETURN_ERROR(msg)   \
	{                       \
		*err = strdup(msg); \
		return nullptr;     \
	}

#define RETURN_STR(msg)             \
	{                               \
		*err = strdup(msg.c_str()); \
		return nullptr;             \
	}

std::string AvErr(int e) {
	char msg[AV_ERROR_MAX_STRING_SIZE] = {0};
	av_make_error_string(msg, AV_ERROR_MAX_STRING_SIZE, e);
	return msg;
}

std::string WithPrefix(const void* nalu, size_t size) {
	std::string s;
	s.resize(size + 3);
	s[0] = (char) 0;
	s[1] = (char) 0;
	s[2] = (char) 1;
	memcpy(&s[3], nalu, size);
	return s;
}

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

bool IsVisualPacket(H264NALUTypes t) {
	return (int) t >= 1 && (int) t <= 5;
}

H264NALUTypes GetH264NALUType(const uint8_t* buf) {
	return (H264NALUTypes) (buf[0] & 31);
}

void AppendNalu(std::string& buf, const void* nalu, size_t size) {
	buf += (char) 0;
	buf += (char) 0;
	buf += (char) 1;
	buf.append((const char*) nalu, size);
}

extern "C" {

// 2048 x 1536
void* MakeEncoder(char** err, const char* format, const char* filename, int width, int height) {
	//av_register_all();
	//avcodec_register_all();

	int  e     = 0;
	auto codec = AV_CODEC_ID_H264;

	auto           encoder = new Encoder();
	EncoderCleanup cleanup(encoder);

	encoder->Format = av_guess_format(format, nullptr, nullptr);
	if (encoder->Format == nullptr)
		RETURN_ERROR("Failed to find format");

	if (avformat_alloc_output_context2(&encoder->OutFormatCtx, encoder->Format, nullptr, nullptr) < 0)
		RETURN_ERROR("Failed to allocate output context");

	encoder->Codec = avcodec_find_encoder(codec);
	if (encoder->Codec == nullptr)
		RETURN_ERROR("Failed to find codec");

	encoder->CodecCtx = avcodec_alloc_context3(encoder->Codec);
	if (encoder->CodecCtx == nullptr)
		RETURN_ERROR("Failed to allocate codec context");

	encoder->OutStream = avformat_new_stream(encoder->OutFormatCtx, encoder->Codec);
	if (encoder->OutStream == nullptr)
		RETURN_ERROR("Failed to allocate output format stream");

	encoder->OutStream->codecpar->codec_id   = codec;
	encoder->OutStream->codecpar->codec_type = AVMEDIA_TYPE_VIDEO;
	encoder->OutStream->codecpar->width      = width;
	encoder->OutStream->codecpar->height     = height;
	encoder->OutStream->codecpar->format     = AV_PIX_FMT_YUV420P;
	encoder->OutStream->codecpar->bit_rate   = 400000;
	encoder->OutStream->time_base            = AVRational{1, 1000000};
	encoder->CodecCtx->time_base             = AVRational{1, 1000000};

	if (avcodec_parameters_to_context(encoder->CodecCtx, encoder->OutStream->codecpar) < 0)
		RETURN_ERROR("avcodec_parameters_to_context failed");

	encoder->CodecCtx->profile = FF_PROFILE_H264_HIGH;
	if (encoder->OutFormatCtx->oformat->flags & AVFMT_GLOBALHEADER)
		encoder->CodecCtx->flags |= AV_CODEC_FLAG_GLOBAL_HEADER;

	if (avcodec_parameters_from_context(encoder->OutStream->codecpar, encoder->CodecCtx) < 0)
		RETURN_ERROR("avcodec_parameters_from_context failed");

	if (avcodec_open2(encoder->CodecCtx, encoder->Codec, nullptr) < 0)
		RETURN_ERROR("avcodec_open2 failed");

	if (!!(encoder->CodecCtx->flags & AVFMT_NOFILE))
		RETURN_ERROR("codec does not write to a file");

	e = avio_open2(&encoder->OutFormatCtx->pb, filename, AVIO_FLAG_WRITE, nullptr, nullptr);
	if (e < 0)
		RETURN_STR(tsf::fmt("avio_open2(%v) failed: %v", filename, AvErr(e)));

	e = avformat_write_header(encoder->OutFormatCtx, nullptr);
	if (e < 0)
		RETURN_STR(tsf::fmt("avformat_write_header failed: %v", AvErr(e)));

	av_dump_format(encoder->OutFormatCtx, 0, filename, 1);

	cleanup.E = nullptr; // allow Encoder to survive
	return encoder;
}

void Encoder_Close(void* _encoder) {
	// when EncoderCleanup goes out of scope, it will clean up
	EncoderCleanup cleanup((Encoder*) _encoder);
}

// Iff naluPrefixLen == 0, then we prepend 00 00 01 to the nalu
void Encoder_WriteNALU(char** err, void* _encoder, int64_t dts, int64_t pts, int naluPrefixLen, const void* _nalu, size_t naluLen) {
	auto encoder    = (Encoder*) _encoder;
	auto nalu       = (const uint8_t*) _nalu;
	auto payload    = nalu + naluPrefixLen;
	auto packetType = GetH264NALUType(payload);
	//if (packetType == H264NALUTypes::SPS && encoder->SeenSPS)
	//	return;
	//if (packetType == H264NALUTypes::PPS && encoder->SeenPPS)
	//	return;
	if (naluPrefixLen != 0 && naluPrefixLen != 3 && naluPrefixLen != 4) {
		*err = strdup(tsf::fmt("Invalid naluPrefixLen %v. May only be one of: [0, 3, 4]", naluPrefixLen).c_str());
		return;
	}

	if (packetType == H264NALUTypes::SPS) {
		if (naluPrefixLen)
			encoder->SPS.assign((const char*) _nalu, naluLen);
		else
			encoder->SPS = WithPrefix(_nalu, naluLen);
		return;
	}
	if (packetType == H264NALUTypes::PPS) {
		if (naluPrefixLen)
			encoder->PPS.assign((const char*) _nalu, naluLen);
		else
			encoder->PPS = WithPrefix(_nalu, naluLen);
		return;
	}
	if ((encoder->SPS.size() == 0 || encoder->PPS.size() == 0) && IsVisualPacket(packetType)) {
		// The codec/format needs SPS and PPS before any frames, so we can't write frames yet
		return;
	}

	AVRational timeBase = encoder->OutStream->time_base;
	AVPacket*  pkt      = av_packet_alloc();
	pkt->dts            = av_rescale_q(dts, AVRational{1, 1000000000}, timeBase);
	pkt->pts            = av_rescale_q(pts, AVRational{1, 1000000000}, timeBase);
	//tsf::print("dts: %v, pts: %v\n", pkt->dts, pkt->pts);
	//pkt->data         = nalu;
	//pkt->size         = (int) naluLen + 3;
	pkt->stream_index = encoder->OutStream->id;
	if (packetType == H264NALUTypes::IDR)
		pkt->flags |= AV_PKT_FLAG_KEY;

	// copy is our temporary buffer, should we need it
	std::string copy;

	if (packetType == H264NALUTypes::IDR && !encoder->SentHeader) {
		const auto& sps = encoder->SPS;
		const auto& pps = encoder->PPS;

		// I don't yet know why, but this is the only way I can get ffmpeg to produce a valid
		// mp4 file. The first packet we send it must be SPS + PPS + Keyframe.
		// It is not sufficient to merely send SPS, then PPS, then Keyframe.
		// I suspect this is something to do with the fact that MP4 stores this information not in the stream,
		// but inside a once-off header in the file. However, I can't find an explicit ffmpeg
		// API call to "write SPS + PPS". Perhaps this is just idomatic... or perhaps it's
		// a hack that just works. But whatever the case, it's the first magic combination that
		// I could find which just worked.

		//uint8_t* side = av_packet_new_side_data(pkt, AV_PKT_DATA_NEW_EXTRADATA, sps.size() + pps.size());
		//memcpy(side, sps.data(), sps.size());
		//memcpy(side + sps.size(), pps.data(), pps.size());
		int extraBytes = 0;
		if (naluPrefixLen == 0)
			extraBytes = 3;
		copy.resize(sps.size() + encoder->PPS.size() + extraBytes + naluLen);
		size_t offset = 0;

		memcpy(&copy[offset], sps.data(), sps.size());
		offset += sps.size();

		memcpy(&copy[offset], pps.data(), pps.size());
		offset += pps.size();

		if (naluPrefixLen == 0) {
			memcpy(&copy[offset], "\x00\x00\x01", 3);
			offset += 3;
		}
		memcpy(&copy[offset], _nalu, naluLen);

		pkt->data           = (uint8_t*) copy.data();
		pkt->size           = (int) copy.size();
		encoder->SentHeader = true;
	} else {
		if (naluPrefixLen == 0) {
			copy.resize(3 + naluLen);
			memcpy(&copy[0], "\x00\x00\x01", 3);
			memcpy(&copy[3], _nalu, naluLen);
			pkt->data = (uint8_t*) copy.data();
			pkt->size = (int) copy.size();
			//tsf::print("slow path\n");
		} else {
			// We want this to be our most common code path, where we don't need any memcpy
			pkt->data = (uint8_t*) _nalu;
			pkt->size = (int) naluLen;
			//tsf::print("fast path\n");
		}
	}

	//int e = av_write_frame(encoder->OutFormatCtx, pkt);
	int e = av_interleaved_write_frame(encoder->OutFormatCtx, pkt);
	av_packet_free(&pkt);
	if (e < 0) {
		uint8_t bytes[4];
		int     i = 0;
		for (i = 0; i < sizeof(bytes) - 1 && nalu[i]; i++)
			bytes[i] = nalu[i];
		bytes[i] = 0;
		*err     = strdup(tsf::fmt("Failed to write packet (%02x %02x %02x %02x ...) len: %v, error: %v", bytes[0], bytes[1], bytes[2], bytes[3], naluLen, AvErr(e)).c_str());
	}
	//free(buf);
	//if (packetType == H264NALUTypes::SPS)
	//	encoder->SeenSPS = true;
	//if (packetType == H264NALUTypes::PPS)
	//	encoder->SeenPPS = true;
}

void Encoder_WritePacket(char** err, void* _encoder, int64_t dts, int64_t pts, int isKeyFrame, const void* packetData, size_t packetLen) {
	auto encoder = (Encoder*) _encoder;

	AVRational timeBase = encoder->OutStream->time_base;
	AVPacket*  pkt      = av_packet_alloc();
	pkt->dts            = av_rescale_q(dts, AVRational{1, 1000000000}, timeBase);
	pkt->pts            = av_rescale_q(pts, AVRational{1, 1000000000}, timeBase);
	pkt->stream_index   = encoder->OutStream->id;
	if (!!isKeyFrame)
		pkt->flags |= AV_PKT_FLAG_KEY;

	pkt->data = (uint8_t*) packetData;
	pkt->size = (int) packetLen;

	//int e = av_write_frame(encoder->OutFormatCtx, pkt);
	int e = av_interleaved_write_frame(encoder->OutFormatCtx, pkt);
	av_packet_free(&pkt);
	if (e < 0) {
		const uint8_t* data     = (const uint8_t*) packetData;
		uint8_t        bytes[8] = {0};
		for (int i = 0; i < sizeof(bytes) && i < packetLen; i++)
			bytes[i] = data[i];
		*err = strdup(
		    tsf::fmt("Failed to write packet (%02x %02x %02x %02x %02x %02x %02x %02x ...) len: %v, error: %v",
		             bytes[0], bytes[1], bytes[2], bytes[3], bytes[4], bytes[5], bytes[6], bytes[7],
		             (int) packetLen, AvErr(e))
		        .c_str());
	}
}

void Encoder_WriteTrailer(char** err, void* _encoder) {
	auto encoder = (Encoder*) _encoder;
	int  e       = av_write_trailer(encoder->OutFormatCtx);
	if (e < 0) {
		*err = strdup(tsf::fmt("av_write_trailer failed: %v", AvErr(e)).c_str());
	}
}

void SetPacketDataPointer(void* _pkt, const void* buf, size_t bufLen) {
	tsf::print("SetPacketDataPointer %v %v %v\n", _pkt, buf, bufLen);
	AVPacket* pkt = (AVPacket*) _pkt;
	pkt->data     = (uint8_t*) buf;
	pkt->size     = (int) bufLen;
}

// I can't figure out how to get AV_ERROR_MAX_STRING_SIZE into Go code.. so we need this extra malloc
// Note that this means you must free() the result.
char* GetAvErrorStr(int averr) {
	char msg[AV_ERROR_MAX_STRING_SIZE] = {0};
	av_make_error_string(msg, AV_ERROR_MAX_STRING_SIZE, averr);
	return strdup(msg);
}

int AvCodecSendPacket(AVCodecContext* ctx, const void* buf, size_t bufLen) {
	AVPacket* pkt = av_packet_alloc();
	pkt->data     = (uint8_t*) buf;
	pkt->size     = (int) bufLen;
	int res       = avcodec_send_packet(ctx, pkt);
	av_packet_free(&pkt);
	return res;
}
}