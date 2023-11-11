void DetectYOLO(ModelTypes modelType, ncnn::Net& net, int target_width, int target_height, float prob_threshold, float nms_threshold, const cv::Mat& img, std::vector<Detection>& objects);
