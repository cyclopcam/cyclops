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
};

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

static void detect_yolov7_8(ModelTypes modelType, ncnn::Net& net, int nn_width, int nn_height, float prob_threshold, float nms_threshold, const cv::Mat& in_img, std::vector<Object>& objects) {
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
	case ModelTypes::YOLOv8: ex.input("in0", in_pad); break;
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
	} else if (modelType == ModelTypes::YOLOv8) {
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
			float        maxProb    = 0;
			int          maxProbCls = 0;
			const float* prob       = out.row(i);
			for (size_t c = 4; c < (size_t) out.w; c++) {
				if (prob[c] > maxProb) {
					maxProb    = prob[c];
					maxProbCls = c - 4;
				}
			}
			if (maxProb >= prob_threshold) {
				Object obj;
				obj.label       = maxProbCls;
				obj.prob        = maxProb;
				obj.rect.x      = prob[0] - prob[2] / 2;
				obj.rect.y      = prob[1] - prob[3] / 2;
				obj.rect.width  = prob[2];
				obj.rect.height = prob[3];
				proposals.push_back(obj);
				//printf("proposal %d: %f %f %f %f (cls: %d, prob: %.2f)\n", (int) i, obj.rect.x, obj.rect.y, obj.rect.width, obj.rect.height, obj.label, obj.prob);
			}
		}

		/*
		size_t stride = out.w;
		for (int ibox = 0; ibox < out.w; ibox++) {
			const float* ptr = (const float*) out.data;
			ptr += ibox;
			float a[8];
			a[0] = ptr[0];
			a[1] = ptr[stride];
			a[2] = ptr[stride * 2];
			a[3] = ptr[stride * 3];
			a[4] = ptr[stride * 4];
			a[5] = ptr[stride * 5];
			a[6] = ptr[stride * 6];
			a[7] = ptr[stride * 7];

			if (a[4] > 0.1) {
				printf("hi!\n");
			}
			maxPerson = std::max(maxPerson, a[4]);

			float b[8];
			b[0] = ptr[stride * 76];
			b[1] = ptr[stride * 77];
			b[2] = ptr[stride * 78];
			b[3] = ptr[stride * 79];
			b[4] = ptr[stride * 80];
			b[5] = ptr[stride * 81];
			b[6] = ptr[stride * 82];
			b[7] = ptr[stride * 83];
			printf("box %d: %5.1f %5.1f %5.1f %5.1f %.1f %.1f %.1f %.1f ... %.1f %.1f %.1f %.1f %.1f %.1f %.1f %.1f\n", ibox, a[0], a[1], a[2], a[3], a[4], a[5], a[6], a[7], b[0], b[1], b[2], b[3], b[4], b[5], b[6], b[7]);
		}
		*/
	}
	//printf("max person: %f\n", maxPerson);

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

		// clip
		x0 = std::max(std::min(x0, (float) (img_w - 1)), 0.f);
		y0 = std::max(std::min(y0, (float) (img_h - 1)), 0.f);
		x1 = std::max(std::min(x1, (float) (img_w - 1)), 0.f);
		y1 = std::max(std::min(y1, (float) (img_h - 1)), 0.f);

		obj.rect.x      = x0;
		obj.rect.y      = y0;
		obj.rect.width  = x1 - x0;
		obj.rect.height = y1 - y0;
	}
}

void DetectYOLO(ModelTypes modelType, ncnn::Net& net, int nn_width, int nn_height, float prob_threshold, float nms_threshold, const cv::Mat& img, std::vector<Detection>& objects) {
	std::vector<Object> obj;
	detect_yolov7_8(modelType, net, nn_width, nn_height, prob_threshold, nms_threshold, img, obj);
	//fprintf(stderr, "obj.size = %d\n", (int) obj.size());

	for (const auto& o : obj) {
		Detection det;
		det.Box.X      = o.rect.x;
		det.Box.Y      = o.rect.y;
		det.Box.Width  = o.rect.width;
		det.Box.Height = o.rect.height;
		det.Class      = o.label;
		det.Confidence = o.prob;
		objects.push_back(det);
	}
}

/*
static void draw_objects(const cv::Mat& bgr, const std::vector<Object>& objects)
{
    static const char* class_names[] = {
        "person", "bicycle", "car", "motorcycle", "airplane", "bus", "train", "truck", "boat", "traffic light",
        "fire hydrant", "stop sign", "parking meter", "bench", "bird", "cat", "dog", "horse", "sheep", "cow",
        "elephant", "bear", "zebra", "giraffe", "backpack", "umbrella", "handbag", "tie", "suitcase", "frisbee",
        "skis", "snowboard", "sports ball", "kite", "baseball bat", "baseball glove", "skateboard", "surfboard",
        "tennis racket", "bottle", "wine glass", "cup", "fork", "knife", "spoon", "bowl", "banana", "apple",
        "sandwich", "orange", "broccoli", "carrot", "hot dog", "pizza", "donut", "cake", "chair", "couch",
        "potted plant", "bed", "dining table", "toilet", "tv", "laptop", "mouse", "remote", "keyboard", "cell phone",
        "microwave", "oven", "toaster", "sink", "refrigerator", "book", "clock", "vase", "scissors", "teddy bear",
        "hair drier", "toothbrush"};

    static const unsigned char colors[19][3] = {
        {54, 67, 244},
        {99, 30, 233},
        {176, 39, 156},
        {183, 58, 103},
        {181, 81, 63},
        {243, 150, 33},
        {244, 169, 3},
        {212, 188, 0},
        {136, 150, 0},
        {80, 175, 76},
        {74, 195, 139},
        {57, 220, 205},
        {59, 235, 255},
        {7, 193, 255},
        {0, 152, 255},
        {34, 87, 255},
        {72, 85, 121},
        {158, 158, 158},
        {139, 125, 96}};

    int color_index = 0;

    cv::Mat image = bgr.clone();

    for (size_t i = 0; i < objects.size(); i++)
    {
        const Object& obj = objects[i];

        const unsigned char* color = colors[color_index % 19];
        color_index++;

        cv::Scalar cc(color[0], color[1], color[2]);

        fprintf(stderr, "%d = %.5f at %.2f %.2f %.2f x %.2f\n", obj.label, obj.prob,
                obj.rect.x, obj.rect.y, obj.rect.width, obj.rect.height);

        cv::rectangle(image, obj.rect, cc, 2);

        char text[256];
        sprintf(text, "%s %.1f%%", class_names[obj.label], obj.prob * 100);

        int baseLine = 0;
        cv::Size label_size = cv::getTextSize(text, cv::FONT_HERSHEY_SIMPLEX, 0.5, 1, &baseLine);

        int x = obj.rect.x;
        int y = obj.rect.y - label_size.height - baseLine;
        if (y < 0)
            y = 0;
        if (x + label_size.width > image.cols)
            x = image.cols - label_size.width;

        cv::rectangle(image, cv::Rect(cv::Point(x, y), cv::Size(label_size.width, label_size.height + baseLine)),
                      cc, -1);

        cv::putText(image, text, cv::Point(x, y + label_size.height),
                    cv::FONT_HERSHEY_SIMPLEX, 0.5, cv::Scalar(255, 255, 255));
    }

    cv::imshow("image", image);
    cv::waitKey(0);
}

int main(int argc, char** argv)
{
    if (argc != 2)
    {
        fprintf(stderr, "Usage: %s [imagepath]\n", argv[0]);
        return -1;
    }

    const char* imagepath = argv[1];

    cv::Mat m = cv::imread(imagepath, 1);
    if (m.empty())
    {
        fprintf(stderr, "cv::imread %s failed\n", imagepath);
        return -1;
    }

    std::vector<Object> objects;
    detect_yolov7(m, objects);

    draw_objects(m, objects);

    return 0;
}
*/