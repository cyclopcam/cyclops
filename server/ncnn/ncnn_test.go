package ncnn_test

import (
	"testing"

	"github.com/bmharper/cyclops/server/ncnn"
)

func TestHello(t *testing.T) {
	detector := ncnn.NewDetector("yolov7", "/home/ben/dev/ncnn/yolov7-tiny.param", "/home/ben/dev/ncnn/yolov7-tiny.bin")
	detector.Close()
}
