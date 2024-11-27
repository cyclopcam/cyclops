package nnaccel

import (
	"fmt"
	"time"
	"unsafe"

	"github.com/cyclopcam/cyclops/pkg/nn"
)

// #include "interface.h"
// #include <malloc.h>
import "C"

type Model struct {
	accel  *Accelerator   // The accelerator that created this model
	handle unsafe.Pointer // Handle to the model
	config nn.ModelConfig
}

func (m *Model) Close() {
	C.NACloseModel(m.accel.handle, m.handle)
}

func (m *Model) Run(batchSize, batchStride, width, height, nchan int, stride int, images unsafe.Pointer) (*AsyncJob, error) {
	job := &AsyncJob{
		accel: m.accel,
	}
	err := m.accel.StatusToErr(C.NARunModel(m.accel.handle, m.handle, C.int(batchSize), C.int(batchStride), C.int(width), C.int(height), C.int(nchan), C.int(stride), images, &job.handle))
	if err != nil {
		return nil, err
	}
	return job, nil
}

// Detection thresholds are ignored here. They need to be setup when the model is initially loaded.
func (m *Model) DetectObjects(batch nn.ImageBatch, params *nn.DetectionParams) ([][]nn.ObjectDetection, error) {
	job, err := m.Run(batch.BatchSize, batch.BatchStride, batch.Width, batch.Height, batch.NChan, batch.Stride, unsafe.Pointer(&batch.Pixels[0]))
	if err != nil {
		return nil, err
	}
	defer job.Close()
	if !job.Wait(5 * time.Second) {
		return nil, fmt.Errorf("Timeout waiting for NN result")
	}
	result := make([][]nn.ObjectDetection, batch.BatchSize)
	for i := 0; i < batch.BatchSize; i++ {
		result[i], err = job.GetObjectDetections(i)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (m *Model) Config() *nn.ModelConfig {
	return &m.config
}
