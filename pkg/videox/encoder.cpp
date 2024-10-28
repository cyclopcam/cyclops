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

const bool DebugEncoder = false;

struct Encoder {
#if LIBAVCODEC_VERSION_MAJOR < 59
	AVOutputFormat* Format = nullptr;
	AVCodec*        Codec  = nullptr;
#else
	const AVOutputFormat* Format = nullptr;
	const AVCodec*        Codec  = nullptr;
#endif
	AVFormatContext* OutFormatCtx = nullptr;
	AVCodecContext*  CodecCtx     = nullptr;
	AVStream*        OutStream    = nullptr;
	AVFrame*         InputFrame   = nullptr;
	AVFrame*         OutputFrame  = nullptr;
	AVPacket*        Packet       = nullptr;
	SwsContext*      SwsCtx       = nullptr;

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
		if (E->InputFrame)
			av_frame_free(&E->InputFrame);
		if (E->OutputFrame)
			av_frame_free(&E->OutputFrame);
		if (E->Packet)
			av_packet_free(&E->Packet);
		if (!(E->OutFormatCtx->oformat->flags & AVFMT_NOFILE))
			avio_closep(&E->OutFormatCtx->pb);
		if (E->OutFormatCtx)
			avformat_free_context(E->OutFormatCtx);
		if (E->CodecCtx)
			avcodec_free_context(&E->CodecCtx);
		if (E->SwsCtx)
			sws_freeContext(E->SwsCtx);
		delete E;
	}
};

// Get the string error message for the given error code
inline std::string AvErr(int e) {
	char msg[AV_ERROR_MAX_STRING_SIZE] = {0};
	av_make_error_string(msg, AV_ERROR_MAX_STRING_SIZE, e);
	return msg;
}

#define RETURN_ERROR_STATIC(msg) return strdup(msg)
#define RETURN_ERROR_STR(msg) return strdup(msg.c_str())
#define RETURN_ERROR_EOF() return strdup("EOF")

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

// codec is either a codec name such as "h264", or a specific encoder such as "libx264"
// pixelFormatInput is the input format that you will be sending to the encoder.
// pixelFormatOutput is the format of the file.
// If pixelFormatInput != pixelFormatOutput, then we use swscale to convert the input to the output.
// fps is allowed to be zero
char* MakeEncoderParams(const char* codec, int width, int height, AVPixelFormat pixelFormatInput, AVPixelFormat pixelFormatOutput, EncoderType encoderType, int fps, EncoderParams* encoderParams) {
	const char** encoders        = nullptr;
	const char*  h264_encoders[] = {"libx264", nullptr};
	const char*  h265_encoders[] = {"libx265", nullptr};

	// Notes on encoders:
	// These are the errors I got when trying to use these encoders.
	// I literally spent 30 seconds on each, and didn't make any attempt to go further.
	// This was all on WSL. Not surprising.
	// h264_nvenc:   [h264_nvenc @ 0x619000004180] dl_fn->cuda_dl->cuInit(0) failed -> CUDA_ERROR_OUT_OF_MEMORY: out of memory
	// h264_qsv:     [h264_qsv @ 0x619000004180] Specified pixel format yuv420p is invalid or not supported ---- OK this was on an AMD CPU, so of course. But strange error.
	// h264_v4l2m2m: [h264_v4l2m2m @ 0x619000004180] Could not find a valid device

	if (strcmp(codec, "h264") == 0) {
		encoders = h264_encoders;
	} else if (strcmp(codec, "h265") == 0) {
		encoders = h265_encoders;
	}

	if (encoders != nullptr) {
		// Try each encoder in turn, until we find one that's available
		for (int i = 0; encoders[i]; i++) {
			encoderParams->Codec = avcodec_find_encoder_by_name(encoders[i]);
			if (encoderParams->Codec)
				break;
		}
		if (encoderParams->Codec == nullptr)
			RETURN_ERROR_STR(tsf::fmt("Failed to find an encoder for '%v'", codec));
	} else {
		// Explicit encoder name (eg libx264)
		encoderParams->Codec = avcodec_find_encoder_by_name(codec);
		if (encoderParams->Codec == nullptr)
			RETURN_ERROR_STR(tsf::fmt("Failed to find encoder '%v'", codec));
	}

	// If FPS is 0, then just choose an arbitrary timebase,
	// and leave FPS undefined.
	AVRational timebase    = AVRational{1, fps};
	AVRational fpsRational = AVRational{fps, 1};
	if (fps == 0) {
		timebase    = AVRational{1, 30 * 1024};
		fpsRational = AVRational{0, 0};
	}
	encoderParams->Width             = width;
	encoderParams->Height            = height;
	encoderParams->Type              = encoderType;
	encoderParams->Timebase          = timebase;
	encoderParams->FPS               = fpsRational;
	encoderParams->PixelFormatInput  = pixelFormatInput;
	encoderParams->PixelFormatOutput = pixelFormatOutput;
	return nullptr;
}

// Format is allowed to be null, in which case we use filename to guess the format
char* MakeEncoder(const char* format, const char* filename, EncoderParams* encoderParams, void** encoderOutput) {
	//av_register_all();
	//avcodec_register_all();

	int e = 0;

	Encoder*       encoder = new Encoder();
	EncoderCleanup cleanup(encoder);

	encoder->Format = av_guess_format(format, filename, nullptr);
	if (encoder->Format == nullptr)
		RETURN_ERROR_STATIC("Failed to find format");

	if (avformat_alloc_output_context2(&encoder->OutFormatCtx, encoder->Format, nullptr, nullptr) < 0)
		RETURN_ERROR_STATIC("Failed to allocate output context");

	encoder->Codec = encoderParams->Codec;
	if (encoder->Codec == nullptr)
		RETURN_ERROR_STATIC("Codec is null");

	encoder->OutStream = avformat_new_stream(encoder->OutFormatCtx, nullptr);
	if (encoder->OutStream == nullptr)
		RETURN_ERROR_STATIC("Failed to allocate output format stream");

	encoder->CodecCtx = avcodec_alloc_context3(encoder->Codec);
	if (encoder->CodecCtx == nullptr)
		RETURN_ERROR_STATIC("Failed to allocate codec context");

	// ChatGPT suggested moving this to above avcodec_alloc_context3, and making the codec NULL
	//encoder->OutStream = avformat_new_stream(encoder->OutFormatCtx, encoder->Codec);
	//if (encoder->OutStream == nullptr)
	//	RETURN_ERROR_STATIC("Failed to allocate output format stream");

	encoder->CodecCtx->width     = encoderParams->Width;
	encoder->CodecCtx->height    = encoderParams->Height;
	encoder->CodecCtx->pix_fmt   = encoderParams->PixelFormatOutput;
	encoder->CodecCtx->time_base = encoderParams->Timebase;

	if (encoderParams->FPS.num != 0)
		encoder->CodecCtx->framerate = encoderParams->FPS;

	// Setting OutStream->time_base  is just a hint.
	// When avformat_write_header is called, then OutStream->time_base will likely be changed.
	// We can also leave it 0/0, and just let the library decide. I'm not sure which method is better.
	encoder->OutStream->time_base = encoderParams->Timebase;
	//encoder->OutStream->r_frame_rate   = encoderParams->FPS;
	//encoder->OutStream->avg_frame_rate = encoderParams->FPS;

	// Before ChatGPT shuffle
	//if (avcodec_parameters_to_context(encoder->CodecCtx, encoder->OutStream->codecpar) < 0)
	//	RETURN_ERROR_STATIC("avcodec_parameters_to_context failed");

	// After ChatGPT shuffle
	if (avcodec_parameters_from_context(encoder->OutStream->codecpar, encoder->CodecCtx) < 0)
		RETURN_ERROR_STATIC("avcodec_parameters_to_context failed");

	//if (encoder->Codec->id == AV_CODEC_ID_H264)
	//	encoder->CodecCtx->profile = FF_PROFILE_H264_HIGH;

	//if (encoder->OutFormatCtx->oformat->flags & AVFMT_GLOBALHEADER)
	//	encoder->CodecCtx->flags |= AV_CODEC_FLAG_GLOBAL_HEADER;

	// ChatGPT suggested moving this to AFTER avcodec_open2
	//if (avcodec_parameters_from_context(encoder->OutStream->codecpar, encoder->CodecCtx) < 0)
	//	RETURN_ERROR_STATIC("avcodec_parameters_from_context failed");

	if (avcodec_open2(encoder->CodecCtx, encoder->Codec, nullptr) < 0)
		RETURN_ERROR_STATIC("avcodec_open2 failed");

	if (avcodec_parameters_from_context(encoder->OutStream->codecpar, encoder->CodecCtx) < 0)
		RETURN_ERROR_STATIC("avcodec_parameters_from_context failed");

	if (!!(encoder->CodecCtx->flags & AVFMT_NOFILE))
		RETURN_ERROR_STATIC("codec does not write to a file");

	e = avio_open2(&encoder->OutFormatCtx->pb, filename, AVIO_FLAG_WRITE, nullptr, nullptr);
	if (e < 0)
		RETURN_ERROR_STR(tsf::fmt("avio_open2(%v) failed: %v", filename, AvErr(e)));

	e = avformat_write_header(encoder->OutFormatCtx, nullptr);
	if (e < 0)
		RETURN_ERROR_STR(tsf::fmt("avformat_write_header failed: %v", AvErr(e)));

	if (encoderParams->Type == EncoderTypeImageFrames) {
		// Allocate output frame buffer (typically YUV420P). This is the frame that is sent to the codec.
		encoder->OutputFrame = av_frame_alloc();
		if (encoder->OutputFrame == nullptr)
			RETURN_ERROR_STATIC("Failed to allocate output frame");
		encoder->OutputFrame->format = encoder->CodecCtx->pix_fmt;
		encoder->OutputFrame->width  = encoder->CodecCtx->width;
		encoder->OutputFrame->height = encoder->CodecCtx->height;
		e                            = av_frame_get_buffer(encoder->OutputFrame, 0);
		if (e < 0)
			RETURN_ERROR_STR(tsf::fmt("av_frame_get_buffer failed: %v", AvErr(e)));

		// If necessary, allocate a 2nd frame buffer for the input (eg RGB24)
		if (encoderParams->PixelFormatInput != encoderParams->PixelFormatOutput) {
			encoder->InputFrame = av_frame_alloc();
			if (encoder->InputFrame == nullptr)
				RETURN_ERROR_STATIC("Failed to allocate input frame");
			encoder->InputFrame->format = encoderParams->PixelFormatInput;
			encoder->InputFrame->width  = encoderParams->Width;
			encoder->InputFrame->height = encoderParams->Height;
			e                           = av_frame_get_buffer(encoder->InputFrame, 0);
			if (e < 0)
				RETURN_ERROR_STR(tsf::fmt("av_frame_get_buffer failed: %v", AvErr(e)));

			encoder->SwsCtx = sws_getContext(encoderParams->Width, encoderParams->Height, encoderParams->PixelFormatInput,
			                                 encoderParams->Width, encoderParams->Height, encoderParams->PixelFormatOutput,
			                                 SWS_POINT, nullptr, nullptr, nullptr);
			if (encoder->SwsCtx == nullptr)
				RETURN_ERROR_STATIC("Failed to allocate sws context");
		}
	}

	encoder->Packet = av_packet_alloc();
	if (encoder->Packet == nullptr)
		RETURN_ERROR_STATIC("Failed to allocate packet");

	if (DebugEncoder)
		av_dump_format(encoder->OutFormatCtx, 0, filename, 1);

	cleanup.E      = nullptr; // allow Encoder to survive
	*encoderOutput = encoder;
	return nullptr;
}

void Encoder_Close(void* _encoder) {
	// when EncoderCleanup goes out of scope, it will clean up
	EncoderCleanup cleanup((Encoder*) _encoder);
}

// Iff naluPrefixLen == 0, then we prepend 00 00 01 to the nalu.
// NOTE! We do not add the emulation prevention bytes here!
// We just add a 00 00 01 prefix. If you want to add the emulation prevention bytes,
// then do it yourself, before calling this function.
// dtsNano and ptsNano are in nanoseconds
char* Encoder_WriteNALU(void* _encoder, int64_t dtsNano, int64_t ptsNano, int naluPrefixLen, const void* _nalu, size_t naluLen) {
	auto encoder    = (Encoder*) _encoder;
	auto nalu       = (const uint8_t*) _nalu;
	auto payload    = nalu + naluPrefixLen;
	auto packetType = GetH264NALUType(payload);
	//if (packetType == H264NALUTypes::SPS && encoder->SeenSPS)
	//	return;
	//if (packetType == H264NALUTypes::PPS && encoder->SeenPPS)
	//	return;
	if (naluPrefixLen != 0 && naluPrefixLen != 3 && naluPrefixLen != 4) {
		RETURN_ERROR_STR(tsf::fmt("Invalid naluPrefixLen %v. May only be one of: [0, 3, 4]", naluPrefixLen));
	}

	if (packetType == H264NALUTypes::SPS) {
		if (naluPrefixLen)
			encoder->SPS.assign((const char*) _nalu, naluLen);
		else
			encoder->SPS = WithPrefix(_nalu, naluLen);
		return nullptr;
	}
	if (packetType == H264NALUTypes::PPS) {
		if (naluPrefixLen)
			encoder->PPS.assign((const char*) _nalu, naluLen);
		else
			encoder->PPS = WithPrefix(_nalu, naluLen);
		return nullptr;
	}
	if ((encoder->SPS.size() == 0 || encoder->PPS.size() == 0) && IsVisualPacket(packetType)) {
		// The codec/format needs SPS and PPS before any frames, so we can't write frames yet
		return nullptr;
	}

	AVRational timeBase = encoder->OutStream->time_base;
	AVPacket*  pkt      = encoder->Packet;
	pkt->dts            = av_rescale_q(dtsNano, AVRational{1, 1000000000}, timeBase);
	pkt->pts            = av_rescale_q(ptsNano, AVRational{1, 1000000000}, timeBase);
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

	int e = av_interleaved_write_frame(encoder->OutFormatCtx, pkt);
	if (e < 0) {
		uint8_t bytes[4];
		int     i = 0;
		for (i = 0; i < sizeof(bytes) - 1 && nalu[i]; i++)
			bytes[i] = nalu[i];
		bytes[i] = 0;
		RETURN_ERROR_STR(tsf::fmt("Failed to write packet (%02x %02x %02x %02x ...) len: %v, error: %v", bytes[0], bytes[1], bytes[2], bytes[3], naluLen, AvErr(e)));
	}
	return nullptr;
}

// dtsNano and ptsNano are in nanoseconds
char* Encoder_WritePacket(void* _encoder, int64_t dtsNano, int64_t ptsNano, int isKeyFrame, const void* packetData, size_t packetLen) {
	auto encoder = (Encoder*) _encoder;

	AVRational timeBase = encoder->OutStream->time_base;
	AVPacket*  pkt      = encoder->Packet;
	pkt->dts            = av_rescale_q(dtsNano, AVRational{1, 1000000000}, timeBase);
	pkt->pts            = av_rescale_q(ptsNano, AVRational{1, 1000000000}, timeBase);
	pkt->stream_index   = encoder->OutStream->id;
	if (!!isKeyFrame)
		pkt->flags |= AV_PKT_FLAG_KEY;

	pkt->data = (uint8_t*) packetData;
	pkt->size = (int) packetLen;

	int e = av_interleaved_write_frame(encoder->OutFormatCtx, pkt);
	if (e < 0) {
		const uint8_t* data     = (const uint8_t*) packetData;
		uint8_t        bytes[8] = {0};
		for (int i = 0; i < sizeof(bytes) && i < packetLen; i++)
			bytes[i] = data[i];
		RETURN_ERROR_STR(
		    tsf::fmt("Failed to write packet (%02x %02x %02x %02x %02x %02x %02x %02x ...) len: %v, error: %v",
		             bytes[0], bytes[1], bytes[2], bytes[3], bytes[4], bytes[5], bytes[6], bytes[7],
		             (int) packetLen, AvErr(e)));
	}
	return nullptr;
}

char* Encoder_MakeFrameWriteable(void* _encoder, AVFrame** _frame) {
	Encoder* encoder = (Encoder*) _encoder;
	AVFrame* frame   = encoder->InputFrame != nullptr ? encoder->InputFrame : encoder->OutputFrame;
	int      e       = av_frame_make_writable(frame);
	if (e < 0)
		RETURN_ERROR_STR(tsf::fmt("av_frame_make_writable failed: %v", AvErr(e)));
	*_frame = frame;
	return nullptr;
}

// Take the frames that have been sent to the encoder, and write the resulting packets
// to the file. Normally it's 1:1 (1 frame -> 1 packet), but it could be different.
char* WriteBufferedPackets(Encoder* encoder) {
	while (true) {
		int e = avcodec_receive_packet(encoder->CodecCtx, encoder->Packet);
		if (e == AVERROR(EAGAIN) || e == AVERROR_EOF)
			return nullptr;
		if (e < 0)
			RETURN_ERROR_STR(tsf::fmt("avcodec_receive_packet failed: %v", AvErr(e)));

		// This next line would appropriate if we used the codec time base when writing the frame, but
		// we already use the OutStream timebase when writing the frame, so there's no need to do any
		// further adjustment here.
		//av_packet_rescale_ts(encoder->Packet, encoder->CodecCtx->time_base, encoder->OutStream->time_base);

		encoder->Packet->stream_index = encoder->OutStream->index;
		e                             = av_interleaved_write_frame(encoder->OutFormatCtx, encoder->Packet);
		av_packet_unref(encoder->Packet);
		if (e < 0)
			RETURN_ERROR_STR(tsf::fmt("av_interleaved_write_frame failed: %v", AvErr(e)));
	}
	return nullptr;
}

// ptsNano is in nanoseconds
char* Encoder_WriteFrame(void* _encoder, int64_t ptsNano) {
	Encoder* encoder = (Encoder*) _encoder;
	if (encoder->InputFrame != nullptr) {
		// Convert from input format to output format (eg RGB24 to YUV420P)
		int e = av_frame_make_writable(encoder->OutputFrame);
		if (e < 0)
			RETURN_ERROR_STR(tsf::fmt("av_frame_make_writable(2) failed: %v", AvErr(e)));
		sws_scale(encoder->SwsCtx,
		          encoder->InputFrame->data, encoder->InputFrame->linesize, 0, encoder->InputFrame->height,
		          encoder->OutputFrame->data, encoder->OutputFrame->linesize);
	}

	encoder->OutputFrame->pts = av_rescale_q(ptsNano, AVRational{1, 1000000000}, encoder->OutStream->time_base);

	// Do the actual codec magic
	int e = avcodec_send_frame(encoder->CodecCtx, encoder->OutputFrame);
	if (e < 0)
		RETURN_ERROR_STR(tsf::fmt("avcodec_send_frame failed: %v", AvErr(e)));

	// Write the resulting packets to the file
	return WriteBufferedPackets(encoder);
}

char* Encoder_WriteTrailer(void* _encoder) {
	Encoder* encoder = (Encoder*) _encoder;

	// Flush the encoder
	int e = avcodec_send_frame(encoder->CodecCtx, nullptr);
	if (e < 0)
		RETURN_ERROR_STR(tsf::fmt("avcodec_send_frame (flush) failed: %v", AvErr(e)));

	// Write remaining packets (if any)
	char* err = WriteBufferedPackets(encoder);
	if (err != nullptr)
		return err;

	e = av_write_trailer(encoder->OutFormatCtx);
	if (e < 0)
		RETURN_ERROR_STR(tsf::fmt("av_write_trailer failed: %v", AvErr(e)));
	return nullptr;
}

void SetPacketDataPointer(void* _pkt, const void* buf, size_t bufLen) {
	tsf::print("SetPacketDataPointer %v %v %v\n", _pkt, buf, bufLen);
	AVPacket* pkt = (AVPacket*) _pkt;
	pkt->data     = (uint8_t*) buf;
	pkt->size     = (int) bufLen;
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