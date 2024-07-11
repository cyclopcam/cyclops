#include "defs.h"
#include "../nnaccel_prototype.h"

#include <hailo/hailort.h>
#include <hailo/hailort_common.hpp>
#include <hailo/vdevice.hpp>
#include <hailo/infer_model.hpp>
#include <chrono>

#include "internal.h"

//struct NNModel {
//	int                                            BatchSize = 0;
//	std::shared_ptr<hailort::InferModel>           InferModel;
//	std::shared_ptr<hailort::ConfiguredInferModel> ConfiguredInferModel;
//	hailort::ConfiguredInferModel::Bindings        Bindings;
//};

void _model_input_sizes(hailort::InferModel* model, int& width, int& height) {
	width  = model->inputs()[0].shape().width;
	height = model->inputs()[0].shape().height;
}

#define debug_printf printf

// noop
//#define debug_printf (void)

NNModel::~NNModel() {
	//debug_printf("~NNModel 1.a\n");
	ConfiguredInferModel->shutdown();
	//debug_printf("~NNModel 1.b\n");
	ConfiguredInferModel = nullptr;
	//debug_printf("~NNModel 2\n");
	InferModel = nullptr;
	//debug_printf("~NNModel 3\n");
}

extern "C" {

int nna_load_model(const char* filename, const NNModelSetup* setup, void** model) {
	using namespace hailort;
	using namespace std::chrono_literals;

	debug_printf("hailo nna_load_model 1\n");

	////////////////////////////////////////////////////////////////////////////////////////////
	// Load/Init
	////////////////////////////////////////////////////////////////////////////////////////////

	Expected<std::unique_ptr<VDevice>> vdevice_exp = VDevice::create();
	if (!vdevice_exp) {
		return _make_own_status(vdevice_exp.status());
	}
	std::unique_ptr<hailort::VDevice> vdevice = vdevice_exp.release();

	debug_printf("hailo nna_load_model 2\n");

	// Create infer model from HEF file.
	Expected<std::shared_ptr<InferModel>> infer_model_exp = vdevice->create_infer_model(filename);
	if (!infer_model_exp) {
		return _make_own_status(infer_model_exp.status());
	}
	std::shared_ptr<hailort::InferModel> infer_model = infer_model_exp.release();
	// What's this for?
	infer_model->set_hw_latency_measurement_flags(HAILO_LATENCY_MEASURE);
	infer_model->set_batch_size(setup->BatchSize);

	debug_printf("hailo nna_load_model 3\n");

	// Configure the infer model
	//infer_model->output()->set_format_type(HAILO_FORMAT_TYPE_FLOAT32);
	Expected<ConfiguredInferModel> configured_infer_model_exp = infer_model->configure();
	if (!configured_infer_model_exp) {
		//LOG_ERROR("Failed to create configured infer model, status = " << configured_infer_model_exp.status());
		return _make_own_status(configured_infer_model_exp.status());
	}
	std::shared_ptr<hailort::ConfiguredInferModel> configured_infer_model = std::make_shared<ConfiguredInferModel>(configured_infer_model_exp.release());

	debug_printf("hailo nna_load_model 4\n");

	//configured_infer_model = nullptr;

	// Create infer bindings
	//Expected<ConfiguredInferModel::Bindings> bindings_exp = configured_infer_model->create_bindings();
	//if (!bindings_exp) {
	//	//LOG_ERROR("Failed to create infer bindings, status = " << bindings_exp.status());
	//	return _make_own_status(bindings_exp.status());
	//}

	NNModel* m              = new NNModel();
	m->Device               = std::move(vdevice);
	m->BatchSize            = setup->BatchSize;
	m->InferModel           = infer_model;
	m->ConfiguredInferModel = configured_infer_model;
	//m->Bindings             = std::move(bindings_exp.release());
	*model = m;

	//debug_printf("Users of configured_infer_model: %d\n", (int) configured_infer_model.use_count());

	debug_printf("hailo nna_load_model 5\n");

	return cySTATUS_OK;
}

void nna_close_model(void* model) {
	debug_printf("nna_close_model 1\n");
	NNModel* m = (NNModel*) model;

	//debug_printf("Users of configured_infer_model: %d\n", (int) m->ConfiguredInferModel.use_count());

	delete m;
	debug_printf("nna_close_model 2\n");
}

void nna_model_info(void* model, NNModelInfo* info) {
	NNModel* m      = (NNModel*) model;
	info->BatchSize = m->BatchSize;
	info->NChan     = 3;
	_model_input_sizes(m->InferModel.get(), info->Width, info->Height);
}

const char* nna_status_str(int _s) {
	cyStatus s = (cyStatus) _s;
	if (s >= cySTATUS_HAILO_STATUS_OFFSET) {
		return hailo_get_status_message(hailo_status(s - cySTATUS_HAILO_STATUS_OFFSET));
	}
	return _cyhailo_status_str_own(s);
}

int nna_run_model(void* model, int batchSize, int width, int height, int nchan, const void* data, void** asyncHandle) {
	using namespace hailort;
	using namespace std::chrono_literals;

	if (!model) {
		return cySTATUS_MODEL_NOT_LOADED;
	}
	NNModel* m = (NNModel*) model;

	const std::string& input_name       = m->InferModel->get_input_names()[0];
	size_t             input_frame_size = m->InferModel->input(input_name)->get_frame_size();

	// Validate inputs
	NNModelInfo info;
	nna_model_info(model, &info);
	if (batchSize != info.BatchSize || width != info.Width || height != info.Height || nchan != info.NChan) {
		return cySTATUS_INVALID_INPUT_DIMENSIONS;
	}
	if (batchSize * width * height * nchan != (int) input_frame_size) {
		return cySTATUS_INVALID_INPUT_DIMENSIONS;
	}

	auto status = m->Bindings.input(input_name)->set_buffer(MemoryView((void*) data, input_frame_size));
	if (status != HAILO_SUCCESS) {
		return _make_own_status(status);
	}

	//OwnAsyncJobHandle ownJob;
	std::vector<OutTensor> outputTensors;

	// Bind output tensors
	for (auto const& output_name : m->InferModel->get_output_names()) {
		size_t output_size = m->InferModel->output(output_name)->get_frame_size();

		//std::shared_ptr<uint8_t> output_buffer = allocator.Allocate(output_size);
		uint8_t* outputBuffer = (uint8_t*) malloc(output_size);
		if (!outputBuffer) {
			//printf("Could not allocate an output buffer!");
			return cySTATUS_OUT_OF_CPU_MEMORY;
		}

		status = m->Bindings.output(output_name)->set_buffer(MemoryView(outputBuffer, output_size));
		if (status != HAILO_SUCCESS) {
			free(outputBuffer);
			//printf("Failed to set infer output buffer, status = %d", (int) status);
			return _make_own_status(status);
		}

		std::vector<hailo_quant_info_t> quant  = m->InferModel->output(output_name)->get_quant_infos();
		hailo_3d_image_shape_t          shape  = m->InferModel->output(output_name)->shape();
		hailo_format_t                  format = m->InferModel->output(output_name)->format();
		outputTensors.push_back(OutTensor(outputBuffer, output_name, quant[0], shape, format));

		//printf("Output tensor %s, %d bytes, shape (%d, %d, %d)\n", output_name.c_str(), (int) output_size, (int) shape.height, (int) shape.width, (int) shape.features);
		// printf("  %s\n", DumpFormat(format).c_str());
		//for (auto q : quant) {
		//	printf("  Quantization scale: %f offset: %f\n", q.qp_scale, q.qp_zp);
		//}
	}

	// Prepare tensors for postprocessing.
	// This is from the original SDK/demos, but I don't understand why this sorting step is necessary.
	// It's quite obviously NOT necessary when there's only one output, which is the case with
	// YOLOv8 object detection on Rpi5+Hailo8L.
	std::sort(outputTensors.begin(), outputTensors.end(), OutTensor::SortFunction);

	// Waiting for available requests in the pipeline.
	status = m->ConfiguredInferModel->wait_for_async_ready(2s);
	if (status != HAILO_SUCCESS) {
		//printf("Failed to wait for async ready, status = %d", (int) status);
		return _make_own_status(status);
	}

	// Dispatch the job.
	Expected<AsyncInferJob> job_exp = m->ConfiguredInferModel->run_async(m->Bindings);
	if (!job_exp) {
		//printf("Failed to start async infer job, status = %d\n", (int) job_exp.status());
		return _make_own_status(job_exp.status());
	}
	//hailort::AsyncInferJob* job = new AsyncInferJob(job_exp.release());

	// Detaches the job. Without detaching, the job's destructor will block until the job finishes.
	// Hmmm, but what if somebody wants to abandon an inference job. We can't delete the memory
	// until the job finishes, so we actually want the destructor to wait.
	// Our destructor is supposed to run on nna_finish_run().
	//job->detach();

	*asyncHandle = new OwnAsyncJobHandle(m, std::move(outputTensors), std::move(job_exp.release()));

	return cySTATUS_OK;
}

int nna_wait_for_job(void* async_handle, uint32_t max_wait_milliseconds) {
	OwnAsyncJobHandle* ownJob = (OwnAsyncJobHandle*) async_handle;
	hailo_status       status = ownJob->HailoJob.wait(std::chrono::milliseconds(max_wait_milliseconds));
	return _make_own_status(status);
}

int nna_get_object_detections(void* async_handle, uint32_t max_wait_milliseconds, int maxDetections, NNMObjectDetection* detections, int* numDetections) {
	OwnAsyncJobHandle* ownJob = (OwnAsyncJobHandle*) async_handle;
	hailo_status       status = ownJob->HailoJob.wait(std::chrono::milliseconds(max_wait_milliseconds));
	if (status != HAILO_SUCCESS) {
		return _make_own_status(status);
	}
	NNModel* model = ownJob->Model;

	bool nmsOnHailo = model->InferModel->outputs().size() == 1 && model->InferModel->outputs()[0].is_nms();
	int  response   = cySTATUS_OK;
	*numDetections  = 0;

	if (nmsOnHailo) {
		OutTensor* out = &ownJob->OutTensors[0];

		const float* raw = (const float*) out->Data;

		//printf("Output shape: %d, %d\n", (int) out->shape.height, (int) out->shape.width);

		// The format is:
		// Number of boxes in that class (N), followed by the 5 box parameters, repeated N times
		size_t numClasses  = (size_t) out->Shape.height;
		size_t classIdx    = 0;
		size_t idx         = 0;
		int    nDetections = 0;
		while (classIdx < numClasses) {
			size_t numBoxes = (size_t) raw[idx++];
			for (size_t i = 0; i < numBoxes; i++) {
				float ymin                         = raw[idx];
				float xmin                         = raw[idx + 1];
				float ymax                         = raw[idx + 2];
				float xmax                         = raw[idx + 3];
				float confidence                   = raw[idx + 4];
				detections[nDetections].ClassID    = (uint32_t) classIdx;
				detections[nDetections].Confidence = confidence;
				detections[nDetections].X          = xmin;
				detections[nDetections].Y          = ymin;
				detections[nDetections].W          = xmax - xmin;
				detections[nDetections].H          = ymax - ymin;
				idx += 5;
				nDetections++;
				if (nDetections >= maxDetections) {
					break;
				}
			}
			if (nDetections >= maxDetections) {
				break;
			}
			classIdx++;
		}
		*numDetections = nDetections;
	} else {
		response = cySTATUS_CPU_NMS_NOT_IMPLEMENTED;
	}

	return response;
}

void nna_finish_run(void* async_handle) {
	OwnAsyncJobHandle* ownJob = (OwnAsyncJobHandle*) async_handle;
	delete ownJob;
}
}