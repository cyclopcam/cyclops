#include <stdio.h>
#include "layer.h"
#include "net.h"

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
	det->Net.load_param(param);
	det->Net.load_model(bin);
	return det;
}

void DeleteDetector(NcnnDetector* detector) {
	delete detector;
}
}