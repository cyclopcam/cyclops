package nnaccel

import (
	"math"
	"time"
	"unsafe"

	"github.com/cyclopcam/cyclops/pkg/nn"
)

// #include "interface.h"
import "C"

type AsyncJob struct {
	accel  *Accelerator   // The accelerator that ran the job
	handle unsafe.Pointer // Handle to the job
}

// Returns true if the job is finished
func (j *AsyncJob) Wait(wait time.Duration) bool {
	milliseconds := wait / time.Millisecond
	if milliseconds > math.MaxInt32 {
		milliseconds = math.MaxInt32
	}
	return C.NAWaitForJob(j.accel.handle, j.handle, C.uint32_t(milliseconds)) == 0
}

func (j *AsyncJob) GetObjectDetections(batchEl int) ([]nn.ObjectDetection, error) {
	// This is an arbitrary limit.
	maxDetections := 1000
	var detections *C.NNAObjectDetection
	var numDetections C.size_t
	C.NAGetObjectDetections(j.accel.handle, j.handle, C.int(batchEl), C.size_t(maxDetections), &detections, &numDetections)
	dets := unsafe.Slice(detections, int(numDetections))
	out := make([]nn.ObjectDetection, len(dets))
	for i := 0; i < len(dets); i++ {
		out[i].Class = int(dets[i].ClassID)
		out[i].Confidence = float32(dets[i].Confidence)
		out[i].Box.X = int32(dets[i].X)
		out[i].Box.Y = int32(dets[i].Y)
		out[i].Box.Width = int32(dets[i].Width)
		out[i].Box.Height = int32(dets[i].Height)
	}
	C.free(unsafe.Pointer(detections))
	return out, nil
}

func (j *AsyncJob) Close() {
	//fmt.Printf("fooXXX\n")
	C.NACloseJob(j.accel.handle, j.handle)
}
