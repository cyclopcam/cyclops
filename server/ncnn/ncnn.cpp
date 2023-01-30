#include <stdio.h>
#include "layer.h"
#include "net.h"
#include "simpleocv.h"

#include "sharedHeader.h"
#include "yolov7.h"

enum DetectorTypes {
	YOLOV7,
};

struct NcnnDetector {
	DetectorTypes Type;
	ncnn::Net     Net;
};

extern "C" {

NcnnDetector* CreateDetector(const char* type, const char* param, const char* bin) {
	if (strcmp(type, "yolov7") != 0) {
		return nullptr;
	}
	auto det  = new NcnnDetector();
	det->Type = YOLOV7;
	int r1    = det->Net.load_param(param);
	int r2    = det->Net.load_model(bin);
	if (r1 == -1 || r2 == -1) {
		delete det;
		return nullptr;
	}
	return det;
}

void DeleteDetector(NcnnDetector* detector) {
	delete detector;
}

void DetectObjects(NcnnDetector* detector, int nchan, const uint8_t* img, int width, int height, int stride, int maxDetections, Detection* detections, int* numDetections) {
	// make sure nchan === CV_8UC<nchan>
	static_assert(CV_8UC1 == 1, "CV_8UC3 != 1");
	static_assert(CV_8UC3 == 3, "CV_8UC3 != 3");
	static_assert(CV_8UC4 == 4, "CV_8UC3 != 4");
	cv::Mat                mat(height, width, nchan, (void*) img);
	std::vector<Detection> objects;
	if (detector->Type == YOLOV7) {
		DetectYOLOv7(detector->Net, 320, 0.25f, 0.45f, mat, objects);
	}
	int n = std::min(maxDetections, (int) objects.size());
	for (int i = 0; i < n; i++) {
		detections[i] = objects[i];
	}
	*numDetections = n;
}
}