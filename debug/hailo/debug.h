#pragma once

#include <hailo/hailort.h>
#include <hailo/hailort_common.hpp>

#include "tsf.h"

std::string DumpShape(std::vector<size_t> shape) {
	std::string out;
	for (auto n : shape) {
		char el[100];
		sprintf(el, "%d,", (int) n);
		out += el;
	}
	return out;
}

std::string DumpShape(hailo_3d_image_shape_t shape) {
	return tsf::fmt("(height: %v, width: %v, features: %v)", shape.height, shape.width, shape.features);
}

std::string DumpFormat(hailo_format_t f) {
	return tsf::fmt("hailo_format = type: %v, order: %v, flags: %v", hailort::HailoRTCommon::get_format_type_str(f.type), hailort::HailoRTCommon::get_format_order_str(f.order), f.flags);
}

std::string DumpStream(const hailort::InferModel::InferStream& s) {
	return tsf::fmt("InferStream '%v' shape: %v, format: %v, frame_size: %v bytes", s.name(), DumpShape(s.shape()), DumpFormat(s.format()), s.get_frame_size());
}

// dump float32 as a 2d matrix
// stride is the number of float32 elements between rows.
// ncols is the number of columns that you want to print per line
// nrows is the number of rows that you want to print
std::string DumpFloat32(const float* out, int stride, int ncols, int nrows, float mul) {
	std::string result;
	for (int row = 0; row < nrows; row++) {
		int p = row * stride;
		for (int col = 0; col < ncols; col++) {
			result += tsf::fmt("%4.3f ", out[p + col] * mul);
		}
		result += "\n";
	}
	return result;
}

//void DumpOutputTensor(