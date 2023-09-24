#include "simpleocv.h"
#include <assert.h>

template <typename T>
void TransposeT(const ncnn::Mat& in, ncnn::Mat& out) {
	size_t w      = in.w;
	size_t h      = in.h;
	size_t stride = in.w;
	for (size_t x = 0; x < w; x++) {
		const T* src = (const T*) in.data;
		T*       dst = (T*) out.data;
		src += x;
		dst += x * out.w;
		for (size_t y = 0; y < h; y++) {
			*dst = *src;
			src += stride;
			dst++;
		}
	}
}
void Transpose(const ncnn::Mat& in, ncnn::Mat& out, ncnn::Allocator* allocator) {
	assert(in.dims == 2);
	if (out.dims != in.dims || out.w != in.h || out.h != in.w || out.elemsize != in.elemsize) {
		out.create(in.h, in.w, in.elemsize, allocator);
	}

	if (in.elemsize == 4) {
		TransposeT<uint32_t>(in, out);
	} else if (in.elemsize == 2) {
		TransposeT<uint16_t>(in, out);
	} else if (in.elemsize == 1) {
		TransposeT<uint8_t>(in, out);
	} else {
		assert(false);
	}
}