#include <stdio.h>
#include "layer.h"
#include "net.h"
#include "simpleocv.h"

#include "sharedHeader.h"
#include "yolo.h"

struct NcnnDetector {
	ModelTypes Type;
	ncnn::Net  Net;
	int        Width  = 0;
	int        Height = 0;
};

extern "C" {

NcnnDetector* CreateDetector(const char* type, const char* param, const char* bin, int width, int height) {
	ModelTypes mtype;
	if (strcmp(type, "yolov7") == 0)
		mtype = ModelTypes::YOLOv7;
	else if (strcmp(type, "yolov8") == 0)
		mtype = ModelTypes::YOLOv8;
	else
		return nullptr;

	auto det = new NcnnDetector();
	//det->Net.opt.num_threads = 1;
	det->Type   = mtype;
	det->Width  = width;
	det->Height = height;
	int r1      = det->Net.load_param(param);
	int r2      = det->Net.load_model(bin);
	if (r1 == -1 || r2 == -1) {
		delete det;
		return nullptr;
	}
	return det;
}

void DeleteDetector(NcnnDetector* detector) {
	delete detector;
}

void DetectObjects(NcnnDetector* detector, int nchan, const uint8_t* img, int width, int height, int stride,
                   float minProbability, float nmsThreshold, int maxDetections, Detection* detections, int* numDetections) {
	// make sure nchan === CV_8UC<nchan>
	static_assert(CV_8UC1 == 1, "CV_8UC3 != 1");
	static_assert(CV_8UC3 == 3, "CV_8UC3 != 3");
	static_assert(CV_8UC4 == 4, "CV_8UC3 != 4");
	cv::Mat                mat(height, width, nchan, (void*) img);
	std::vector<Detection> objects;
	if (detector->Type == ModelTypes::YOLOv7 ||
	    detector->Type == ModelTypes::YOLOv8) {
		DetectYOLO(detector->Type, detector->Net, detector->Width, detector->Height, minProbability, nmsThreshold, mat, objects);
	}
	int n = std::min(maxDetections, (int) objects.size());
	for (int i = 0; i < n; i++) {
		detections[i] = objects[i];
	}
	*numDetections = n;
}
}