#include "defs.h"
#include "../module_prototype.h"

#include <hailo/hailort.h>
#include <hailo/hailort_common.hpp>
#include <hailo/vdevice.hpp>
#include <hailo/infer_model.hpp>
#include <chrono>

cyStatus _make_own_status(hailo_status s) {
	return (cyStatus) (s + cySTATUS_HAILO_STATUS_OFFSET);
}

struct NNModel {
	int                                            BatchSize = 0;
	std::shared_ptr<hailort::InferModel>           InferModel;
	std::shared_ptr<hailort::ConfiguredInferModel> ConfiguredInferModel;
	hailort::ConfiguredInferModel::Bindings        Bindings;
};

void _model_input_sizes(hailort::InferModel* model, int& width, int& height) {
	width  = model->inputs()[0].shape().width;
	height = model->inputs()[0].shape().height;
}

extern "C" {

int nnm_load_model(const char* filename, const NNModelSetup* setup, void** model) {
	using namespace hailort;
	using namespace std::chrono_literals;

	////////////////////////////////////////////////////////////////////////////////////////////
	// Load/Init
	////////////////////////////////////////////////////////////////////////////////////////////

	Expected<std::unique_ptr<VDevice>> vdevice_exp = VDevice::create();
	if (!vdevice_exp) {
		return _make_own_status(vdevice_exp.status());
	}
	std::unique_ptr<hailort::VDevice> vdevice = vdevice_exp.release();

	// Create infer model from HEF file.
	Expected<std::shared_ptr<InferModel>> infer_model_exp = vdevice->create_infer_model(filename);
	if (!infer_model_exp) {
		return _make_own_status(infer_model_exp.status());
	}
	std::shared_ptr<hailort::InferModel> infer_model = infer_model_exp.release();
	// What's this for?
	infer_model->set_hw_latency_measurement_flags(HAILO_LATENCY_MEASURE);
	infer_model->set_batch_size(setup->BatchSize);

	// Configure the infer model
	//infer_model->output()->set_format_type(HAILO_FORMAT_TYPE_FLOAT32);
	Expected<ConfiguredInferModel> configured_infer_model_exp = infer_model->configure();
	if (!configured_infer_model_exp) {
		//LOG_ERROR("Failed to create configured infer model, status = " << configured_infer_model_exp.status());
		return _make_own_status(configured_infer_model_exp.status());
	}
	std::shared_ptr<hailort::ConfiguredInferModel> configured_infer_model = std::make_shared<ConfiguredInferModel>(configured_infer_model_exp.release());

	// Create infer bindings
	Expected<ConfiguredInferModel::Bindings> bindings_exp = configured_infer_model->create_bindings();
	if (!bindings_exp) {
		//LOG_ERROR("Failed to create infer bindings, status = " << bindings_exp.status());
		return _make_own_status(bindings_exp.status());
	}
	//hailort::ConfiguredInferModel::Bindings bindings = std::move(bindings_exp.release());

	NNModel* m              = new NNModel();
	m->BatchSize            = setup->BatchSize;
	m->InferModel           = infer_model;
	m->ConfiguredInferModel = configured_infer_model;
	m->Bindings             = std::move(bindings_exp.release());
	*model                  = m;

	return cySTATUS_OK;
}

void nnm_close_model(void* model) {
	NNModel* m = (NNModel*) model;
	delete m;
}

void nnm_model_info(void* model, NNModelInfo* info) {
	NNModel* m      = (NNModel*) model;
	info->BatchSize = m->BatchSize;
	info->NChan     = 3;
	_model_input_sizes(m->InferModel.get(), info->Width, info->Height);
}

const char* nnm_status_str(int _s) {
	cyStatus s = (cyStatus) _s;
	if (s >= cySTATUS_HAILO_STATUS_OFFSET) {
		return hailo_get_status_message(hailo_status(s - cySTATUS_HAILO_STATUS_OFFSET));
	}
	return _cyhailo_status_str_own(s);
}

int nnm_run_model(void* model, int batchSize, int width, int height, int nchan, const void* data, void** async_handle) {
	if (!model) {
		return cySTATUS_MODEL_NOT_LOADED;
	}
	NNModel* m = (NNModel*) model;

	// Validate inputs
	NNModelInfo info;
	nnm_model_info(model, &info);
	if (batchSize != info.BatchSize || width != info.Width || height != info.Height || nchan != info.NChan) {
		return cySTATUS_INVALID_INPUT_DIMENSIONS;
	}

	auto status = m->Bindings.input(input_name)->set_buffer(MemoryView((void*) (img_rgb_8), input_frame_size));
	if (status != HAILO_SUCCESS) {
		printf("Failed to set memory buffer: %d\n", (int) status);
		return status;
	}
}
}