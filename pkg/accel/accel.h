#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

void YUV420pToRGB(int width, int height, const uint8_t* y, const uint8_t* u, const uint8_t* v, int strideY, int strideU, int strideV, uint8_t* rgb, int strideRGB);

#ifdef __cplusplus
}
#endif
