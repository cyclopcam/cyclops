#include <stdio.h>
#include "layer.h"
#include "net.h"

//struct NcnnDetector {
//	ncnn::Net Net;
//};
typedef ncnn::Net NcnnDetector;

extern "C" {

NcnnDetector* CreateDetector(const char* type, const char* param, const char* bin) {
	auto det = new NcnnDetector();
	//det->load_model()
	return det;
}

void DeleteDetector(NcnnDetector* detector) {
	delete detector;
}
}