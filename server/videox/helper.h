#include <string.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

void* MakeEncoder(char** err, const char* format, const char* filename, int width, int height);
void  Encoder_Close(void* encoder);
void  Encoder_WritePacket(char** err, void* encoder, int64_t dts, int64_t pts, int naluPrefixLen, const void* nalu, size_t naluLen);
void  Encoder_WriteTrailer(char** err, void* encoder);

#ifdef __cplusplus
}
#endif
