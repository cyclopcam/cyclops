package hailotest

import (
	"path/filepath"
	"testing"
	"time"
	"unsafe"

	"github.com/bmharper/cimg/v2"
	"github.com/cyclopcam/cyclops/pkg/nn"
	"github.com/cyclopcam/cyclops/pkg/nnaccel"
	"github.com/stretchr/testify/require"
)

const repoRoot = "../../.."

// modelName is eg "yolov8s"
func loadModel(t *testing.T, modelName string) *nnaccel.Model {
	device, err := nnaccel.Load("hailo")
	require.NoError(t, err)

	setup := nnaccel.ModelSetup{
		BatchSize: 1,
	}
	model, err := device.LoadModel(filepath.Join(repoRoot, "models/hailo/8L/"+modelName+".hef"), &setup)
	require.NoError(t, err)

	return model
}

func BenchmarkObjectDetection(b *testing.B) {
	modelName := "yolov8s"
	batchSize := 1

	device, _ := nnaccel.Load("hailo")
	setup := nnaccel.ModelSetup{
		BatchSize: batchSize,
	}
	model, _ := device.LoadModel(filepath.Join(repoRoot, "models/hailo/8L/"+modelName+".hef"), &setup)
	img, _ := cimg.ReadFile(filepath.Join(repoRoot, "testdata/yard-640x640.jpg"))
	rgb := img.ToRGB()
	batch := make([]byte, batchSize*img.Width*img.Height*img.NChan())
	for i := 0; i < batchSize; i++ {
		copy(batch[i*img.Width*img.Height*img.NChan():], rgb.Pixels)
	}

	times := [4]time.Duration{}

	for i := 0; i < b.N+1; i++ {
		if i == 1 {
			// The first inference run is slow
			b.ResetTimer()
		}
		t1 := time.Now()
		job, _ := model.Run(batchSize, img.Width, img.Height, img.NChan(), unsafe.Pointer(&batch[0]))
		t2 := time.Now()
		job.Wait(time.Second)
		t3 := time.Now()
		job.GetObjectDetections()
		t4 := time.Now()
		job.Close()
		t5 := time.Now()

		if i >= 1 {
			times[0] += t2.Sub(t1)
			times[1] += t3.Sub(t2)
			times[2] += t4.Sub(t3)
			times[3] += t5.Sub(t4)
		}
	}

	for i := 0; i < len(times); i++ {
		times[i] = times[i] / time.Duration(b.N)
	}
	b.Logf("Segment times: %v, %v, %v, %v", times[0], times[1], times[2], times[3])
	b.Logf("FPS:           %v (%v / %v)", float64(b.N)/float64(b.Elapsed().Seconds()), b.N, b.Elapsed().Seconds())

	model.Close()
}

func TestObjectDetection(t *testing.T) {
	model := loadModel(t, "yolov8s")

	img, err := cimg.ReadFile(filepath.Join(repoRoot, "testdata/yard-640x640.jpg"))
	require.NoError(t, err)
	rgb := img.ToRGB() // might already be RGB, but just to be sure

	job, err := model.Run(1, img.Width, img.Height, img.NChan(), unsafe.Pointer(&rgb.Pixels[0]))
	require.NoError(t, err)

	// Wait for async job to complete
	require.True(t, job.Wait(time.Second))

	dets, err := job.GetObjectDetections()
	require.NoError(t, err)
	for _, d := range dets {
		t.Logf("Class %v (confidence %.3f): %v,%v - %v,%v", d.Class, d.Confidence, d.Box.X, d.Box.Y, d.Box.X+d.Box.Width, d.Box.Y+d.Box.Height)
	}
	job.Close()

	expectDets := []nn.ObjectDetection{
		{Class: 0, Box: nn.Rect{X: 452, Y: 244, Width: 75, Height: 222}},
		{Class: 2, Box: nn.Rect{X: 61, Y: 205, Width: 336, Height: 159}},
	}
	require.Equal(t, len(expectDets), len(dets))
	for i := 0; i < len(expectDets); i++ {
		//t.Logf("iou %v\n", expectDets[i].Box.IOU(dets[i].Box))
		require.Equal(t, expectDets[i].Class, dets[i].Class)
		require.GreaterOrEqualf(t, expectDets[i].Box.IOU(dets[i].Box), float32(0.9), "IOU too low")
	}

	model.Close()

	//fmt.Printf("Done\n")
}
