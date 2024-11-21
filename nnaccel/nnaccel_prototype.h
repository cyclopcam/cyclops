#pragma once

// In order to build an NN accelerator, you must expose the following functions:

#include <stdint.h>
#include <stdlib.h>

// Parameters that need to be configured at model compile time
typedef struct _NNModelSetup {
	int   BatchSize;
	float ProbabilityThreshold;
	float NmsIouThreshold;
} NNModelSetup;

typedef struct _NNModelInfo {
	int BatchSize;
	int NChan;
	int Width;
	int Height;
} NNModelInfo;

typedef struct _NNAObjectDetection {
	uint32_t ClassID;
	float    Confidence;
	int      X;
	int      Y;
	int      Width;
	int      Height;
} NNAObjectDetection;

// nna_run_model_func takes batchStride so that you can pad every batch element to the memory page size.
// Hailo wants all buffers aligned to page size, and padded up to an integer page size.
// We could also allow each element of the batch to come in from a different pointer, but I'll make
// that change if it becomes necessary.

typedef void (*nna_model_files_func)(const char** subdir, const char** ext);
typedef int (*nna_load_model_func)(const char* filename, const NNModelSetup* setup, void** model);
typedef void (*nna_close_model_func)(void* model);
typedef void (*nna_model_info_func)(void* model, NNModelInfo* info);
typedef const char* (*nna_status_str_func)(int s);
typedef int (*nna_run_model_func)(void* model, int batchSize, int batchStride, int width, int height, int nchan, int stride, const void* data, void** job_handle);
typedef int (*nna_wait_for_job_func)(void* job_handle, uint32_t max_wait_milliseconds);
typedef int (*nna_get_object_detections_func)(void* job_handle, int batchEl, size_t maxDetections, NNAObjectDetection** detections, size_t* numDetections);
typedef void (*nna_close_job_func)(void* job_handle);

//#ifdef __cplusplus
//extern "C" {
//#endif
//
//void        nna_model_files(const char** subdir, const char** ext);
//int         nna_load_model(const char* filename, const NNModelSetup* setup, void** model);
//void        nna_close_model(void* model);
//void        nna_model_info(void* model, NNModelInfo* info);
//const char* nna_status_str(int s);
//int         nna_run_model(void* model, int batchSize, int batchStride, int width, int height, int nchan, int stride, const void* data, void** job_handle);
//int         nna_wait_for_job(void* job_handle, uint32_t max_wait_milliseconds);
//int         nna_get_object_detections(void* job_handle, int batchEl, size_t maxDetections, NNAObjectDetection** detections, size_t* numDetections);
//void        nna_close_job(void* job_handle);
//
//#ifdef __cplusplus
//}
//#endif