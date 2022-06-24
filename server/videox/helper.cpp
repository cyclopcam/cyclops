extern "C" {
#include <libavcodec/avcodec.h>
#include <libavformat/avformat.h>
#include <libavformat/avio.h>
}

#include <stdint.h>
#include "helper.h"

struct Encoder {
	AVFormatContext* OutFormatCtx = nullptr;
	AVOutputFormat*  Format       = nullptr;
	AVCodec*         Codec        = nullptr;
	AVCodecContext*  CodecCtx     = nullptr;
	AVStream*        OutStream    = nullptr;
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

extern "C" {

// 2048 x 1536
void* MakeEncoder(char** err, const char* format, const char* filename, int width, int height) {
	//av_register_all();
	avcodec_register_all();

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
	encoder->OutStream->codecpar->bit_rate   = 4000000;
	encoder->OutStream->time_base            = AVRational{1, 30};
	encoder->CodecCtx->time_base             = AVRational{1, 30};

	if (avcodec_parameters_to_context(encoder->CodecCtx, encoder->OutStream->codecpar) < 0)
		RETURN_ERROR("avcodec_parameters_to_context failed");

	encoder->CodecCtx->profile = FF_PROFILE_H264_HIGH;
	if (encoder->OutFormatCtx->oformat->flags & AVFMT_GLOBALHEADER)
		encoder->CodecCtx->flags |= AV_CODEC_FLAG_GLOBAL_HEADER;

	if (avcodec_parameters_from_context(encoder->OutStream->codecpar, encoder->CodecCtx) < 0)
		RETURN_ERROR("avcodec_parameters_from_context failed");

	if (avcodec_open2(encoder->CodecCtx, encoder->Codec, nullptr) < 0)
		RETURN_ERROR("avcodec_open2 failed");

	if (avio_open2(&encoder->OutFormatCtx->pb, filename, AVIO_FLAG_WRITE, nullptr, nullptr) < 0)
		RETURN_ERROR("avio_open2 failed");

	if (avformat_write_header(encoder->OutFormatCtx, nullptr) < 0)
		RETURN_ERROR("Error avformat_write_header");

	av_dump_format(encoder->OutFormatCtx, 0, filename, 1);

	cleanup.E = nullptr; // allow Encoder to survive
	return encoder;
}
}