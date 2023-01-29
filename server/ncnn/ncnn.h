#include <string.h>
#include <stdint.h>

#include "sharedHeader.h"

#ifdef __cplusplus
extern "C" {
#endif

typedef void* NcnnDetector;

NcnnDetector CreateDetector(const char* type, const char* param, const char* bin);
void         DeleteDetector(NcnnDetector detector);
void         DetectObjects(NcnnDetector detector, int nchan, const uint8_t* img, int width, int height, int stride, int maxDetections, Detection* detections, int* numDetections);

#ifdef __cplusplus
}
#endif
