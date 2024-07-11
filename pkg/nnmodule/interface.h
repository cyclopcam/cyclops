#pragma once

#include <malloc.h>
#include "../../modules/module_prototype.h"

#ifdef __cplusplus
extern "C" {
#endif

// Load an NN module from a shared library
char* LoadNNModule(const char* filename, void** nnModule);

// It might be possible to expose the dynamically loaded C function pointers into Go,
// it's much easier to provide these wrappers here, which can be used easily from cgo.

int         NMLoadModel(void* nnModule, const char* filename, const NNModelSetup* setup, void** model);
void        NMCloseModel(void* nnModule, void* model);
void        NMModelInfo(void* nnModule, void* model, NNModelInfo* info);
const char* NMStatusStr(void* nnModule, int s);
int         NMRunModel(void* nnModule, void* model, int batchSize, int width, int height, int nchan, const void* data, void** async_handle);
int         NMWaitForJob(void* nnModule, void* async_handle, uint32_t max_wait_milliseconds);
int         NMGetObjectDetections(void* nnModule, void* async_handle, uint32_t max_wait_milliseconds, int maxDetections, NNMObjectDetection* detections, int* numDetections);
void        NMFinishRun(void* nnModule, void* async_handle);

#ifdef __cplusplus
}
#endif
