#pragma once

// For intellisense
#include <hailo/hailort.h>
#include <hailo/hailort_common.hpp>
#include <hailo/vdevice.hpp>
#include <hailo/infer_model.hpp>

#include <memory>
#include <vector>
#include <string>

#include "defs.h"
#include "pagealloc.h"

inline cyStatus _make_own_status(hailo_status s) {
	switch (s) {
	case HAILO_SUCCESS:
		return cySTATUS_OK;
	case HAILO_TIMEOUT:
		return cySTATUS_TIMEOUT;
	default:
		return (cyStatus) (s + cySTATUS_HAILO_STATUS_OFFSET);
	}
}

// List of buffers that were allocated with malloc(), which we free()
// in our destructor.
class BufferList {
public:
	std::vector<void*> Buffers;

	BufferList& operator=(BufferList&& b) = default;

	~BufferList() {
		for (auto b : Buffers) {
			PageAlignedFree(b);
		}
	}

	void Add(void* p) {
		Buffers.push_back(p);
	}
};

struct NNDevice {
	std::unique_ptr<hailort::VDevice> VDevice;
	std::string                       Name; // eg "8L";
};

class NNModel {
public:
	NNDevice*                                      Device = nullptr;
	std::shared_ptr<hailort::InferModel>           InferModel;
	std::shared_ptr<hailort::ConfiguredInferModel> ConfiguredInferModel;
	int                                            BatchSize = 0;

	NNModel(NNDevice* device, std::shared_ptr<hailort::InferModel> inferModel, std::shared_ptr<hailort::ConfiguredInferModel> configuredInferModel, int batchSize) {
		Device               = device,
		InferModel           = inferModel;
		ConfiguredInferModel = configuredInferModel;
		BatchSize            = batchSize;
	}
	~NNModel();
};

class OutTensor {
public:
	uint8_t*               Data; // This data needs to be freed once the job is finished
	std::string            Name;
	hailo_quant_info_t     Quant;
	hailo_3d_image_shape_t Shape;
	hailo_format_t         Format;

	OutTensor(uint8_t* data, const std::string& name, const hailo_quant_info_t& quant, const hailo_3d_image_shape_t& shape, hailo_format_t format) {
		Data   = data;
		Name   = name;
		Quant  = quant;
		Shape  = shape;
		Format = format;
	}

	static bool SortFunction(const OutTensor& l, const OutTensor& r) {
		return l.Shape.width < r.Shape.width;
	}
};

// A job that is busy executing on the Hailo TPU
class OwnAsyncJobHandle {
public:
	NNModel*                                             Model;
	std::vector<hailort::ConfiguredInferModel::Bindings> Bindings;   // Length equal to batch size
	std::vector<OutTensor>                               OutTensors; // Parallel to Bindings
	hailort::AsyncInferJob                               HailoJob;
	BufferList                                           Buffers;

	OwnAsyncJobHandle(NNModel*                                               model,
	                  std::vector<hailort::ConfiguredInferModel::Bindings>&& bindings,
	                  std::vector<OutTensor>&&                               outTensors,
	                  hailort::AsyncInferJob&&                               hailoJob,
	                  BufferList&&                                           buffers) {
		Model      = model;
		Bindings   = std::move(bindings);
		OutTensors = std::move(outTensors);
		HailoJob   = std::move(hailoJob);
		Buffers    = std::move(buffers);
	}

	~OwnAsyncJobHandle() {
		// Assign a new AsyncInferJob to HailoJob, thereby invoking the destructor
		// of our own HailoJob.
		// This will wait for the job to finish.
		// We can't free the memory until we're sure that the job is finished.
		HailoJob = hailort::AsyncInferJob();

		//printf("~OwnAsyncJobHandle 1\n");
		//fflush(stdout);
		//OutTensors = std::vector<OutTensor>();
		//printf("~OwnAsyncJobHandle 2\n");
		//fflush(stdout);
		//HailoJob.detach();
		//printf("~OwnAsyncJobHandle 3\n");
		//fflush(stdout);
	}
};