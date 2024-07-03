#include <hailo/hailort.h>
#include <hailo/hailort_common.hpp>
#include <hailo/vdevice.hpp>
#include <hailo/infer_model.hpp>
#include <chrono>

#include "allocator.h"
#include "output_tensor.h"
#include "common/hailo_objects.hpp"
#include "misc.h"
#include "yolov8_postprocess.hpp"

#define STB_IMAGE_IMPLEMENTATION
#include "stb_image.h"

// Release
// g++ -o yolohailo yolov8.cpp allocator.cpp -lhailort && ./yolohailo
// Debug
// g++ -g -O0 -o yolohailo yolov8.cpp allocator.cpp yolov8_postprocess.cpp -lhailort && ./yolohailo

std::string hefFile = "/home/ben/yolov8s.hef";

int run() {
	//hailo_create_vdevice(nullptr, nullptr);

	using namespace hailort;
	using namespace std::chrono_literals;

	////////////////////////////////////////////////////////////////////////////////////////////
	// Load/Init
	////////////////////////////////////////////////////////////////////////////////////////////

	Expected<std::unique_ptr<VDevice>> vdevice_exp = VDevice::create();
	if (!vdevice_exp) {
		//LOG_ERROR("Failed create vdevice, status = " << vdevice_exp.status());
		printf("Failed create vdevice\n");
		return vdevice_exp.status();
	}
	std::unique_ptr<hailort::VDevice> vdevice = vdevice_exp.release();

	// Create infer model from HEF file.
	Expected<std::shared_ptr<InferModel>> infer_model_exp = vdevice->create_infer_model(hefFile);
	if (!infer_model_exp) {
		//LOG_ERROR("Failed to create infer model, status = " << infer_model_exp.status());
		return infer_model_exp.status();
	}
	std::shared_ptr<hailort::InferModel> infer_model = infer_model_exp.release();
	infer_model->set_hw_latency_measurement_flags(HAILO_LATENCY_MEASURE);

	printf("infer_model N inputs: %d\n", (int) infer_model->inputs().size());
	printf("infer_model N outputs: %d\n", (int) infer_model->outputs().size());
	printf("infer_model inputstream[0]: %s\n", DumpStream(infer_model->inputs()[0]).c_str());
	printf("infer_model outputstream[0]: %s\n", DumpStream(infer_model->outputs()[0]).c_str());

	// Configure the infer model
	//infer_model->output()->set_format_type(HAILO_FORMAT_TYPE_FLOAT32);
	Expected<ConfiguredInferModel> configured_infer_model_exp = infer_model->configure();
	if (!configured_infer_model_exp) {
		//LOG_ERROR("Failed to create configured infer model, status = " << configured_infer_model_exp.status());
		return configured_infer_model_exp.status();
	}
	std::shared_ptr<hailort::ConfiguredInferModel> configured_infer_model = std::make_shared<ConfiguredInferModel>(configured_infer_model_exp.release());

	// Create infer bindings
	Expected<ConfiguredInferModel::Bindings> bindings_exp = configured_infer_model->create_bindings();
	if (!bindings_exp) {
		//LOG_ERROR("Failed to create infer bindings, status = " << bindings_exp.status());
		return bindings_exp.status();
	}
	hailort::ConfiguredInferModel::Bindings bindings = std::move(bindings_exp.release());

	////////////////////////////////////////////////////////////////////////////////////////////
	// Run
	////////////////////////////////////////////////////////////////////////////////////////////

	const std::string& input_name       = infer_model->get_input_names()[0];
	size_t             input_frame_size = infer_model->input(input_name)->get_frame_size();
	printf("input_name: %s\n", input_name.c_str());
	printf("input_frame_size: %d\n", (int) input_frame_size); // eg 640x640x3 = 1228800

	const char*    img_filename = "../../testdata/yard-640x640.jpg";
	int            width = 0, height = 0, nchan = 0;
	unsigned char* img_rgb_8 = stbi_load(img_filename, &width, &height, &nchan, 3);
	if (!img_rgb_8) {
		printf("Failed to load image %s\n", img_filename);
		return 1;
	}
	if (width * height * nchan != input_frame_size) {
		printf("Imput image resolution %d not equal to NN input size %d", int(width * height * nchan), (int) input_frame_size);
	}

	auto status = bindings.input(input_name)->set_buffer(MemoryView((void*) (img_rgb_8), input_frame_size));
	if (status != HAILO_SUCCESS) {
		//LOG_ERROR("Could not write to input stream with status " << status);
		printf("Failed to set memory buffer: %d\n", (int) status);
		return status;
	}

	Allocator              allocator;
	std::vector<OutTensor> output_tensors;

	// Output tensors.
	for (auto const& output_name : infer_model->get_output_names()) {
		size_t output_size = infer_model->output(output_name)->get_frame_size();

		std::shared_ptr<uint8_t> output_buffer = allocator.Allocate(output_size);
		if (!output_buffer) {
			printf("Could not allocate an output buffer!");
			return status;
		}

		status = bindings.output(output_name)->set_buffer(MemoryView(output_buffer.get(), output_size));
		if (status != HAILO_SUCCESS) {
			printf("Failed to set infer output buffer, status = %d", (int) status);
			return status;
		}

		const std::vector<hailo_quant_info_t> quant  = infer_model->output(output_name)->get_quant_infos();
		const hailo_3d_image_shape_t          shape  = infer_model->output(output_name)->shape();
		const hailo_format_t                  format = infer_model->output(output_name)->format();
		output_tensors.emplace_back(std::move(output_buffer), output_name, quant[0], shape, format);

		printf("Output tensor %s, %d bytes, shape (%d, %d, %d)\n", output_name.c_str(), (int) output_size, (int) shape.height, (int) shape.width, (int) shape.features);
		printf("  %s\n", DumpFormat(format).c_str());
		for (auto q : quant) {
			printf("  Quantization scale: %f offset: %f\n", q.qp_scale, q.qp_zp);
		}
	}

	// Waiting for available requests in the pipeline.
	status = configured_infer_model->wait_for_async_ready(1s);
	if (status != HAILO_SUCCESS) {
		printf("Failed to wait for async ready, status = %d", (int) status);
		return status;
	}

	//	std::chrono::time_point<std::chrono::steady_clock> last_frame = std::chrono::steady_clock::now();
	//
	//	Expected<LatencyMeasurementResult>                 inf_time_exp = configured_infer_model->get_hw_latency_measurement();
	//	std::chrono::time_point<std::chrono::steady_clock> this_frame   = std::chrono::steady_clock::now();
	//
	//	if (inf_time_exp && last_frame.time_since_epoch() != 0s) {
	//		const auto inf_time   = std::chrono::duration_cast<std::chrono::milliseconds>(inf_time_exp.release().avg_hw_latency);
	//		const auto frame_time = std::chrono::duration_cast<std::chrono::milliseconds>(this_frame - last_frame);
	//
	//		//if (frame_time < inf_time)
	//		//	LOG(2, "Warning: model inferencing time of " << inf_time.count() << "ms " << "> current job interval of " << frame_time.count() << "ms!");
	//	}
	//last_frame = this_frame;

	// Dispatch the job.
	Expected<AsyncInferJob> job_exp = configured_infer_model->run_async(bindings);
	if (!job_exp) {
		printf("Failed to start async infer job, status = %d\n", (int) job_exp.status());
		return status;
	}
	hailort::AsyncInferJob job = job_exp.release();

	// Detach and let the job run.
	job.detach();

	// Usually we'd go off and do something else at this point.

	// Prepare tensors for postprocessing.
	std::sort(output_tensors.begin(), output_tensors.end(), OutTensor::SortFunction);

	// Wait for job completion.
	status = job.wait(1s);
	if (status != HAILO_SUCCESS) {
		printf("Failed to wait for inference to finish, status = %d\n", (int) status);
		return status;
	}

	HailoROIPtr roi = MakeROI(output_tensors, infer_model.get());

	bool nms_on_hailo = false;
	if (infer_model->outputs().size() == 1 && infer_model->outputs()[0].is_nms()) {
		printf("NMS on hailo\n");
		nms_on_hailo = true;
	} else {
		printf("NMS on CPU\n");
	}

	/*
	using PostProcFuncPtr    = void (*)(HailoROIPtr, YoloParams*);
	using PostProcFuncPtrNms = void (*)(HailoROIPtr);

	if (nms_on_hailo) {
		PostProcFuncPtrNms filter = reinterpret_cast<PostProcFuncPtrNms>(postproc_nms_.GetSymbol("filter"));
		if (!filter)
			return {};

		filter(roi);
	} else {
		PostProcFuncPtr filter = reinterpret_cast<PostProcFuncPtr>(postproc_.GetSymbol("filter"));
		if (!filter)
			return {};

		filter(roi, yolo_params_);
	}
	*/

	// Try to figure out the data format of the output
	{
		std::vector<HailoTensorPtr> tensors = roi->get_tensors();
		const float*                raw     = (const float*) tensors[0]->data();
		// our buffer is 40080 floats long
		// 40080 / 5 = 8016. 5 is often a magic object detection output number, because of X,Y,W,H,C
		//printf("%s\n", (DumpFloat32(raw, 30, 30, 100).c_str()));

		//printf("%s\n", (DumpFloat32(raw, 5, 5, 10, 640).c_str()));
		//for (int i = 0; i < 40080; i++) {
		//	if (raw[i] != 0)
		//		printf("%d %.3f\n", i, raw[i]);
		//}
		printf("%d, %d\n", (int) tensors[0]->height(), (int) tensors[0]->width());

		float nnWidth  = 640;
		float nnHeight = 640;

		// Success - a reply post on the Hailo forums told us the formula:
		size_t numClasses = (size_t) tensors[0]->height();
		size_t classIdx   = 0;
		size_t idx        = 0;
		while (classIdx < numClasses) {
			size_t numBoxes = (size_t) raw[idx++];
			for (size_t i = 0; i < numBoxes; i++) {
				float ymin       = raw[idx];
				float xmin       = raw[idx + 1];
				float ymax       = raw[idx + 2];
				float xmax       = raw[idx + 3];
				float confidence = raw[idx + 4];
				if (confidence >= 0.5f) {
					printf("class: %d, confidence: %.2f, %.0f,%.0f - %.0f,%.0f\n", classIdx, confidence, xmin * nnWidth, ymin * nnHeight, xmax * nnWidth, ymax * nnHeight);
				}
				idx += 5;
			}
			classIdx++;
		}
	}

	//yolov8_postprocess_1(roi);

	return 12345;
}

int main(int argc, char** argv) {
	int status = run();
	printf("status: %d\n", status);
	return 0;
}