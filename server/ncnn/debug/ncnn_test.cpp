
// build & run:
// Regular CMake build for ncnn with OpenMP enabled:
// cmake -DNCNN_SIMPLEOCV=1 -DNCNN_OPENMP=1 -DCMAKE_BUILD_TYPE=Release ..
//
// cd cyclops/server/ncnn
// g++ -O3 -std=c++17 -fopenmp -I. -I../../ncnn/build/src -I../../ncnn/src -L../../ncnn/build/src -o ncnn_test debug/ncnn_test.cpp yolo.cpp ncnn.cpp ncnn_helpers.cpp -lgomp -lstdc++ -lncnn
// ./ncnn_test

// For debugging, first configure the ncnn build with "cmake -DNCNN_SIMPLEOCV=1 -DCMAKE_BUILD_TYPE=Debug ..", and then:
// (note that we link to ncnnd, instead of ncnn)
// g++ -g -Og -std=c++17 -fopenmp -I. -I../../ncnn/build/src -I../../ncnn/src -L../../ncnn/build/src -o ncnn_test debug/ncnn_test.cpp yolo.cpp ncnn.cpp ncnn_helpers.cpp -lgomp -lstdc++ -lncnnd

// With OpenMP disabled inside ncnn:
// cmake -DNCNN_SIMPLEOCV=1 -DNCNN_OPENMP=0 -DNCNN_THREADS=0 -DCMAKE_BUILD_TYPE=Release ..
// g++ -O3 -std=c++17 -fopenmp -I. -I../../ncnn/build/src -I../../ncnn/src -L../../ncnn/build/src -o ncnn_test debug/ncnn_test.cpp yolo.cpp ncnn.cpp ncnn_helpers.cpp -lgomp -lstdc++ -lncnn

#include "layer.h"
#include "net.h"
#include "simpleocv.h"

#include "ncnn.h"

#include <float.h>
#include <stdio.h>
#include <sys/time.h>
#include <string>
#include <vector>
#include <mutex>
#include <thread>
#include <atomic>

bool Benchmark   = false;
bool QuitThreads = false;
bool CSV         = Benchmark;
int  MinThreads  = 1;
int  MaxThreads  = Benchmark ? 12 : 1;

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

void RunDetection(NcnnDetector detector, const cv::Mat& img, bool benchmark) {
	Detection dets[100];
	int       numDetections = 0;
	DetectObjects(detector, 3, img.data, img.cols, img.rows, img.cols * 3, 100, dets, &numDetections);
	if (!benchmark) {
		for (int i = 0; i < numDetections; i++) {
			auto& d = dets[i];
			printf("  class %d, confidence %f, box (%d, %d, %d, %d)\n", d.Class, d.Confidence, d.Box.X, d.Box.Y, d.Box.Width, d.Box.Height);
		}
	}
}

void DetectionThread(std::mutex* lock, std::vector<cv::Mat*>* queue, std::atomic<int>* numResults, TestModel tm) {
	auto detector = CreateDetector(tm.ModelType.c_str(), tm.ParamFile.c_str(), tm.BinFile.c_str(), tm.Width, tm.Height);

	while (true) {
		if (QuitThreads) {
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
		RunDetection(detector, *img, Benchmark);
		numResults->fetch_add(1);
	}

	DeleteDetector(detector);
}

int main(int argc, char** argv) {
	const char* imagepath = "../../testdata/driveway001-man.jpg";
	cv::Mat     m         = cv::imread(imagepath, 1);
	if (m.empty()) {
		fprintf(stderr, "cv::imread %s failed\n", imagepath);
		return -1;
	}

	std::vector<TestModel> testModels = {
	    {"yolov7t", "yolov7", "../../models/yolov7-tiny.param", "../../models/yolov7-tiny.bin", 320, 320},
	    {"yolov8n", "yolov8", "../../models/yolov8n.param", "../../models/yolov8n.bin", 320, 256},
	    {"yolov8s", "yolov8", "../../models/yolov8s.param", "../../models/yolov8s.bin", 320, 256},
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

			QuitThreads = false;
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
				auto detector = CreateDetector(tm.ModelType.c_str(), tm.ParamFile.c_str(), tm.BinFile.c_str(), tm.Width, tm.Height);
				RunDetection(detector, m, true);
				DeleteDetector(detector);
			}

			double estimateRuntime = SecondsSince(start);
			double targetSeconds   = 5.0;
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
			QuitThreads = true;

			for (int i = 0; i < nThreads; i++) {
				threads[i].join();
			}
			//printf("Threads joined\n");

			elapsed = SecondsSince(start);
			if (!Benchmark && elapsed >= 3)
				break;
			if (Benchmark && !CSV)
				printf("  %.1f FPS, %.1f ms/frame (%d reps)\n", nReps / elapsed, elapsed * 1000 / nReps, nReps);
			fps.push_back(nReps / elapsed);
			if (!CSV)
				printf("\n");
		}
		if (CSV) {
			printf("%d,", nThreads);
			for (size_t i = 0; i < fps.size(); i++) {
				printf("%.1f", fps[i]);
				if (i < fps.size() - 1)
					printf(",");
			}
			printf("\n");
		}
	}

	return 0;
}
