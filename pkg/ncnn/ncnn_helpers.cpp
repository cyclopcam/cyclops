#include "simpleocv.h"
#include <assert.h>

// Transpose a matrix by operating on blocks of N x N pixels at a time.
// The block size is hardcoded inside this function to 8 x 8, but I haven't
// measured whether this is optimal.
template <typename T>
void TransposeT(const ncnn::Mat& in, ncnn::Mat& out) {
	size_t w         = in.w;
	size_t h         = in.h;
	size_t stride    = in.w;
	size_t outStride = out.w;
	size_t blockSize = 8;
	for (size_t xBlock = 0; xBlock < w; xBlock += blockSize) {
		size_t blockWidth = std::min(blockSize, w - xBlock);
		for (size_t yBlock = 0; yBlock < h; yBlock += blockSize) {
			size_t   blockHeight = std::min(blockSize, h - yBlock);
			const T* src         = (const T*) in.data;
			T*       dst         = (T*) out.data;
			src += xBlock + yBlock * stride;
			dst += yBlock + xBlock * outStride;
			for (size_t x = 0; x < blockWidth; x++) {
				for (size_t y = 0; y < blockHeight; y++) {
					*dst = *src;
					src += stride;
					dst++;
				}
				// right by one, and up by blockHeight
				src += 1 - blockHeight * stride;
				// down by one, and left by blockHeight
				dst += outStride - blockHeight;
			}
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
