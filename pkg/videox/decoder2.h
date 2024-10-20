#include "common.h"

#include <string.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

char* MakeDecoder(const char* filename, const char* codecName, void** output_decoder);
void  Decoder_Close(void* decoder);
char* Decoder_NextFrame(void* decoder, YUVImage* output);
char* Decoder_DecodePacket(void* decoder, const void* packet, size_t packetSize, YUVImage* output);

#ifdef __cplusplus
}
#endif
