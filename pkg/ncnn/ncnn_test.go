package ncnn_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bmharper/cimg/v2"
	"github.com/cyclopcam/cyclops/pkg/ncnn"
	"github.com/cyclopcam/cyclops/pkg/nn"
	"github.com/stretchr/testify/require"
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

func TestInit(t *testing.T) {
	ncnn.Initialize()
}

func TestYoloV7(t *testing.T) {
	testModel(t, "yolov7-tiny")
}

func TestYoloV8n(t *testing.T) {
	testModel(t, "yolov8n")
}

func TestYoloV8s(t *testing.T) {
	testModel(t, "yolov8s")
}

func testModel(t *testing.T, modelFilename string) {
	config, err := nn.LoadModelConfig(filepath.Join(modelsDir(), modelFilename+".json"))
	require.NoError(t, err)
	nnWidth, nnHeight := config.Width, config.Height
	detector, _ := ncnn.NewDetector(config, nn.ThreadingModeSingle, filepath.Join(modelsDir(), modelFilename+".param"), filepath.Join(modelsDir(), modelFilename+".bin"))
	defer detector.Close()
	img := loadImage("driveway001-man.jpg")
	img = cimg.ResizeNew(img, nnWidth, nnHeight, nil)
	batch := nn.MakeImageBatchSingle(img.Width, img.Height, img.NChan(), img.Stride, img.Pixels)
	detectionsBatch, _ := detector.DetectObjects(batch, nn.NewDetectionParams())
	require.Equal(t, 1, len(detectionsBatch))
	detections := detectionsBatch[0]
	t.Logf("num detections: %v", len(detections))
	for _, det := range detections {
		t.Logf("det: %v", det)
	}
}

func BenchmarkYoloV7Tiny(b *testing.B) {
	benchmarkModel(b, "yolov7-tiny")
}

func benchmarkModel(b *testing.B, modelFilename string) {
	config, err := nn.LoadModelConfig(filepath.Join(modelsDir(), modelFilename+".json"))
	require.NoError(b, err)
	detector, _ := ncnn.NewDetector(config, nn.ThreadingModeSingle, filepath.Join(modelsDir(), modelFilename+".param"), filepath.Join(modelsDir(), modelFilename+".bin"))
	defer detector.Close()
	img := loadImage("driveway001-man.jpg")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		batch := nn.MakeImageBatchSingle(img.Width, img.Height, img.NChan(), img.Stride, img.Pixels)
		detector.DetectObjects(batch, nn.NewDetectionParams())
	}
}
