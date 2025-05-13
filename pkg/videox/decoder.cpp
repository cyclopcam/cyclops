#include "decoder.h"
#include "tsf.hpp"

struct Decoder {
	AVFormatContext*    FormatCtx    = nullptr; // Only populated for files
	int                 VideoStream  = -1;      // Only populated for files
	AVCodecContext*     CodecCtx     = nullptr;
	AVFrame*            FrameA       = nullptr; // Frame that codec emits. Can be in hardware space (eg DRM_PRIME)
	AVFrame*            FrameB       = nullptr; // Frame we've copied to CPU.
	AVFrame*            FrameC       = nullptr; // Frame we've decoded from some other format, into AV_PIX_FMT_YUV420P
	SwsContext*         SwsCtx       = nullptr;
	AVPacket*           DecodePacket = nullptr;
	AVBufferRef*        HwDeviceCtx  = nullptr; // Hardware device context
	enum AVHWDeviceType HwType       = AV_HWDEVICE_TYPE_NONE;
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
		if (D->FrameA)
			av_frame_free(&D->FrameA);
		if (D->FrameB)
			av_frame_free(&D->FrameB);
		if (D->FrameC)
			av_frame_free(&D->FrameC);
		if (D->SwsCtx)
			sws_freeContext(D->SwsCtx);
		if (D->CodecCtx)
			avcodec_free_context(&D->CodecCtx);
		if (D->FormatCtx)
			avformat_close_input(&D->FormatCtx);
		if (D->HwDeviceCtx)
			av_buffer_unref(&D->HwDeviceCtx);
		delete D;
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

// Find hardware decoder
//static const AVCodec* find_hw_decoder(AVCodecID codec_id, enum AVHWDeviceType hw_type) {
//	const AVCodec* codec = nullptr;
//	void*          iter  = nullptr;
//	while ((codec = av_codec_iterate(&iter))) {
//		if (codec->id == codec_id && av_codec_is_decoder(codec)) {
//			if (codec->capabilities & AV_CODEC_CAP_HARDWARE) {
//				return codec;
//			}
//		}
//	}
//	return nullptr;
//}

static enum AVPixelFormat get_format_drm_prime(AVCodecContext*           ctx,
                                               const enum AVPixelFormat* pix) {
	//printf("Hello from get_format_drm_prime\n");
	for (const enum AVPixelFormat* p = pix; *p != AV_PIX_FMT_NONE; p++)
		// what the hwaccel returns (hevc on pi5)
		if (*p == AV_PIX_FMT_DRM_PRIME)
			return *p;
	// fallback to software
	return pix[0];
}

// Copy from hardware to CPU if needed
char* Decoder_ExtractFrame(Decoder* d, AVFrame** output) {
	int e = 0;

	// Transfer from hardware to CPU if needed
	if (d->HwDeviceCtx && d->FrameA->format != AV_PIX_FMT_YUV420P) {
		if (d->FrameA->format == AV_PIX_FMT_DRM_PRIME) {
			// Copy from hardware to CPU
			e = av_hwframe_transfer_data(d->FrameB, d->FrameA, 0);
			if (e < 0)
				return DUPSTR(tsf::fmt("av_hwframe_transfer_data() failed: %v", AvErr(e)));
			if (d->FrameB->format == AV_PIX_FMT_YUV420P) {
				*output = d->FrameB;
				return nullptr;
			}
		} else {
			return strdup("Unsupported pixel format");
		}

		// Unstressed code path!
		if (d->SwsCtx == nullptr) {
			d->SwsCtx = sws_getContext(
			    d->FrameB->width, d->FrameB->height, (enum AVPixelFormat) d->FrameB->format,
			    d->FrameB->width, d->FrameB->height, AV_PIX_FMT_YUV420P,
			    SWS_BILINEAR, nullptr, nullptr, nullptr);

			if (!d->SwsCtx)
				return strdup("sws_getContext() failed");
		}

		if (d->FrameC == nullptr) {
			d->FrameC = av_frame_alloc();
			if (d->FrameC == nullptr)
				return strdup("av_frame_alloc() failed");
			d->FrameC->format = AV_PIX_FMT_YUV420P;
			d->FrameC->width  = d->FrameB->width;
			d->FrameC->height = d->FrameB->height;
			e                 = av_frame_get_buffer(d->FrameC, 0);
			if (e < 0)
				return DUPSTR(tsf::fmt("av_frame_get_buffer() failed: %v", AvErr(e)));
		}

		// Why was this block generated by the AI?
		//e = av_frame_copy(d->FrameC, d->FrameB);
		//if (e < 0)
		//	return DUPSTR(tsf::fmt("av_frame_copy() failed: %v", AvErr(e)));

		e = sws_scale(d->SwsCtx,
		              d->FrameB->data, d->FrameB->linesize, 0, d->FrameB->height,
		              d->FrameC->data, d->FrameC->linesize);

		if (e < 0)
			return DUPSTR(tsf::fmt("sws_scale() failed: %v", AvErr(e)));

		*output = d->FrameC;
	} else {
		*output = d->FrameA;
	}
	return nullptr;
}

extern "C" {

char* MakeDecoder(const char* filename, const char* codecName, void** output_decoder) {
	Decoder*       d = new Decoder();
	DecoderCleanup cleanup(d);
	int            e = 0;
#if LIBAVCODEC_VERSION_MAJOR < 59
	AVCodec* codec = nullptr;
#else
	const AVCodec* codec = nullptr;
#endif

	//av_log_set_level(AV_LOG_DEBUG);

	AVCodecID codecID = AV_CODEC_ID_NONE;

	if (filename != nullptr) {
		e = avformat_open_input(&d->FormatCtx, filename, nullptr, nullptr);
		if (e < 0)
			return DUPSTR(tsf::fmt("avformat_open_input(%v) failed: %v", filename, AvErr(e)));

		e = avformat_find_stream_info(d->FormatCtx, nullptr);
		if (e < 0)
			return DUPSTR(tsf::fmt("avformat_find_stream_info(%v) failed: %v", filename, AvErr(e)));

		//d->VideoStream = av_find_best_stream(d->FormatCtx, AVMEDIA_TYPE_VIDEO, -1, -1, &codec, 0);
		d->VideoStream = av_find_best_stream(d->FormatCtx, AVMEDIA_TYPE_VIDEO, -1, -1, nullptr, 0);
		if (d->VideoStream < 0)
			return DUPSTR(tsf::fmt("av_find_best_stream(%v) failed: %v", filename, AvErr(d->VideoStream)));

		codecID = d->FormatCtx->streams[d->VideoStream]->codecpar->codec_id;
	} else if (codecName != nullptr) {
		if (strcmp(codecName, "h264") == 0) {
			codecID = AV_CODEC_ID_H264;
		} else if (strcmp(codecName, "h265") == 0 || strcmp(codecName, "hevc") == 0) {
			//d->NeedInit = true;
			codecID = AV_CODEC_ID_HEVC;
		} else {
			return strdup("Unknown codec. Only valid values are 'h264', 'h265', and 'hevc'");
		}
	} else {
		return strdup("Must specify either filename or codecName");
	}

	// The only hardware we're currently targeting for hwaccel is Rpi5,
	// and that only supports hevc.
	bool enableHwAccel = codecID == AV_CODEC_ID_HEVC;

	if (enableHwAccel) {
		const char* hw_type_name = "drm";

		//enum AVHWDeviceType iter = AV_HWDEVICE_TYPE_NONE;
		//while (1) {
		//	iter = av_hwdevice_iterate_types(iter);
		//	if (iter == AV_HWDEVICE_TYPE_NONE)
		//		break;
		//	printf("hw: %s\n", av_hwdevice_get_type_name(iter));
		//}

		// Initialize hardware device context
		d->HwType = av_hwdevice_find_type_by_name(hw_type_name);
		if (d->HwType != AV_HWDEVICE_TYPE_NONE) {
			e = av_hwdevice_ctx_create(&d->HwDeviceCtx, d->HwType, nullptr, nullptr, 0);
			if (e < 0) {
				//printf("Failed to create hardware device context: %d %s\n", e, AvErr(e).c_str());
				d->HwType      = AV_HWDEVICE_TYPE_NONE;
				d->HwDeviceCtx = nullptr;
			}
		}
	}

	// Try to find hardware decoder first
	/*
	if (d->HwType != AV_HWDEVICE_TYPE_NONE) {
		codec = find_hw_decoder(d->FormatCtx->streams[d->VideoStream]->codecpar->codec_id, d->HwType);
		if (!codec) {
			printf("Falling back to software decoder\n");
			av_buffer_unref(&d->HwDeviceCtx);
			d->HwDeviceCtx = nullptr;
			d->HwType      = AV_HWDEVICE_TYPE_NONE;
		}
	}
	if (!codec) {
		// fall back to software decoder
		if (codecID == AV_CODEC_ID_HEVC) {
			if (!codec) {
				codec = avcodec_find_decoder_by_name("hevc_v4l2request");
				if (!codec)
					printf("Failed to find hevc_v4l2request\n");
			}
			if (!codec) {
				codec = avcodec_find_decoder_by_name("hevc_v4l2m2m");
				if (!codec)
					printf("Failed to find hevc_v4l2m2m\n");
			}
		}
		if (!codec)
			codec = avcodec_find_decoder(codecID);
	}
	*/
	codec = avcodec_find_decoder(codecID);

	if (!codec)
		return DUPSTR(tsf::fmt("No suitable decoder found for %v", filename));

	d->CodecCtx = avcodec_alloc_context3(codec);
	if (d->CodecCtx == nullptr)
		return DUPSTR(tsf::fmt("avcodec_alloc_context3(%v) failed", filename));

	if (d->VideoStream >= 0) {
		e = avcodec_parameters_to_context(d->CodecCtx, d->FormatCtx->streams[d->VideoStream]->codecpar);
		if (e < 0)
			return DUPSTR(tsf::fmt("avcodec_parameters_to_context(%v) failed: %v", filename, AvErr(e)));
	}

	// Set hardware device context
	if (d->HwDeviceCtx) {
		//printf("Using hardware decoder (1)\n");
		d->CodecCtx->hw_device_ctx = av_buffer_ref(d->HwDeviceCtx);
		if (!d->CodecCtx->hw_device_ctx)
			return DUPSTR(tsf::fmt("Failed to set hardware device context for %v", codecName));

		// For v4l2m2m:
		//d->CodecCtx->pix_fmt = AV_PIX_FMT_DRM_PRIME;

		//printf("Using hardware decoder (settings get_format callback)\n");
		d->CodecCtx->get_format = get_format_drm_prime;
		//d->CodecCtx->get_format = [](AVCodecContext* ctx, const enum AVPixelFormat* pix_fmts) {
		//	printf("get_format!!\n");
		//	for (int i = 0; pix_fmts[i] != AV_PIX_FMT_NONE; i++) {
		//		printf("pix_fmts[%d] = %d\n", i, pix_fmts[i]);
		//		if (pix_fmts[i] == AV_PIX_FMT_VAAPI ||
		//		    pix_fmts[i] == AV_PIX_FMT_CUDA ||
		//		    pix_fmts[i] == AV_PIX_FMT_DXVA2_VLD) {
		//			return pix_fmts[i];
		//		}
		//	}
		//	return pix_fmts[0]; // Fallback to software format
		//};
	}

	e = avcodec_open2(d->CodecCtx, codec, nullptr);
	if (e < 0)
		return DUPSTR(tsf::fmt("avcodec_open2(%v) failed: %v", filename, AvErr(e)));

	d->FrameA = av_frame_alloc();
	if (d->FrameA == nullptr)
		return DUPSTR(tsf::fmt("av_frame_alloc(%v) failed", filename));

	d->FrameB = av_frame_alloc();
	if (d->FrameB == nullptr)
		return DUPSTR(tsf::fmt("av_frame_alloc(dst) failed", filename));

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

// Do not free the codecName string. It is a static string.
void Decoder_VideoInfo(void* decoder, int* width, int* height, const char** codecName) {
	Decoder* d = (Decoder*) decoder;
	*width     = d->CodecCtx->width;
	*height    = d->CodecCtx->height;
	*codecName = avcodec_get_name(d->CodecCtx->codec_id);
}

void Decoder_VideoSize(void* decoder, int* width, int* height) {
	Decoder* d = (Decoder*) decoder;
	*width     = d->CodecCtx->width;
	*height    = d->CodecCtx->height;
}

// Decode the next frame in the video file
char* Decoder_ReadAndReceiveFrame(void* decoder, AVFrame** output) {
	int       e      = 0;
	Decoder*  d      = (Decoder*) decoder;
	AVPacket* packet = d->DecodePacket;

	while (true) {
		e = av_read_frame(d->FormatCtx, packet);
		if (e == AVERROR_EOF)
			return ERROR_EOF;
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

		e = avcodec_receive_frame(d->CodecCtx, d->FrameA);
		if (e == AVERROR_EOF)
			return ERROR_EOF;
		else if (e == AVERROR(EAGAIN))
			continue;
		else if (e < 0)
			return DUPSTR(tsf::fmt("avcodec_receive_frame() failed: %v", AvErr(e)));

		return Decoder_ExtractFrame(d, output);
	}
}

// If there is a frame available, return it
char* Decoder_ReceiveFrame(void* decoder, AVFrame** output) {
	int       e      = 0;
	Decoder*  d      = (Decoder*) decoder;
	AVPacket* packet = d->DecodePacket;

	while (true) {
		e = avcodec_receive_frame(d->CodecCtx, d->FrameA);
		if (e == AVERROR_EOF)
			return ERROR_EOF;
		else if (e == AVERROR(EAGAIN))
			return ERROR_EAGAIN;
		else if (e < 0)
			return DUPSTR(tsf::fmt("avcodec_receive_frame() failed: %v", AvErr(e)));

		return Decoder_ExtractFrame(d, output);
	}
}

// Read the next packet out of the file, and return a copy of it.
// This is a low level function built for testing the decoder in streaming mode.
// We use this function to read packets out of an MP4 file, and then feed them
// into another explicit 'h264' decoder.
// This function is inherently wasteful because it clones the contents of the packet.
// That's not something you'd normally want to do.
// The caller must free() the packet when done.
char* Decoder_NextPacket(void* decoder, void** packet, size_t* packetSize, int64_t* pts, int64_t* dts) {
	int       e = 0;
	Decoder*  d = (Decoder*) decoder;
	AVPacket* p = d->DecodePacket;

	while (true) {
		e = av_read_frame(d->FormatCtx, p);
		if (e == AVERROR_EOF)
			return ERROR_EOF;
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
			*pts        = p->pts;
			*dts        = p->dts;
		}
		av_packet_unref(p);
		if (err != nullptr)
			return err;
		if (!isMyStream)
			continue;

		return nullptr;
	}
}

// Decode a packet from a video stream, but do not attempt to receive a frame.
char* Decoder_OnlyDecodePacket(void* decoder, const void* packet, size_t packetSize) {
	int       e = 0;
	Decoder*  d = (Decoder*) decoder;
	AVPacket* p = d->DecodePacket;

	p->data = (uint8_t*) packet;
	p->size = packetSize;

	e = avcodec_send_packet(d->CodecCtx, p);
	if (e < 0)
		return DUPSTR(tsf::fmt("avcodec_send_packet() failed: %v", AvErr(e)));

	return nullptr;
}

// Decode a packet from a video stream, and then try to receive a frame.
char* Decoder_DecodePacket(void* decoder, const void* packet, size_t packetSize, AVFrame** output) {
	Decoder* d = (Decoder*) decoder;

	char* err = Decoder_OnlyDecodePacket(decoder, packet, packetSize);
	if (err != nullptr)
		return err;

	int e = avcodec_receive_frame(d->CodecCtx, d->FrameA);
	if (e == AVERROR_EOF)
		return ERROR_EOF;
	else if (e == AVERROR(EAGAIN))
		return ERROR_EAGAIN;
	else if (e < 0)
		return DUPSTR(tsf::fmt("avcodec_receive_frame() failed: %v", AvErr(e)));

	return Decoder_ExtractFrame(d, output);
}

// Return the native PTS in nanoseconds, or -1 on error
int64_t Decoder_PTSNano(void* decoder, int64_t pts) {
	Decoder* d = (Decoder*) decoder;
	if (d->FormatCtx == nullptr || d->VideoStream >= d->FormatCtx->nb_streams)
		return -1;
	return av_rescale_q(pts, d->FormatCtx->streams[d->VideoStream]->time_base, (AVRational) {1, 1000000000});
}

} // extern "C"