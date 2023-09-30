package ncnn_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bmharper/cimg/v2"
	"github.com/cyclopcam/cyclops/server/ncnn"
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

func TestYoloV7(t *testing.T) {
	testModel(t, "yolov7", "yolov7-tiny", 320, 256)
}

func TestYoloV8n(t *testing.T) {
	testModel(t, "yolov8", "yolov8n", 320, 256)
}

func TestYoloV8s(t *testing.T) {
	testModel(t, "yolov8", "yolov8s", 320, 256)
}

func testModel(t *testing.T, modelType, modelFilename string, width, height int) {
	detector, _ := ncnn.NewDetector(modelType, filepath.Join(modelsDir(), modelFilename+".param"), filepath.Join(modelsDir(), modelFilename+".bin"), width, height)
	defer detector.Close()
	img := loadImage("driveway001-man.jpg")
	detections, _ := detector.DetectObjects(img.NChan(), img.Pixels, img.Width, img.Height, nil)
	t.Logf("num detections: %v", len(detections))
	for _, det := range detections {
		t.Logf("det: %v", det)
	}
}

func BenchmarkYoloV7Tiny(b *testing.B) {
	benchmarkModel(b, "yolov7", "yolov7-tiny", 320, 256)
}

func benchmarkModel(b *testing.B, modelType, modelFilename string, width, height int) {
	detector, _ := ncnn.NewDetector(modelType, filepath.Join(modelsDir(), modelFilename+".param"), filepath.Join(modelsDir(), modelFilename+".bin"), width, height)
	defer detector.Close()
	img := loadImage("driveway001-man.jpg")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		detector.DetectObjects(img.NChan(), img.Pixels, img.Width, img.Height, nil)
	}
}
