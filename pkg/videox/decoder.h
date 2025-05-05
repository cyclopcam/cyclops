#include "common.h"

#include <string.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

char*   MakeDecoder(const char* filename, const char* codecName, void** output_decoder);
void    Decoder_Close(void* decoder);
void    Decoder_VideoSize(void* decoder, int* width, int* height);
char*   Decoder_NextFrame(void* decoder, AVFrame** output);
char*   Decoder_NextPacket(void* decoder, void** packet, size_t* packetSize, int64_t* pts, int64_t* dts);
char*   Decoder_DecodePacket(void* decoder, const void* packet, size_t packetSize, AVFrame** output);
int64_t Decoder_PTSNano(void* decoder, int64_t pts);

#ifdef __cplusplus
}
#endif
