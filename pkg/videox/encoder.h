#include "common.h"

#include <string.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

enum EncoderType {
	EncoderTypePackets,     // Sending pre-encoded packets/NALUs to the encoder
	EncoderTypeImageFrames, // Sending image frames to the encoder
};

typedef struct EncoderParams {
#if LIBAVCODEC_VERSION_MAJOR < 59
	AVCodec* Codec;
#else
	const AVCodec* Codec;
#endif
	int                Width;
	int                Height;
	enum EncoderType   Type;
	AVRational         Timebase;
	AVRational         FPS;
	enum AVPixelFormat PixelFormatOutput;
	enum AVPixelFormat PixelFormatInput;
} EncoderParams;

char* MakeEncoderParams(const char* codec, int width, int height, enum AVPixelFormat pixelFormatInput, enum AVPixelFormat pixelFormatOutput, enum EncoderType encoderType, int fps, EncoderParams* encoderParams);
char* MakeEncoder(const char* format, const char* filename, EncoderParams* encoderParams, void** encoderOutput);
void  Encoder_Close(void* encoder);
char* Encoder_WriteNALU(void* encoder, int64_t dtsNano, int64_t ptsNano, int naluPrefixLen, const void* nalu, size_t naluLen);
char* Encoder_WritePacket(void* encoder, int64_t dtsNano, int64_t ptsNano, int isKeyFrame, const void* packetData, size_t packetLen);
char* Encoder_MakeFrameWriteable(void* encoder, AVFrame** frame);
char* Encoder_WriteFrame(void* encoder, int64_t ptsNano);
char* Encoder_WriteTrailer(void* encoder);
void  SetPacketDataPointer(void* pkt, const void* buf, size_t bufLen);
char* GetAvErrorStr(int averr);
int   AvCodecSendPacket(AVCodecContext* ctx, const void* buf, size_t bufLen);

#ifdef __cplusplus
}
#endif
