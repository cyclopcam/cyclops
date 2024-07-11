#pragma once

// In order to build an NN accelerator, you must expose the following functions:

#include <stdint.h>

typedef struct _NNModelSetup {
	int BatchSize;
} NNModelSetup;

typedef struct _NNModelInfo {
	int BatchSize;
	int NChan;
	int Width;
	int Height;
} NNModelInfo;

typedef struct _NNMObjectDetection {
	uint32_t ClassID;
	float    Confidence;
	float    X;
	float    Y;
	float    W;
	float    H;
} NNMObjectDetection;

typedef int (*nnm_load_model_func)(const char* filename, const NNModelSetup* setup, void** model);
typedef void (*nnm_close_model_func)(void* model);
typedef void (*nnm_model_info_func)(void* model, NNModelInfo* info);
typedef const char* (*nnm_status_str_func)(int s);
typedef int (*nnm_run_model_func)(void* model, int batchSize, int width, int height, int nchan, const void* data, void** async_handle);
typedef int (*nnm_wait_for_job_func)(void* async_handle, uint32_t max_wait_milliseconds);
typedef int (*nnm_get_object_detections_func)(void* async_handle, uint32_t max_wait_milliseconds, int maxDetections, NNMObjectDetection* detections, int* numDetections);
typedef void (*nnm_finish_run_func)(void* async_handle);

#ifdef __cplusplus
extern "C" {
#endif

int         nnm_load_model(const char* filename, const NNModelSetup* setup, void** model);
void        nnm_close_model(void* model);
void        nnm_model_info(void* model, NNModelInfo* info);
const char* nnm_status_str(int s);
int         nnm_run_model(void* model, int batchSize, int width, int height, int nchan, const void* data, void** async_handle);
int         nnm_wait_for_job(void* async_handle, uint32_t max_wait_milliseconds);
int         nnm_get_object_detections(void* async_handle, uint32_t max_wait_milliseconds, int maxDetections, NNMObjectDetection* detections, int* numDetections);
void        nnm_finish_run(void* async_handle);

#ifdef __cplusplus
}
#endif