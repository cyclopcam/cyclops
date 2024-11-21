#include <dlfcn.h>
#include <string.h>

#include "interface.h"

// An NN accelerator module that has been loaded dynamically from a shared library
typedef struct _NNAccel {
	void*                          DLHandle;
	nna_model_files_func           model_files;
	nna_load_model_func            load_model;
	nna_close_model_func           close_model;
	nna_model_info_func            model_info;
	nna_status_str_func            status_str;
	nna_run_model_func             run_model;
	nna_wait_for_job_func          wait_for_job;
	nna_get_object_detections_func get_object_detections;
	nna_close_job_func             close_job;
} NNAccel;

extern "C" {

char* LoadNNAccel(const char* filename, void** module) {
	void* lib = dlopen(filename, RTLD_NOW);
	if (lib == nullptr) {
		return strdup(dlerror());
	}

	NNAccel m;
	m.DLHandle              = lib;
	m.model_files           = (nna_model_files_func) dlsym(lib, "nna_model_files");
	m.load_model            = (nna_load_model_func) dlsym(lib, "nna_load_model");
	m.close_model           = (nna_close_model_func) dlsym(lib, "nna_close_model");
	m.model_info            = (nna_model_info_func) dlsym(lib, "nna_model_info");
	m.status_str            = (nna_status_str_func) dlsym(lib, "nna_status_str");
	m.run_model             = (nna_run_model_func) dlsym(lib, "nna_run_model");
	m.wait_for_job          = (nna_wait_for_job_func) dlsym(lib, "nna_wait_for_job");
	m.get_object_detections = (nna_get_object_detections_func) dlsym(lib, "nna_get_object_detections");
	m.close_job             = (nna_close_job_func) dlsym(lib, "nna_close_job");

	char* err = nullptr;

	if (!m.model_files)
		err = strdup("Failed to find nna_model_files in dynamic library");
	else if (!m.load_model)
		err = strdup("Failed to find nna_load_model in dynamic library");
	else if (!m.close_model)
		err = strdup("Failed to find nna_close_model in dynamic library");
	else if (!m.model_info)
		err = strdup("Failed to find nna_model_info in dynamic library");
	else if (!m.status_str)
		err = strdup("Failed to find nna_status_str in dynamic library");
	else if (!m.run_model)
		err = strdup("Failed to find nna_run_model in dynamic library");
	else if (!m.wait_for_job)
		err = strdup("Failed to find nna_wait_for_job in dynamic library");
	else if (!m.get_object_detections)
		err = strdup("Failed to find nna_get_object_detections in dynamic library");
	else if (!m.close_job)
		err = strdup("Failed to find nna_close_job in dynamic library");

	if (err != nullptr) {
		dlclose(lib);
		return err;
	}

	NNAccel* pm = new NNAccel();
	*pm         = m;
	*module     = pm;
	return nullptr;
}

void NAModelFiles(void* nnModule, const char** subdir, const char** ext) {
	NNAccel* m = (NNAccel*) nnModule;
	m->model_files(subdir, ext);
}

int NALoadModel(void* nnModule, const char* filename, const NNModelSetup* setup, void** model) {
	NNAccel* m = (NNAccel*) nnModule;
	//printf("NALoadModel %p\n", m->load_model);
	return m->load_model(filename, setup, model);
}

void NACloseModel(void* nnModule, void* model) {
	NNAccel* m = (NNAccel*) nnModule;
	//printf("NACloseModel %p\n", m->close_model);
	m->close_model(model);
}

void NAModelInfo(void* nnModule, void* model, NNModelInfo* info) {
	NNAccel* m = (NNAccel*) nnModule;
	m->model_info(model, info);
}

const char* NAStatusStr(void* nnModule, int s) {
	NNAccel* m = (NNAccel*) nnModule;
	return m->status_str(s);
}

int NARunModel(void* nnModule, void* model, int batchSize, int batchStride, int width, int height, int nchan, int stride, const void* data, void** job_handle) {
	NNAccel* m = (NNAccel*) nnModule;
	return m->run_model(model, batchSize, batchStride, width, height, nchan, stride, data, job_handle);
}

int NAWaitForJob(void* nnModule, void* job_handle, uint32_t max_wait_milliseconds) {
	NNAccel* m = (NNAccel*) nnModule;
	return m->wait_for_job(job_handle, max_wait_milliseconds);
}

int NAGetObjectDetections(void* nnModule, void* job_handle, int batchEl, size_t maxDetections, NNAObjectDetection** detections, size_t* numDetections) {
	NNAccel* m = (NNAccel*) nnModule;
	return m->get_object_detections(job_handle, batchEl, maxDetections, detections, numDetections);
}

void NACloseJob(void* nnModule, void* job_handle) {
	NNAccel* m = (NNAccel*) nnModule;
	m->close_job(job_handle);
}
}