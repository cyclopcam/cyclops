
// build & run:
//
// cd cyclops/server/ncnn
// g++ -g -std=c++17 -fopenmp -I. -I../../ncnn/build/src -I../../ncnn/src -L../../ncnn/build/src -o ncnn_test debug/ncnn_test.cpp yolov7.cpp ncnn.cpp -lgomp -lstdc++ -lncnn
// ./ncnn_test

#include "layer.h"
#include "net.h"
#include "simpleocv.h"

#include "ncnn.h"

#include <float.h>
#include <stdio.h>
#include <string>
#include <vector>

int main(int argc, char** argv) {
	const char* imagepath = "testdata/driveway001-man.jpg";
	cv::Mat     m         = cv::imread(imagepath, 1);
	if (m.empty()) {
		fprintf(stderr, "cv::imread %s failed\n", imagepath);
		return -1;
	}

	auto      detector = CreateDetector("yolov7", "models/yolov7-tiny.param", "models/yolov7-tiny.bin");
	Detection dets[100];
	int       numDetections = 0;
	DetectObjects(detector, 3, m.data, m.cols, m.rows, m.cols * 3, 100, dets, &numDetections);

	return 0;
}
