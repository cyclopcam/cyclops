package ncnn_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bmharper/cimg/v2"
	"github.com/bmharper/cyclops/server/ncnn"
)

func projectRootDir() string {
	cd, _ := os.Getwd()
	return filepath.Dir(filepath.Dir(cd))
}

func testDataDir() string {
	return filepath.Join(projectRootDir(), "testdata")
}

func modelsDir() string {
	return filepath.Join(projectRootDir(), "models")
}

func loadImage(name string) *cimg.Image {
	bin, err := os.ReadFile(filepath.Join(testDataDir(), name))
	if err != nil {
		panic(err)
	}
	img, err := cimg.Decompress(bin)
	if err != nil {
		panic(err)
	}
	if img.NChan() == 3 {
		return img
	}
	return img.ToRGB()
}

func TestHello(t *testing.T) {
	detector, _ := ncnn.NewDetector("yolov7", filepath.Join(modelsDir(), "yolov7-tiny.param"), filepath.Join(modelsDir(), "yolov7-tiny.bin"))
	defer detector.Close()
	img := loadImage("driveway001-man.jpg")
	detections, _ := detector.DetectObjects(img.NChan(), img.Pixels, img.Width, img.Height)
	t.Logf("num detections: %v", len(detections))
	for _, det := range detections {
		t.Logf("det: %v", det)
	}
}
