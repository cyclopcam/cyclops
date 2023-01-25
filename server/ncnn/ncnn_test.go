package ncnn_test

import (
	"testing"

	"github.com/bmharper/cyclops/server/ncnn"
)

func TestHello(t *testing.T) {
	detector := ncnn.NewDetector("test")
	detector.Close()
}
