// Run 'build' from the project root to build this debug/analysis program

// ncnn includes
#include "simpleocv.h"

// Our own ncnn wrapper/helper
#include "ncnn_helpers.h"

#include <assert.h>

void Print(const ncnn::Mat& m) {
	for (int y = 0; y < m.h; y++) {
		for (int x = 0; x < m.w; x++) {
			printf("%2.0f ", m.row(y)[x]);
		}
		printf("\n");
	}
}

void TestTranspose() {
	for (int width = 1; width < 20; width++) {
		for (int height = 1; height < 20; height++) {
			ncnn::Mat in(width, height);
			for (int y = 0; y < in.h; y++) {
				for (int x = 0; x < in.w; x++) {
					in.row(y)[x] = y * in.w + x;
				}
			}
			ncnn::Mat out;
			Transpose(in, out, nullptr);
			assert(out.w == in.h);
			assert(out.h == in.w);
			//Print(in);
			//Print(out);
			for (int y = 0; y < out.h; y++) {
				for (int x = 0; x < out.w; x++) {
					assert(out.row(y)[x] == x * out.h + y);
				}
			}
		}
	}
}

int main(int argc, char** argv) {
	return 0;
}