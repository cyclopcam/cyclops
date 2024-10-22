#include "decoder2.h"
#include "tsf.hpp"

struct Decoder {
	AVFormatContext* FormatCtx;   // Only populated for files
	int              VideoStream; // Only populated for files
	AVCodecContext*  CodecCtx;
	AVFrame*         SrcFrame;
	SwsContext*      SwsCtx;
	AVFrame*         DstFrame;
	uint8_t*         DstFramePtr;
	AVPacket*        DecodePacket; // Used during decode
};

struct DecoderCleanup {
	Decoder* D;
	DecoderCleanup(Decoder* d) {
		D = d;
	}
	~DecoderCleanup() {
		if (!D)
			return;
		if (D->DecodePacket)
			av_packet_free(&D->DecodePacket);
		if (D->SrcFrame)
			av_frame_free(&D->SrcFrame);
		if (D->CodecCtx)
			avcodec_free_context(&D->CodecCtx);
		if (D->FormatCtx)
			avformat_close_input(&D->FormatCtx);
		//avformat_free_context(D->FormatCtx);
		free(D);
	}
};

// Get the string error message for the given error code
inline std::string AvErr(int e) {
	char msg[AV_ERROR_MAX_STRING_SIZE] = {0};
	av_make_error_string(msg, AV_ERROR_MAX_STRING_SIZE, e);
	return msg;
}

#define DUPSTR(s) strdup(s.c_str())
#define RETURN_ERROR_STR(msg) return strdup(msg.c_str())
#define RETURN_ERROR_STATIC(msg) return strdup(msg)
#define ERROR_EOF "EOF"

//inline char* ShallowCopyFrameToYUVImage(AVFrame* srcFrame, YUVImage* output) {
//	// The strides are typically quite a bit more than the width (or width/2 for UV).
//	//if (srcFrame->linesize[0] < srcFrame->width)
//	//	return DUPSTR(tsf::fmt("Only 4:2:0 images supported (plane 0): %v < %v", srcFrame->linesize[0], srcFrame->width));
//	//if (srcFrame->linesize[1] < srcFrame->width / 2)
//	//	return DUPSTR(tsf::fmt("Only 4:2:0 images supported (plane 1): %v < %v", srcFrame->linesize[1], srcFrame->width / 2));
//	//if (srcFrame->linesize[2] < srcFrame->width / 2)
//	//	return DUPSTR(tsf::fmt("Only 4:2:0 images supported (plane 2): %v < %v", srcFrame->linesize[2], srcFrame->width / 2));
//
//	// Limit it here, because our 'accel' package assume 420P
//	if (srcFrame->format != AV_PIX_FMT_YUV420P)
//		return DUPSTR(tsf::fmt("Only YUV420P images supported: %v", srcFrame->format));
//
//	output->Chroma  = ChromaSampling_420;
//	output->Width   = srcFrame->width;
//	output->Height  = srcFrame->height;
//	output->YStride = srcFrame->linesize[0];
//	output->UStride = srcFrame->linesize[1];
//	output->VStride = srcFrame->linesize[2];
//	output->Y       = srcFrame->data[0];
//	output->U       = srcFrame->data[1];
//	output->V       = srcFrame->data[2];
//	return nullptr;
//}

extern "C" {

char* MakeDecoder(const char* filename, const char* codecName, void** output_decoder) {
	Decoder* d = (Decoder*) malloc(sizeof(Decoder));
	memset(d, 0, sizeof(Decoder));
	DecoderCleanup cleanup(d);
	int            e     = 0;
	const AVCodec* codec = nullptr;

	if (filename != nullptr) {
		e = avformat_open_input(&d->FormatCtx, filename, nullptr, nullptr);
		if (e < 0)
			return DUPSTR(tsf::fmt("avformat_open_input(%v) failed: %v", filename, AvErr(e)));

		e = avformat_find_stream_info(d->FormatCtx, nullptr);
		if (e < 0)
			return DUPSTR(tsf::fmt("avformat_find_stream_info(%v) failed: %v", filename, AvErr(e)));

		d->VideoStream = av_find_best_stream(d->FormatCtx, AVMEDIA_TYPE_VIDEO, -1, -1, &codec, 0);
		if (d->VideoStream < 0)
			return DUPSTR(tsf::fmt("av_find_best_stream(%v) failed: %v", filename, AvErr(d->VideoStream)));

		d->CodecCtx = avcodec_alloc_context3(codec);
		if (d->CodecCtx == nullptr)
			return DUPSTR(tsf::fmt("avcodec_alloc_context3(%v) failed", filename));

		e = avcodec_parameters_to_context(d->CodecCtx, d->FormatCtx->streams[d->VideoStream]->codecpar);
		if (e < 0)
			return DUPSTR(tsf::fmt("avcodec_parameters_to_context(%v) failed: %v", filename, AvErr(e)));
	} else if (codecName != nullptr) {
		AVCodecID codecID = AV_CODEC_ID_NONE;
		if (strcmp(codecName, "h264") == 0)
			codecID = AV_CODEC_ID_H264;
		else if (strcmp(codecName, "h265") == 0)
			codecID = AV_CODEC_ID_H265;
		else
			return strdup("Unknown codec. Only valid values are 'h264' and 'h265'");

		codec = avcodec_find_decoder(codecID);
		if (codec == nullptr)
			return DUPSTR(tsf::fmt("avcodec_find_decoder(%v) failed", codecName));

		d->CodecCtx = avcodec_alloc_context3(codec);
		if (d->CodecCtx == nullptr)
			return DUPSTR(tsf::fmt("avcodec_alloc_context3(%v) failed", filename));
	} else {
		return strdup("Must specify either filename or codecName");
	}

	e = avcodec_open2(d->CodecCtx, codec, nullptr);
	if (e < 0)
		return DUPSTR(tsf::fmt("avcodec_open2(%v) failed: %v", filename, AvErr(e)));

	d->SrcFrame = av_frame_alloc();
	if (d->SrcFrame == nullptr)
		return DUPSTR(tsf::fmt("av_frame_alloc(%v) failed", filename));

	d->DecodePacket = av_packet_alloc();
	if (d->DecodePacket == nullptr)
		return DUPSTR(tsf::fmt("av_packet_alloc(%v) failed", filename));

	cleanup.D       = nullptr; // Allow Decoder to live
	*output_decoder = d;
	return nullptr;
}

void Decoder_Close(void* decoder) {
	DecoderCleanup cleanup((Decoder*) decoder);
}

void Decoder_VideoSize(void* decoder, int* width, int* height) {
	Decoder* d = (Decoder*) decoder;
	*width     = d->CodecCtx->width;
	*height    = d->CodecCtx->height;
}

// Decode the next frame in the video file
char* Decoder_NextFrame(void* decoder, AVFrame** output) {
	int       e      = 0;
	Decoder*  d      = (Decoder*) decoder;
	AVPacket* packet = d->DecodePacket;

	while (true) {
		e = av_read_frame(d->FormatCtx, packet);
		if (e == AVERROR_EOF)
			return strdup(ERROR_EOF);
		else if (e < 0)
			return DUPSTR(tsf::fmt("av_read_frame() failed: %v", AvErr(e)));

		int sendPacketErr = 0;
		if (packet->stream_index == d->VideoStream) {
			sendPacketErr = avcodec_send_packet(d->CodecCtx, packet);
		}

		// After av_read_frame, we need to unref the packet
		av_packet_unref(packet);

		if (sendPacketErr < 0)
			return DUPSTR(tsf::fmt("avcodec_send_packet() failed: %v", AvErr(sendPacketErr)));

		e = avcodec_receive_frame(d->CodecCtx, d->SrcFrame);
		if (e == AVERROR_EOF)
			return strdup(ERROR_EOF);
		else if (e == AVERROR(EAGAIN))
			continue;
		else if (e < 0)
			return DUPSTR(tsf::fmt("avcodec_receive_frame() failed: %v", AvErr(e)));

		//return ShallowCopyFrameToYUVImage(d->SrcFrame, output);
		*output = d->SrcFrame;
		return nullptr;
	}
}

// Read the next packet out of the file, and return a copy of it.
// This is a low level function built for testing the decoder in streaming mode.
// We use this function to read packets out of an MP4 file, and then feed them
// into another explicit 'h264' decoder.
// This function is inherently wasteful because it clones the contents of the packet.
// That's not something you'd normally want to do.
// The caller must free() the packet when done.
char* Decoder_NextPacket(void* decoder, void** packet, size_t* packetSize) {
	int       e = 0;
	Decoder*  d = (Decoder*) decoder;
	AVPacket* p = d->DecodePacket;

	while (true) {
		e = av_read_frame(d->FormatCtx, p);
		if (e == AVERROR_EOF)
			return strdup(ERROR_EOF);
		else if (e < 0)
			return DUPSTR(tsf::fmt("av_read_frame() failed: %v", AvErr(e)));

		bool  isMyStream = p->stream_index == d->VideoStream;
		char* err        = nullptr;
		if (isMyStream) {
			*packet = malloc(p->size);
			if (*packet == nullptr)
				err = DUPSTR(tsf::fmt("malloc(%v) for packet failed", p->size));
			memcpy(*packet, p->data, p->size);
			*packetSize = p->size;
		}
		av_packet_unref(p);
		if (err != nullptr)
			return err;
		if (!isMyStream)
			continue;

		return nullptr;
	}
}

// Decode a packet from a video stream
char* Decoder_DecodePacket(void* decoder, const void* packet, size_t packetSize, AVFrame** output) {
	int       e = 0;
	Decoder*  d = (Decoder*) decoder;
	AVPacket* p = d->DecodePacket;

	p->data = (uint8_t*) packet;
	p->size = packetSize;

	e = avcodec_send_packet(d->CodecCtx, p);
	if (e < 0)
		return DUPSTR(tsf::fmt("avcodec_send_packet() failed: %v", AvErr(e)));

	e = avcodec_receive_frame(d->CodecCtx, d->SrcFrame);
	if (e == AVERROR_EOF)
		return strdup(ERROR_EOF);
	else if (e < 0)
		return DUPSTR(tsf::fmt("avcodec_receive_frame() failed: %v", AvErr(e)));

	//return ShallowCopyFrameToYUVImage(d->SrcFrame, output);
	*output = d->SrcFrame;
	return nullptr;
}

} // extern "C"