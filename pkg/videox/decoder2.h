#include "common.h"

#include <string.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

char* MakeDecoder(const char* filename, const char* codecName, void** output_decoder);
void  Decoder_Close(void* decoder);
void  Decoder_VideoSize(void* decoder, int* width, int* height);
char* Decoder_NextFrame(void* decoder, YUVImage* output);
char* Decoder_NextPacket(void* decoder, void** packet, size_t* packetSize);
char* Decoder_DecodePacket(void* decoder, const void* packet, size_t packetSize, YUVImage* output);

#ifdef __cplusplus
}
#endif
