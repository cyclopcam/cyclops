#include "layer.h"
#include "net.h"
#include "simpleocv.h"

#include "sharedHeader.h"
#include "yolo.h"
#include "ncnn_helpers.h"

#include "stb_image_write.h"

//#if defined(USE_NCNN_SIMPLEOCV)
//#include "simpleocv.h"
//#else
//#include <opencv2/core/core.hpp>
//#include <opencv2/highgui/highgui.hpp>
//#include <opencv2/imgproc/imgproc.hpp>
//#endif

#include <float.h>
#include <stdio.h>
#include <vector>

#define MODEL_STRIDE 32

struct Object {
	cv::Rect_<float> rect;
	int              label;
	float            prob;
	float            probMargin; // Only outputted for YOLOv8 and YOLOv11
};

static inline float clamp(float v, float vmin, float vmax) {
	if (v < vmin)
		return vmin;
	if (v > vmax)
		return vmax;
	return v;
}

static inline float intersection_area(const Object& a, const Object& b) {
	cv::Rect_<float> inter = a.rect & b.rect;
	return inter.area();
}

static void qsort_descent_inplace(std::vector<Object>& objects, int left, int right) {
	int   i = left;
	int   j = right;
	float p = objects[(left + right) / 2].prob;

	while (i <= j) {
		while (objects[i].prob > p)
			i++;

		while (objects[j].prob < p)
			j--;

		if (i <= j) {
			// swap
			std::swap(objects[i], objects[j]);

			i++;
			j--;
		}
	}

	//#pragma omp parallel sections
	{
		//#pragma omp section
		{
			if (left < j)
				qsort_descent_inplace(objects, left, j);
		}
		//#pragma omp section
		{
			if (i < right)
				qsort_descent_inplace(objects, i, right);
		}
	}
}

static void qsort_descent_inplace(std::vector<Object>& objects) {
	if (objects.empty())
		return;

	qsort_descent_inplace(objects, 0, objects.size() - 1);
}

static void nms_sorted_bboxes(const std::vector<Object>& objects, std::vector<int>& picked, float nms_threshold, bool agnostic = false) {
	picked.clear();

	const int n = objects.size();

	std::vector<float> areas(n);
	for (int i = 0; i < n; i++) {
		areas[i] = objects[i].rect.area();
	}

	for (int i = 0; i < n; i++) {
		const Object& a = objects[i];

		int keep = 1;
		for (int j = 0; j < (int) picked.size(); j++) {
			const Object& b = objects[picked[j]];

			if (!agnostic && a.label != b.label)
				continue;

			// intersection over union
			float inter_area = intersection_area(a, b);
			float union_area = areas[i] + areas[picked[j]] - inter_area;
			// float IoU = inter_area / union_area
			if (inter_area / union_area > nms_threshold)
				keep = 0;
		}

		if (keep)
			picked.push_back(i);
	}
}

static inline float sigmoid(float x) {
	return static_cast<float>(1.f / (1.f + exp(-x)));
}

static void generate_proposals(const ncnn::Mat& anchors, int stride, const ncnn::Mat& in_pad, const ncnn::Mat& feat_blob, float prob_threshold, std::vector<Object>& objects) {
	const int num_grid = feat_blob.h;

	int num_grid_x;
	int num_grid_y;
	if (in_pad.w > in_pad.h) {
		num_grid_x = in_pad.w / stride;
		num_grid_y = num_grid / num_grid_x;
	} else {
		num_grid_y = in_pad.h / stride;
		num_grid_x = num_grid / num_grid_y;
	}

	const int num_class = feat_blob.w - 5;

	const int num_anchors = anchors.w / 2;

	for (int q = 0; q < num_anchors; q++) {
		const float anchor_w = anchors[q * 2];
		const float anchor_h = anchors[q * 2 + 1];

		const ncnn::Mat feat = feat_blob.channel(q);

		for (int i = 0; i < num_grid_y; i++) {
			for (int j = 0; j < num_grid_x; j++) {
				const float* featptr        = feat.row(i * num_grid_x + j);
				float        box_confidence = sigmoid(featptr[4]);
				if (box_confidence >= prob_threshold) {
					// find class index with max class score
					int   class_index = 0;
					float class_score = -FLT_MAX;
					for (int k = 0; k < num_class; k++) {
						float score = featptr[5 + k];
						if (score > class_score) {
							class_index = k;
							class_score = score;
						}
					}
					float confidence = box_confidence * sigmoid(class_score);
					if (confidence >= prob_threshold) {
						float dx = sigmoid(featptr[0]);
						float dy = sigmoid(featptr[1]);
						float dw = sigmoid(featptr[2]);
						float dh = sigmoid(featptr[3]);

						float pb_cx = (dx * 2.f - 0.5f + j) * stride;
						float pb_cy = (dy * 2.f - 0.5f + i) * stride;

						float pb_w = pow(dw * 2.f, 2) * anchor_w;
						float pb_h = pow(dh * 2.f, 2) * anchor_h;

						float x0 = pb_cx - pb_w * 0.5f;
						float y0 = pb_cy - pb_h * 0.5f;
						float x1 = pb_cx + pb_w * 0.5f;
						float y1 = pb_cy + pb_h * 0.5f;

						Object obj;
						obj.rect.x      = x0;
						obj.rect.y      = y0;
						obj.rect.width  = x1 - x0;
						obj.rect.height = y1 - y0;
						obj.label       = class_index;
						obj.prob        = confidence;

						objects.push_back(obj);
					}
				}
			}
		}
	}
}

static void detect_yolov7_8(ModelTypes modelType, ncnn::Net& net, int nn_width, int nn_height, int detectFlags, float prob_threshold, float nms_threshold, const cv::Mat& in_img, std::vector<Object>& objects) {
	// yolov7.opt.use_vulkan_compute = true;
	// yolov7.opt.use_bf16_storage = true;

	// original pretrained model from https://github.com/WongKinYiu/yolov7
	// the ncnn model https://github.com/nihui/ncnn-assets/tree/master/models
	//yolov7.load_param("yolov7-tiny.param");
	//yolov7.load_model("yolov7-tiny.bin");

	//const int   target_size    = 640;
	//const float prob_threshold = 0.25f;
	//const float nms_threshold  = 0.45f;

	// It's ultra useful to dump the image right before inference, to verify that you haven't screwed something up along the way
	//stbi_write_jpg("/home/ben/dev/cyclops/inference.jpg", in_img.cols, in_img.rows, in_img.channels(), in_img.data, 95);

	int img_w = in_img.cols;
	int img_h = in_img.rows;

	/*
	// letterbox pad to multiple of MODEL_STRIDE
	float img_aspect = img_w / (float) img_h;
	float nn_aspect  = nn_width / (float) nn_height;
	int   w          = img_w;
	int   h          = img_h;
	float scale      = 1.f;
	if (nn_aspect < img_aspect) {
		scale = (float) nn_width / w;
		w     = nn_width;
		h     = h * scale;
	} else {
		scale = (float) nn_height / h;
		h     = nn_height;
		w     = w * scale;
	}

	// It's wasteful to make a copy of our data if it's already RGB.
	// It doesn't look like ncnn::Mat has a way of referencing existing data without re-allocating it.
	//ncnn::Mat in = ncnn::Mat::from_pixels_resize(bgr.data, ncnn::Mat::PIXEL_BGR2RGB, img_w, img_h, w, h);
	ncnn::Mat in;
	if (in_img.channels() == 1)
		in = ncnn::Mat::from_pixels_resize(in_img.data, ncnn::Mat::PIXEL_GRAY2RGB, img_w, img_h, w, h);
	else if (in_img.channels() == 3)
		in = ncnn::Mat::from_pixels_resize(in_img.data, ncnn::Mat::PIXEL_RGB, img_w, img_h, w, h);
	else if (in_img.channels() == 4)
		in = ncnn::Mat::from_pixels_resize(in_img.data, ncnn::Mat::PIXEL_RGBA2RGB, img_w, img_h, w, h);
	else
		return;

	int       wpad = (w + MODEL_STRIDE - 1) / MODEL_STRIDE * MODEL_STRIDE - w;
	int       hpad = (h + MODEL_STRIDE - 1) / MODEL_STRIDE * MODEL_STRIDE - h;
	ncnn::Mat in_pad;
	ncnn::copy_make_border(in, in_pad, hpad / 2, hpad - hpad / 2, wpad / 2, wpad - wpad / 2, ncnn::BORDER_CONSTANT, 114.f);
	*/
	int   resized_w = img_w;
	int   resized_h = img_h;
	float scale     = 1.f;
	if (img_w > nn_width || img_h > nn_height) {
		scale     = std::min((float) nn_width / img_w, (float) nn_height / img_h);
		resized_w = int(img_w * scale);
		resized_h = int(img_h * scale);
	}

	float img_aspect = img_w / (float) img_h;
	float nn_aspect  = nn_width / (float) nn_height;

	// It's wasteful to make a copy of our data if it's already RGB.
	// It doesn't look like ncnn::Mat has a way of referencing existing data without re-allocating it.
	// Also: from_pixels_resize will perform no scaling if the input size matches the output size,
	// and this is our expected usual case (i.e. we ship an NN that is just big enough to fit most
	// of the low res camera streams - typically 320 x 240).
	//ncnn::Mat in = ncnn::Mat::from_pixels_resize(bgr.data, ncnn::Mat::PIXEL_BGR2RGB, img_w, img_h, w, h);
	// UPDATE: ncnn::Mat here is f32, so it is necessary to make a copy.
	ncnn::Mat in;
	if (in_img.channels() == 1)
		in = ncnn::Mat::from_pixels_resize(in_img.data, ncnn::Mat::PIXEL_GRAY2RGB, img_w, img_h, resized_w, resized_h);
	else if (in_img.channels() == 3)
		in = ncnn::Mat::from_pixels_resize(in_img.data, ncnn::Mat::PIXEL_RGB, img_w, img_h, resized_w, resized_h);
	else if (in_img.channels() == 4)
		in = ncnn::Mat::from_pixels_resize(in_img.data, ncnn::Mat::PIXEL_RGBA2RGB, img_w, img_h, resized_w, resized_h);
	else
		return;

	// "in" is f32
	//{
	//	cv::Mat tmp(in.h, in.w, CV_8UC3);
	//	in.to_pixels(tmp.data, ncnn::Mat::PIXEL_RGB);
	//	cv::imwrite("in.jpg", tmp);
	//}

	int       wpad = nn_width - resized_w;
	int       hpad = nn_height - resized_h;
	ncnn::Mat in_pad;
	ncnn::copy_make_border(in, in_pad, hpad / 2, hpad - hpad / 2, wpad / 2, wpad - wpad / 2, ncnn::BORDER_CONSTANT, 114.f);

	//{
	//	cv::Mat tmp(in_pad.h, in_pad.w, CV_8UC3);
	//	in_pad.to_pixels(tmp.data, ncnn::Mat::PIXEL_RGB);
	//	cv::imwrite("in_pad.jpg", tmp);
	//}

	const float norm_vals[3] = {1 / 255.f, 1 / 255.f, 1 / 255.f};
	in_pad.substract_mean_normalize(0, norm_vals);

	ncnn::Extractor ex = net.create_extractor();

	switch (modelType) {
	case ModelTypes::YOLOv7: ex.input("images", in_pad); break;
	case ModelTypes::YOLOv8:
	case ModelTypes::YOLO11: ex.input("in0", in_pad); break;
	}

	std::vector<Object> proposals;
	//float               maxPerson = 0;

	if (modelType == ModelTypes::YOLOv7) {
		// stride 8
		{
			ncnn::Mat out;
			ex.extract("output", out);

			ncnn::Mat anchors(6);
			anchors[0] = 12.f;
			anchors[1] = 16.f;
			anchors[2] = 19.f;
			anchors[3] = 36.f;
			anchors[4] = 40.f;
			anchors[5] = 28.f;

			std::vector<Object> objects8;
			generate_proposals(anchors, 8, in_pad, out, prob_threshold, objects8);

			proposals.insert(proposals.end(), objects8.begin(), objects8.end());
		}

		// stride 16
		{
			ncnn::Mat out;

			ex.extract("288", out);

			ncnn::Mat anchors(6);
			anchors[0] = 36.f;
			anchors[1] = 75.f;
			anchors[2] = 76.f;
			anchors[3] = 55.f;
			anchors[4] = 72.f;
			anchors[5] = 146.f;

			std::vector<Object> objects16;
			generate_proposals(anchors, 16, in_pad, out, prob_threshold, objects16);

			proposals.insert(proposals.end(), objects16.begin(), objects16.end());
		}

		// stride 32
		{
			ncnn::Mat out;

			ex.extract("302", out);

			ncnn::Mat anchors(6);
			anchors[0] = 142.f;
			anchors[1] = 110.f;
			anchors[2] = 192.f;
			anchors[3] = 243.f;
			anchors[4] = 459.f;
			anchors[5] = 401.f;

			std::vector<Object> objects32;
			generate_proposals(anchors, 32, in_pad, out, prob_threshold, objects32);

			proposals.insert(proposals.end(), objects32.begin(), objects32.end());
		}
	} else if (modelType == ModelTypes::YOLOv8 || modelType == ModelTypes::YOLO11) {
		ncnn::Mat outRaw;
		ex.extract("out0", outRaw);
		//ncnn::Mat shape = out.shape();
		//printf("output whdc: %d %d %d %d (dims %d)\n", out.w, out.h, out.d, out.c, out.dims);
		//printf("output shape whdc: %d %d %d %d (dims %d)\n", shape.w, shape.h, shape.d, shape.c, shape.dims);
		// Example: 1680 84 1 1
		// 80 is the number of classes, so the other 4 must be the bounding box.
		// If the number was 85, then the first number would be the confidence.
		// But since its 84, the confidence is taken as the max of the 80 classes.
		// This is indeed correct - and the first 4 numbers are xywh, in pixel coordinates.
		// The remaining numbers are the 80 classes.
		//
		// We need to transpose the output, otherwise we're doing 84 sparse reads per box.

		//printf("output shape: %f %f %f\n", shape[0], shape[1], shape[2]);
		//for (int row = 0; row < 10; row++) {
		//	const float* rowptr = out.row(row);
		//	printf("row %2d: %f %f %f %f %f ... %f %f %f %f %f\n", row, rowptr[0], rowptr[1], rowptr[2], rowptr[3], rowptr[4], rowptr[79], rowptr[80], rowptr[81], rowptr[82], rowptr[83]);
		//}

		ncnn::Mat out;
		Transpose(outRaw, out, nullptr);
		proposals.reserve(256);

		for (size_t i = 0; i < (size_t) out.h; i++) {
			float        secondMaxProb = 0;
			float        maxProb       = 0;
			int          maxProbCls    = 0;
			const float* prob          = out.row(i);
			for (size_t c = 4; c < (size_t) out.w; c++) {
				if (prob[c] > maxProb) {
					secondMaxProb = maxProb;
					maxProb       = prob[c];
					maxProbCls    = c - 4;
				}
			}
			if (maxProb >= prob_threshold) {
				Object obj;
				obj.label       = maxProbCls;
				obj.prob        = maxProb;
				obj.probMargin  = maxProb - secondMaxProb;
				obj.rect.x      = prob[0] - prob[2] / 2;
				obj.rect.y      = prob[1] - prob[3] / 2;
				obj.rect.width  = prob[2];
				obj.rect.height = prob[3];
				proposals.push_back(obj);
				//printf("proposal %d: %f %f %f %f (cls: %d, prob: %.2f)\n", (int) i, obj.rect.x, obj.rect.y, obj.rect.width, obj.rect.height, obj.label, obj.prob);
			}
		}
	}

	// sort all proposals by score from highest to lowest
	qsort_descent_inplace(proposals);

	// apply nms with nms_threshold
	std::vector<int> picked;
	nms_sorted_bboxes(proposals, picked, nms_threshold);

	int count = picked.size();

	objects.resize(count);

	//printf("scale: %f, wpad: %d, hpad: %d\n", scale, wpad, hpad);

	for (int i = 0; i < count; i++) {
		objects[i]  = proposals[picked[i]];
		Object& obj = objects[i];

		//printf("object %d, class %d (%.1f%%) %f,%f,%f,%f\n", i, obj.label, obj.prob * 100, obj.rect.x, obj.rect.y, obj.rect.width, obj.rect.height);

		// adjust offset to original unpadded
		float x0 = (obj.rect.x - (wpad / 2)) / scale;
		float y0 = (obj.rect.y - (hpad / 2)) / scale;
		float x1 = (obj.rect.x + obj.rect.width - (wpad / 2)) / scale;
		float y1 = (obj.rect.y + obj.rect.height - (hpad / 2)) / scale;

		if (detectFlags & DetectFlagNoClip) {
			// clip to 1x the NN size, in case we have crazy numbers.
			x0 = clamp(x0, (float) -img_w, (float) (img_w * 2));
			y0 = clamp(y0, (float) -img_h, (float) (img_h * 2));
			x1 = clamp(x1, (float) -img_w, (float) (img_w * 2));
			y1 = clamp(y1, (float) -img_h, (float) (img_h * 2));
		} else {
			// clip to NN size. The img_w - 1 and img_h - 1 (i.e. the -1) came from the original
			// demo code. But looking at it now, it looks to me like the -1 is wrong.
			x0 = clamp(x0, 0.f, (float) (img_w - 1));
			y0 = clamp(y0, 0.f, (float) (img_h - 1));
			x1 = clamp(x1, 0.f, (float) (img_w - 1));
			y1 = clamp(y1, 0.f, (float) (img_h - 1));
		}

		obj.rect.x      = x0;
		obj.rect.y      = y0;
		obj.rect.width  = x1 - x0;
		obj.rect.height = y1 - y0;
	}
}

void DetectYOLO(ModelTypes modelType, ncnn::Net& net, int nn_width, int nn_height, int detectFlags, float prob_threshold, float nms_threshold, const cv::Mat& img, std::vector<Detection>& objects) {
	std::vector<Object> obj;
	detect_yolov7_8(modelType, net, nn_width, nn_height, detectFlags, prob_threshold, nms_threshold, img, obj);
	//fprintf(stderr, "obj.size = %d\n", (int) obj.size());

	for (const auto& o : obj) {
		Detection det;
		det.Box.X            = o.rect.x;
		det.Box.Y            = o.rect.y;
		det.Box.Width        = o.rect.width;
		det.Box.Height       = o.rect.height;
		det.Class            = o.label;
		det.Confidence       = o.prob;
		det.ConfidenceMargin = o.probMargin;
		objects.push_back(det);
	}
}
