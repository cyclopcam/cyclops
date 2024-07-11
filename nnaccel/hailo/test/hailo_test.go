package hailotest

import (
	"fmt"
	"testing"
	"time"
	"unsafe"

	"github.com/bmharper/cimg/v2"
	"github.com/cyclopcam/cyclops/pkg/nn"
	"github.com/cyclopcam/cyclops/pkg/nnaccel"
	"github.com/stretchr/testify/require"
)

func TestObjectDetection(t *testing.T) {
	device, err := nnaccel.Load("hailo")
	require.NoError(t, err)

	fmt.Printf("Hailo module loaded\n")

	setup := nnaccel.ModelSetup{
		BatchSize: 1,
	}
	model, err := device.LoadModel("../../../models/hailo/8L/yolov8s.hef", &setup)
	require.NoError(t, err)

	img, err := cimg.ReadFile("../../../testdata/yard-640x640.jpg")
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
