#include <dlfcn.h>
#include <string.h>

#include "interface.h"

// An NN module that has been loaded dynamically from a shared library
typedef struct _NNModule {
	void*                          DLHandle;
	nnm_load_model_func            load_model;
	nnm_close_model_func           close_model;
	nnm_model_info_func            model_info;
	nnm_status_str_func            status_str;
	nnm_run_model_func             run_model;
	nnm_wait_for_job_func          wait_for_job;
	nnm_get_object_detections_func get_object_detections;
	nnm_finish_run_func            finish_run;
} NNModule;

extern "C" {

char* LoadNNModule(const char* filename, void** module) {
	void* lib = dlopen(filename, RTLD_NOW);
	if (lib == nullptr) {
		return strdup(dlerror());
	}

	NNModule m;
	m.DLHandle              = lib;
	m.load_model            = (nnm_load_model_func) dlsym(lib, "nnm_load_model");
	m.close_model           = (nnm_close_model_func) dlsym(lib, "nnm_close_model");
	m.model_info            = (nnm_model_info_func) dlsym(lib, "nnm_model_info");
	m.status_str            = (nnm_status_str_func) dlsym(lib, "nnm_status_str");
	m.run_model             = (nnm_run_model_func) dlsym(lib, "nnm_run_model");
	m.wait_for_job          = (nnm_wait_for_job_func) dlsym(lib, "nnm_wait_for_job");
	m.get_object_detections = (nnm_get_object_detections_func) dlsym(lib, "nnm_get_object_detections");
	m.finish_run            = (nnm_finish_run_func) dlsym(lib, "nnm_finish_run");

	char* err = nullptr;

	if (!m.load_model)
		err = strdup("Failed to find nnm_load_model in dynamic library");
	else if (!m.close_model)
		err = strdup("Failed to find nnm_close_model in dynamic library");
	else if (!m.model_info)
		err = strdup("Failed to find nnm_model_info in dynamic library");
	else if (!m.status_str)
		err = strdup("Failed to find nnm_status_str in dynamic library");
	else if (!m.run_model)
		err = strdup("Failed to find nnm_run_model in dynamic library");
	else if (!m.wait_for_job)
		err = strdup("Failed to find nnm_wait_for_job in dynamic library");
	else if (!m.get_object_detections)
		err = strdup("Failed to find nnm_get_object_detections in dynamic library");
	else if (!m.finish_run)
		err = strdup("Failed to find nnm_finish_run in dynamic library");

	if (err != nullptr) {
		dlclose(lib);
		return err;
	}

	NNModule* pm = new NNModule();
	*pm          = m;
	*module      = pm;
	return nullptr;
}

int NMLoadModel(void* nnModule, const char* filename, const NNModelSetup* setup, void** model) {
	NNModule* m = (NNModule*) nnModule;
	//printf("NMLoadModel %p\n", m->load_model);
	return m->load_model(filename, setup, model);
}

void NMCloseModel(void* nnModule, void* model) {
	NNModule* m = (NNModule*) nnModule;
	//printf("NMCloseModel %p\n", m->close_model);
	m->close_model(model);
}

void NMModelInfo(void* nnModule, void* model, NNModelInfo* info) {
	NNModule* m = (NNModule*) nnModule;
	m->model_info(model, info);
}

const char* NMStatusStr(void* nnModule, int s) {
	NNModule* m = (NNModule*) nnModule;
	return m->status_str(s);
}

int NMRunModel(void* nnModule, void* model, int batchSize, int width, int height, int nchan, const void* data, void** async_handle) {
	NNModule* m = (NNModule*) nnModule;
	return m->run_model(model, batchSize, width, height, nchan, data, async_handle);
}

int NMWaitForJob(void* nnModule, void* async_handle, uint32_t max_wait_milliseconds) {
	NNModule* m = (NNModule*) nnModule;
	return m->wait_for_job(async_handle, max_wait_milliseconds);
}

int NMGetObjectDetections(void* nnModule, void* async_handle, uint32_t max_wait_milliseconds, int maxDetections, NNMObjectDetection* detections, int* numDetections) {
	NNModule* m = (NNModule*) nnModule;
	return m->get_object_detections(async_handle, max_wait_milliseconds, maxDetections, detections, numDetections);
}

void NMFinishRun(void* nnModule, void* async_handle) {
	NNModule* m = (NNModule*) nnModule;
	m->finish_run(async_handle);
}
}