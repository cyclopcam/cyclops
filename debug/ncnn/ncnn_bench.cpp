// Run 'build' from the project root to build this debug/analysis program.
// In addition to a benchmarking tool, this also helps verify object detection output.

// ncnn includes
#include "layer.h"
#include "net.h"
#include "simpleocv.h"

// Our own ncnn wrapper/helper
#include "ncnn.h"

// stb_image and stb_image_write implementations are inside ncnn's simpleocv, so we don't
// have to instantiate them here.
//#define STB_IMAGE_IMPLEMENTATION
#include "../stb/stb_image.h"
#include "../stb/stb_image_write.h"

#define STB_IMAGE_RESIZE_IMPLEMENTATION
#include "../stb/stb_image_resize2.h"

#include <float.h>
#include <stdio.h>
#include <sys/time.h>
#include <string>
#include <vector>
#include <mutex>
#include <thread>
#include <atomic>

bool Benchmark  = true;
bool CSV        = Benchmark;
bool DumpImages = !Benchmark;
int  MinThreads = 1;
//int  MaxThreads = Benchmark ? 12 : 1;
int MaxThreads = 1;

// If you make DetectorFlags = 0, then NCNN will run each NN on as many CPU cores
// as it can. This is how we run NCNN in practice on Rpi5. On a desktop CPU, we
// run it single threaded, and spawn our own NN threads.
int DetectorFlags = DetectorFlagSingleThreaded;
//int DetectorFlags = 0;

bool QuitThreadsSignal = false;

struct TestModel {
	std::string Name;
	std::string ModelType;
	std::string ParamFile;
	std::string BinFile;
	int         Width;
	int         Height;
};

int64_t timeInMilliseconds(void) {
	struct timeval tv;

	gettimeofday(&tv, NULL);
	return (((int64_t) tv.tv_sec) * 1000) + (tv.tv_usec / 1000);
}

void Nanosleep(int64_t ns) {
	timespec tim, tim2;
	tim.tv_sec  = ns / 1000000000L;
	tim.tv_nsec = ns % 1000000000L;
	nanosleep(&tim, &tim2);
}

void Sleep(double seconds) {
	Nanosleep((int64_t) (seconds * 1000000000L));
}

double SecondsSince(int64_t ms) {
	return (timeInMilliseconds() - ms) / 1000.0;
}

void RunDetection(NcnnDetector* detector, const cv::Mat& img, bool benchmark, const TestModel& tm) {
	Detection dets[100];
	int       numDetections = 0;
	bool      draw          = DumpImages && !Benchmark;
	cv::Mat   copy;
	float     minProb      = 0.5f;
	float     nmsThreshold = 0.45f;
	DetectObjects(detector, 3, img.data, img.cols, img.rows, img.cols * 3, 0, minProb, nmsThreshold, 100, dets, &numDetections);
	if (!benchmark) {
		if (draw)
			copy = img.clone();

		for (int i = 0; i < numDetections; i++) {
			const auto& d = dets[i];
			printf("  class %d, confidence %f, box (%d, %d, %d, %d)\n", d.Class, d.Confidence, d.Box.X, d.Box.Y, d.Box.Width, d.Box.Height);
			if (draw) {
				cv::rectangle(copy, cv::Rect(d.Box.X, d.Box.Y, d.Box.Width, d.Box.Height), cv::Scalar(0, 255, 0), 2);
			}
		}

		if (draw) {
			char fn[256];
			sprintf(fn, "%s-detection.jpg", tm.Name.c_str());
			stbi_write_jpg(fn, copy.cols, copy.rows, 3, copy.data, 95);
			//cv::imwrite(fn, copy);
		}
	}
}

void DetectionThread(std::mutex* lock, std::vector<cv::Mat*>* queue, std::atomic<int>* numResults, TestModel tm) {
	auto detector = CreateDetector(DetectorFlags, tm.ModelType.c_str(), tm.ParamFile.c_str(), tm.BinFile.c_str(), tm.Width, tm.Height);

	while (true) {
		if (QuitThreadsSignal) {
			//printf("Thread quitting\n");
			break;
		}
		lock->lock();
		if (queue->size() == 0) {
			lock->unlock();
			Sleep(0.001);
			continue;
		}
		auto img = queue->back();
		queue->pop_back();
		lock->unlock();
		//printf("Running detection\n");
		RunDetection(detector, *img, Benchmark, tm);
		numResults->fetch_add(1);
	}

	DeleteDetector(detector);
}

int main(int argc, char** argv) {
	const char* imagepath = "testdata/driveway001-man.jpg";
	//const char* imagepath = "testdata/porch003-man.jpg";
	//const char* imagepath = "testdata/man-pos-2-0.jpg";
	//cv::Mat m = cv::imread(imagepath, 1);
	//if (m.empty()) {
	//	fprintf(stderr, "cv::imread %s failed\n", imagepath);
	//	return -1;
	//}
	int      width, height, channels;
	uint8_t* img = stbi_load(imagepath, &width, &height, &channels, 3);
	if (channels != 3) {
		fprintf(stderr, "stbi_load %s failed - channels (%d) != 3\n", imagepath, channels);
		return -1;
	}

	std::vector<TestModel> testModels = {
	    {"yolov8s_320_256", "yolov8", "models/coco/ncnn/yolov8s_320_256.param", "models/coco/ncnn/yolov8s_320_256.bin", 320, 256},
	    {"yolov8m_320_256", "yolov8", "models/coco/ncnn/yolov8m_320_256.param", "models/coco/ncnn/yolov8m_320_256.bin", 320, 256},
	    //{"yolov8m_640_480", "yolov8", "models/coco/ncnn/yolov8m_640_480.param", "models/coco/ncnn/yolov8m_640_480.bin", 640, 480},
	    {"yolo11s_320_256", "yolo11", "models/coco/ncnn/yolo11s_320_256.param", "models/coco/ncnn/yolo11s_320_256.bin", 320, 256},
	    {"yolo11m_320_256", "yolo11", "models/coco/ncnn/yolo11m_320_256.param", "models/coco/ncnn/yolo11m_320_256.bin", 320, 256},
	    //{"yolo11m_640_480", "yolo11", "models/coco/ncnn/yolo11m_640_480.param", "models/coco/ncnn/yolo11m_640_480.bin", 640, 480},
	};

	if (CSV) {
		printf("threads,");
		for (int i = 0; i < testModels.size(); i++) {
			printf("%s", testModels[i].Name.c_str());
			if (i < testModels.size() - 1)
				printf(",");
		}
		printf("\n");
	}

	for (int nThreads = MinThreads; nThreads <= MaxThreads; nThreads++) {
		if (!CSV)
			printf("%d threads\n", nThreads);

		std::vector<double> fps;

		for (auto tm : testModels) {
			if (!CSV)
				printf("Testing %s\n", tm.Name.c_str());

			// letterbox to top-left
			double   scale         = tm.Width / (double) width;
			int      resizedWidth  = tm.Width;
			int      resizedHeight = (int) (height * scale);
			uint8_t* imgResized    = (uint8_t*) malloc(resizedWidth * resizedHeight * 3);

			uint8_t* imgNN = (uint8_t*) malloc(tm.Width * tm.Height * 3);
			memset(imgNN, 0, tm.Width * tm.Height * 3);
			for (int y = 0; y < std::min(resizedHeight, tm.Height); y++)
				memcpy(imgNN + y * tm.Width * 3, img + y * width * 3, resizedWidth * 3);
			free(imgResized);

			cv::Mat m(tm.Height, tm.Width, CV_8UC3, imgNN);

			QuitThreadsSignal = false;
			std::vector<std::thread> threads;
			std::mutex               queueLock;
			std::vector<cv::Mat*>    queue;
			std::atomic<int>         numResults;
			numResults = 0;

			for (int i = 0; i < nThreads; i++) {
				threads.push_back(std::thread(DetectionThread, &queueLock, &queue, &numResults, tm));
			}

			auto start = timeInMilliseconds();

			// Measure the speed of a single run, so that we can figure out how many iterations to perform
			if (Benchmark) {
				auto detector = CreateDetector(DetectorFlags, tm.ModelType.c_str(), tm.ParamFile.c_str(), tm.BinFile.c_str(), tm.Width, tm.Height);
				RunDetection(detector, m, true, tm);
				DeleteDetector(detector);
			}

			double estimateRuntime = SecondsSince(start);
			double targetSeconds   = 4.0;
			int    nReps           = 1;
			if (Benchmark) {
				nReps = (int) ceil(nThreads * targetSeconds / estimateRuntime);
				nReps = std::max(1, nReps);
			}
			double elapsed = 0;
			start          = timeInMilliseconds();
			for (int i = 0; i < nReps; i++) {
				queueLock.lock();
				queue.push_back(&m);
				queueLock.unlock();
			}

			//printf("Waiting for %d nReps\n", nReps);

			while (true) {
				Sleep(0.001);
				if (numResults.load() == nReps)
					break;
			}
			//printf("All done\n");
			QuitThreadsSignal = true;

			for (int i = 0; i < nThreads; i++) {
				threads[i].join();
			}
			//printf("Threads joined\n");

			elapsed = SecondsSince(start);
			if (!Benchmark && elapsed >= 3)
				break;
			if (Benchmark && !CSV)
				printf("  %.2f FPS, %.1f ms/frame (%d reps)\n", nReps / elapsed, elapsed * 1000 / nReps, nReps);
			fps.push_back(nReps / elapsed);
			if (!CSV)
				printf("\n");
			free(imgResized);
		}
		if (CSV) {
			printf("%d,", nThreads);
			for (size_t i = 0; i < fps.size(); i++) {
				//printf("%.2f", fps[i]);
				printf("%.2f", 1000.0 / fps[i]);
				if (i < fps.size() - 1)
					printf(",");
			}
			printf("\n");
		}
	}

	free(img);
	return 0;
}
