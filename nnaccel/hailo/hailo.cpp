#include "defs.h"
#include "../nnaccel_prototype.h"

#include <hailo/hailort.h>
#include <hailo/hailort_common.hpp>
#include <hailo/vdevice.hpp>
#include <hailo/infer_model.hpp>
#include <chrono>
#include <string.h>

#include "internal.h"

void CopyImageToDenseBuffer(const void* image, int width, int height, int nchan, int stride, void* denseBuffer);

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

int nna_load_model(const char* modelDir, const char* modelName, const NNModelSetup* setup, void** model) {
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

	std::string fullpath;
	if (!modelDir || modelDir[0] == 0) {
		// absolute path specified by modelName
		fullpath = modelName;
	} else {
		// combine modelDir and modelName
		// eg
		//   modelDir = /var/lib/cyclops/models
		//   modelName = "yolov8s"
		//   fullpath = /var/lib/cyclops/models/hailo/8L/yolov8s.hef
		// So!
		// Here we add "hailo/8L" to the model directory. Only we can do this, because only we
		// know that we have an 8L accelerator. If we had support for others, then we'd have
		// more model directories, eg hailo/15
		fullpath = modelDir;
		if (fullpath.back() != '/') {
			fullpath += "/";
		}
		fullpath += "hailo/8L/";
		fullpath += modelName;
		fullpath += ".hef";
	}

	debug_printf("hailo nna_load_model fullpath = %s\n", fullpath.c_str());

	// Create infer model from HEF file.
	Expected<std::shared_ptr<InferModel>> infer_model_exp = vdevice->create_infer_model(fullpath.c_str());
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
	Expected<ConfiguredInferModel::Bindings> bindings_exp = configured_infer_model->create_bindings();
	if (!bindings_exp) {
		//LOG_ERROR("Failed to create infer bindings, status = " << bindings_exp.status());
		return _make_own_status(bindings_exp.status());
	}

	NNModel* m              = new NNModel();
	m->Device               = std::move(vdevice);
	m->BatchSize            = setup->BatchSize;
	m->InferModel           = infer_model;
	m->ConfiguredInferModel = configured_infer_model;
	m->Bindings             = std::move(bindings_exp.release());
	*model                  = m;

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

int nna_run_model(void* model, int batchSize, int width, int height, int nchan, int stride, const void* data, void** asyncHandle) {
	using namespace hailort;
	using namespace std::chrono_literals;

	if (!model) {
		return cySTATUS_MODEL_NOT_LOADED;
	}
	NNModel* m = (NNModel*) model;

	const std::string& input_name       = m->InferModel->get_input_names()[0];
	size_t             input_frame_size = m->InferModel->input(input_name)->get_frame_size();

	// Validate inputs
	if (stride == 0) {
		stride = width * nchan;
	}
	NNModelInfo info;
	nna_model_info(model, &info);
	if (batchSize != info.BatchSize || width != info.Width || height != info.Height || nchan != info.NChan) {
		return cySTATUS_INVALID_INPUT_DIMENSIONS;
	}
	if (batchSize * width * height * nchan != (int) input_frame_size) {
		return cySTATUS_INVALID_INPUT_DIMENSIONS;
	}

	BufferList buffers;

	uint8_t* denseInput = nullptr;
	if (stride != width * nchan) {
		denseInput = (uint8_t*) malloc(batchSize * width * height * nchan);
		if (!denseInput) {
			return cySTATUS_OUT_OF_CPU_MEMORY;
		}
		buffers.Add(denseInput);
		CopyImageToDenseBuffer(data, width, height, nchan, stride, denseInput);
	} else {
		denseInput = (uint8_t*) data;
	}

	auto status = m->Bindings.input(input_name)->set_buffer(MemoryView(denseInput, input_frame_size));
	if (status != HAILO_SUCCESS) {
		return _make_own_status(status);
	}

	//OwnAsyncJobHandle ownJob;
	std::vector<OutTensor> outputTensors;

	// Bind output tensors
	for (auto const& output_name : m->InferModel->get_output_names()) {
		size_t output_size = m->InferModel->output(output_name)->get_frame_size();

		uint8_t* outputBuffer = (uint8_t*) malloc(output_size);
		if (!outputBuffer) {
			//printf("Could not allocate an output buffer!");
			return cySTATUS_OUT_OF_CPU_MEMORY;
		}
		buffers.Add(outputBuffer);

		status = m->Bindings.output(output_name)->set_buffer(MemoryView(outputBuffer, output_size));
		if (status != HAILO_SUCCESS) {
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

	OwnAsyncJobHandle* myJob = new OwnAsyncJobHandle(m, std::move(outputTensors), std::move(job_exp.release()));
	myJob->Buffers           = std::move(buffers);
	*asyncHandle             = myJob;

	return cySTATUS_OK;
}

int nna_wait_for_job(void* job_handle, uint32_t max_wait_milliseconds) {
	OwnAsyncJobHandle* ownJob = (OwnAsyncJobHandle*) job_handle;
	hailo_status       status = ownJob->HailoJob.wait(std::chrono::milliseconds(max_wait_milliseconds));
	return _make_own_status(status);
}

int nna_get_object_detections(void* job_handle, size_t maxDetections, NNAObjectDetection** detections, size_t* numDetections) {
	*detections    = nullptr;
	*numDetections = 0;

	OwnAsyncJobHandle* ownJob                = (OwnAsyncJobHandle*) job_handle;
	uint32_t           max_wait_milliseconds = 0;
	hailo_status       status                = ownJob->HailoJob.wait(std::chrono::milliseconds(max_wait_milliseconds));
	if (status != HAILO_SUCCESS) {
		return cySTATUS_TIMEOUT;
	}
	NNModel* model = ownJob->Model;

	int nnWidth;
	int nnHeight;
	_model_input_sizes(model->InferModel.get(), nnWidth, nnHeight);

	bool nmsOnHailo = model->InferModel->outputs().size() == 1 && model->InferModel->outputs()[0].is_nms();

	if (nmsOnHailo) {
		OutTensor* out = &ownJob->OutTensors[0];

		const float* raw = (const float*) out->Data;

		//printf("Output shape: %d, %d\n", (int) out->shape.height, (int) out->shape.width);

		// The format is:
		// Number of boxes in that class (N), followed by the 5 box parameters, repeated N times
		size_t numClasses  = (size_t) out->Shape.height;
		size_t classIdx    = 0;
		size_t idx         = 0;
		size_t nDetections = 0;

		// Count the total number of boxes so that we can allocate the right size output buffer
		while (classIdx < numClasses) {
			size_t numBoxes = (size_t) raw[idx++];
			nDetections += numBoxes;
			idx += numBoxes * 5;
			classIdx++;
		}
		nDetections = std::min(nDetections, maxDetections);

		NNAObjectDetection* dets = (NNAObjectDetection*) malloc(nDetections * sizeof(NNAObjectDetection));

		classIdx    = 0;
		idx         = 0;
		size_t iDet = 0;
		while (classIdx < numClasses && iDet < nDetections) {
			size_t numBoxes = (size_t) raw[idx++];
			for (size_t i = 0; i < numBoxes; i++) {
				if (iDet >= nDetections) {
					break;
				}
				NNAObjectDetection det;
				float              ymin = raw[idx];
				float              xmin = raw[idx + 1];
				float              ymax = raw[idx + 2];
				float              xmax = raw[idx + 3];
				det.Confidence          = raw[idx + 4];
				det.ClassID             = (uint32_t) classIdx;
				det.X                   = xmin * nnWidth;
				det.Y                   = ymin * nnHeight;
				det.Width               = (xmax - xmin) * nnWidth;
				det.Height              = (ymax - ymin) * nnHeight;
				dets[iDet++]            = det;
				idx += 5;
			}
			classIdx++;
		}
		*numDetections = nDetections;
		*detections    = dets;
		//printf("FOOBAR!!!\n");
		//fflush(stdout);
		return cySTATUS_OK;
	} else {
		return cySTATUS_CPU_NMS_NOT_IMPLEMENTED;
	}
}

void nna_close_job(void* job_handle) {
	//printf("nna_close_job 1\n");
	//fflush(stdout);
	OwnAsyncJobHandle* ownJob = (OwnAsyncJobHandle*) job_handle;
	//printf("nna_close_job 2\n");
	//fflush(stdout);
	delete ownJob;
	//printf("nna_close_job 3\n");
	//fflush(stdout);
}
}

void CopyImageToDenseBuffer(const void* image, int width, int height, int nchan, int stride, void* denseBuffer) {
	const uint8_t* srcRow    = (const uint8_t*) image;
	uint8_t*       dstRow    = (uint8_t*) denseBuffer;
	int            inStride  = stride;
	int            outStride = width * nchan;
	for (int y = 0; y < height; y++) {
		memcpy(dstRow, srcRow, outStride);
		srcRow += inStride;
		dstRow += outStride;
	}
}