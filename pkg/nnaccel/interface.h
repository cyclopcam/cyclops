#pragma once

#include <malloc.h>
#include "../../nnaccel/nnaccel_prototype.h"

#ifdef __cplusplus
extern "C" {
#endif

// Load an NN module from a shared library
char* LoadNNAccel(const char* filename, void** nnModule);

// It might be possible to expose the dynamically loaded C function pointers into Go,
// it's much easier to provide these wrappers here, which can be used easily from cgo.

int         NALoadModel(void* nnModule, const char* filename, const NNModelSetup* setup, void** model);
void        NACloseModel(void* nnModule, void* model);
void        NAModelInfo(void* nnModule, void* model, NNModelInfo* info);
const char* NAStatusStr(void* nnModule, int s);
int         NARunModel(void* nnModule, void* model, int batchSize, int width, int height, int nchan, const void* data, void** async_handle);
int         NAWaitForJob(void* nnModule, void* async_handle, uint32_t max_wait_milliseconds);
int         NAGetObjectDetections(void* nnModule, void* async_handle, uint32_t max_wait_milliseconds, int maxDetections, NNMObjectDetection* detections, int* numDetections);
void        NAFinishRun(void* nnModule, void* async_handle);

#ifdef __cplusplus
}
#endif
