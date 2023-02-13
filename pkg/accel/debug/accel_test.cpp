/*

build & run:

cd pkg/accel
g++ -g -std=c++17 -fopenmp -I../../Simd/src -L../../Simd/build -o accel_test debug/accel_test.cpp accel.cpp -lgomp -lstdc++ -lSimd && ./accel_test

*/

#include <float.h>
#include <stdio.h>
#include <string>
#include <vector>

#include "Simd/SimdLib.hpp"

typedef Simd::View<Simd::Allocator> View;

using namespace std;

string TestData = "../../testdata";

vector<char> LoadFile(const string& path) {
	FILE* file = fopen(path.c_str(), "rb");
	if (file == NULL) {
		printf("Error: can't open file %s\n", path.c_str());
		return vector<char>();
	}
	fseek(file, 0, SEEK_END);
	auto size = ftell(file);
	fseek(file, 0, SEEK_SET);
	vector<char> data(size);
	fread(data.data(), 1, size, file);
	fclose(file);
	return data;
}

void TestYUV() {
	auto yRaw       = LoadFile(TestData + "/yuv/dump.y");
	auto uRaw       = LoadFile(TestData + "/yuv/dump.u");
	auto vRaw       = LoadFile(TestData + "/yuv/dump.v");
	int  width      = 320;
	int  height     = 240;
	int  strides[3] = {384, 192, 192}; // Hardcoded from when I dumped these files out of ffmpeg
	View y(width, height, strides[0], View::Gray8, yRaw.data());
	View u(width / 2, height / 2, strides[1], View::Gray8, uRaw.data());
	View v(width / 2, height / 2, strides[2], View::Gray8, vRaw.data());
	View rgb(width, height, View::Rgb24, nullptr);
	Simd::Yuv420pToRgb(y, u, v, rgb);
	rgb.Save(TestData + "/yuv/dump.jpg", SimdImageFileJpeg, 90);
}

int main(int argc, char** argv) {
	TestYUV();
	return 0;
}
