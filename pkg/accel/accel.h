#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

void YUV420pToRGB(int width, int height, const uint8_t* y, const uint8_t* u, const uint8_t* v, int strideY, int strideU, int strideV, uint8_t* rgb, int strideRGB);

// Shrink by 2x. nchannel must be 1,3,4, which correspond to Gray, RGB, RGBA.
void ReduceHalf(int width, int height, int nchannel, const uint8_t* src, int srcStride, uint8_t* dst, int dstStride);

#ifdef __cplusplus
}
#endif
