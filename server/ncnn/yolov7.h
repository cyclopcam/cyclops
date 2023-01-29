void DetectYOLOv7(ncnn::Net& net, int target_size, float prob_threshold, float nms_threshold, const cv::Mat& rgb, std::vector<Detection>& objects);
