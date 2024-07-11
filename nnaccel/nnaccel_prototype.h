#pragma once

// In order to build an NN accelerator, you must expose the following functions:

#include <stdint.h>
#include <stdlib.h>

typedef struct _NNModelSetup {
	int BatchSize;
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

typedef int (*nna_load_model_func)(const char* filename, const NNModelSetup* setup, void** model);
typedef void (*nna_close_model_func)(void* model);
typedef void (*nna_model_info_func)(void* model, NNModelInfo* info);
typedef const char* (*nna_status_str_func)(int s);
typedef int (*nna_run_model_func)(void* model, int batchSize, int width, int height, int nchan, const void* data, void** job_handle);
typedef int (*nna_wait_for_job_func)(void* job_handle, uint32_t max_wait_milliseconds);
typedef int (*nna_get_object_detections_func)(void* job_handle, size_t maxDetections, NNAObjectDetection** detections, size_t* numDetections);
typedef void (*nna_close_job_func)(void* job_handle);

#ifdef __cplusplus
extern "C" {
#endif

int         nna_load_model(const char* filename, const NNModelSetup* setup, void** model);
void        nna_close_model(void* model);
void        nna_model_info(void* model, NNModelInfo* info);
const char* nna_status_str(int s);
int         nna_run_model(void* model, int batchSize, int width, int height, int nchan, const void* data, void** job_handle);
int         nna_wait_for_job(void* job_handle, uint32_t max_wait_milliseconds);
int         nna_get_object_detections(void* job_handle, size_t maxDetections, NNAObjectDetection** detections, size_t* numDetections);
void        nna_close_job(void* job_handle);

#ifdef __cplusplus
}
#endif