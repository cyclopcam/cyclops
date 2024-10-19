#include "common.h"

#include <string.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

void* MakeEncoder(char** err, const char* format, const char* filename, int width, int height);
void  Encoder_Close(void* encoder);
void  Encoder_WriteNALU(char** err, void* encoder, int64_t dts, int64_t pts, int naluPrefixLen, const void* nalu, size_t naluLen);
void  Encoder_WritePacket(char** err, void* encoder, int64_t dts, int64_t pts, int isKeyFrame, const void* packetData, size_t packetLen);
void  Encoder_WriteTrailer(char** err, void* encoder);
void  SetPacketDataPointer(void* pkt, const void* buf, size_t bufLen);
char* GetAvErrorStr(int averr);
int   AvCodecSendPacket(AVCodecContext* ctx, const void* buf, size_t bufLen);

#ifdef __cplusplus
}
#endif
