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

// List of buffers that are freed by our destructor
class BufferList {
public:
	std::vector<void*> Buffers;

	BufferList& operator=(BufferList&& b) = default;

	~BufferList() {
		for (auto b : Buffers) {
			free(b);
		}
	}

	void Add(void* p) {
		Buffers.push_back(p);
	}
};

struct NNModel {
	int                                            BatchSize = 0;
	std::unique_ptr<hailort::VDevice>              Device;
	std::shared_ptr<hailort::InferModel>           InferModel;
	std::shared_ptr<hailort::ConfiguredInferModel> ConfiguredInferModel;
	hailort::ConfiguredInferModel::Bindings        Bindings;

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
	NNModel*               Model;
	std::vector<OutTensor> OutTensors;
	hailort::AsyncInferJob HailoJob;
	BufferList             Buffers;

	OwnAsyncJobHandle(NNModel* model, std::vector<OutTensor>&& outTensors, hailort::AsyncInferJob&& hailoJob) {
		Model      = model;
		OutTensors = std::move(outTensors);
		HailoJob   = std::move(hailoJob);
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