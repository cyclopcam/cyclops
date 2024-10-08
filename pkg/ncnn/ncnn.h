#include <string.h>
#include <stdint.h>

#include "sharedHeader.h"

#ifdef __cplusplus
extern "C" {
#endif

//typedef void* NcnnDetector;
typedef struct NcnnDetector NcnnDetector;

enum DetectorFlags {
	DetectorFlagSingleThreaded = 1,
};

void InitNcnn();

NcnnDetector* CreateDetector(int detectorFlags, const char* type, const char* param, const char* bin, int width, int height);
void          DeleteDetector(NcnnDetector* detector);
void          DetectObjects(NcnnDetector* detector, int nchan, const uint8_t* img, int width, int height, int stride,
                            int detectFlags, float minProbability, float nmsIouThreshold, int maxDetections, Detection* detections, int* numDetections);

#ifdef __cplusplus
}
#endif
