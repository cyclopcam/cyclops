#pragma once

#include <string.h>
#include "debug.h"

typedef std::shared_ptr<HailoROI> HailoROIPtr;

HailoROIPtr MakeROI(const std::vector<OutTensor>& output_tensors, hailort::InferModel* infer_model) {
	HailoROIPtr roi = std::make_shared<HailoROI>(HailoROI(HailoBBox(0.0f, 0.0f, 1.0f, 1.0f)));

	for (auto const& t : output_tensors) {
		hailo_vstream_info_t info;

		strncpy(info.name, t.name.c_str(), sizeof(info.name));
		// To keep GCC quiet...
		info.name[HAILO_MAX_STREAM_NAME_SIZE - 1] = '\0';
		info.format                               = t.format;
		info.quant_info                           = t.quant_info;
		if (hailort::HailoRTCommon::is_nms(info))
			info.nms_shape = infer_model->outputs()[0].get_nms_shape().release();
		else
			info.shape = t.shape;

		auto outTensor = std::make_shared<HailoTensor>(t.data.get(), info);

		printf("Adding tensor %s to HailoROI\n", info.name);
		printf("  Shape: %s\n", DumpShape(outTensor->shape()).c_str());

		roi->add_tensor(outTensor);
	}

	return roi;
}
