#include <stdint.h>

#include "Simd/SimdLib.hpp"

typedef Simd::View<Simd::Allocator> View;

extern "C" {

void YUV420pToRGB(int width, int height, const uint8_t* y, const uint8_t* u, const uint8_t* v, int strideY, int strideU, int strideV, uint8_t* rgb, int strideRGB) {
	View yView(width, height, strideY, View::Gray8, const_cast<uint8_t*>(y));
	View uView(width / 2, height / 2, strideU, View::Gray8, const_cast<uint8_t*>(u));
	View vView(width / 2, height / 2, strideV, View::Gray8, const_cast<uint8_t*>(v));
	View rgbView(width, height, strideRGB, View::Rgb24, rgb);
	Simd::Yuv420pToRgb(yView, uView, vView, rgbView);
}

void ReduceHalf(int width, int height, int nchannel, const uint8_t* src, int srcStride, uint8_t* dst, int dstStride) {
	View::Format f;
	if (nchannel == 1)
		f = View::Gray8;
	else if (nchannel == 3)
		f = View::Rgb24;
	else if (nchannel == 4)
		f = View::Rgba32;
	else
		return;
	View srcView(width, height, srcStride, f, const_cast<uint8_t*>(src));
	View dstView(width / 2, height / 2, dstStride, f, dst);
	Simd::Reduce2x2(srcView, dstView);
}
}
