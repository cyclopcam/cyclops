package ncnn

// package ncnn is a wrapper around https://github.com/Tencent/ncnn

// #cgo CPPFLAGS: -fopenmp -I${SRCDIR}/../../ncnn/build/src -I${SRCDIR}/../../ncnn/src
// #cgo LDFLAGS: -L${SRCDIR}/../../ncnn/build/src -lncnn -lgomp
// #include <stdio.h>
// #include <stdlib.h>
// #include "ncnn.h"
import "C"

import (
	"fmt"
	"unsafe"

	"github.com/cyclopcam/cyclops/pkg/nn"
)

func Initialize() {
	C.InitNcnn()
}

type Detector struct {
	detector *C.NcnnDetector
	config   nn.ModelConfig
}

func NewDetector(config *nn.ModelConfig, threadingMode nn.ThreadingMode, paramsFile, binFile string) (*Detector, error) {
	detectorFlags := C.int(0)
	if threadingMode == nn.ThreadingModeSingle {
		detectorFlags |= C.DetectorFlagSingleThreaded
	}
	cModelType := C.CString(config.Architecture)
	cParams := C.CString(paramsFile)
	cBin := C.CString(binFile)
	cdet := C.CreateDetector(detectorFlags, cModelType, cParams, cBin, C.int(config.Width), C.int(config.Height))
	C.free(unsafe.Pointer(cModelType))
	C.free(unsafe.Pointer(cParams))
	C.free(unsafe.Pointer(cBin))
	if cdet == nil {
		return nil, fmt.Errorf("Failed to create NN detector (%v, '%v', '%v')", config.Architecture, paramsFile, binFile)
	}
	return &Detector{
		detector: cdet,
		config:   *config,
	}, nil
}

func (d *Detector) Close() {
	C.DeleteDetector(d.detector)
}

func (d *Detector) DetectObjects(batch nn.ImageBatch, params *nn.DetectionParams) ([][]nn.ObjectDetection, error) {
	// NCNN is built for single element operation (i.e. batch size 1), so we just loop over
	// all the images in the batch.
	batchResult := make([][]nn.ObjectDetection, batch.BatchSize)
	for i := 0; i < batch.BatchSize; i++ {
		detections := make([]C.Detection, 100)
		nDetections := C.int(0)
		flags := C.int(0)
		if params.Unclipped {
			flags |= C.DetectFlagNoClip
		}
		img := batch.Image(i)
		C.DetectObjects(d.detector,
			C.int(img.NChan), (*C.uchar)(img.Pointer()), C.int(img.CropWidth), C.int(img.CropHeight), C.int(img.Stride()),
			flags, C.float(params.ProbabilityThreshold), C.float(params.NmsIouThreshold),
			C.int(len(detections)), (*C.Detection)(unsafe.Pointer(&detections[0])), &nDetections)
		result := make([]nn.ObjectDetection, nDetections)

		for i := 0; i < int(nDetections); i++ {
			result[i] = nn.ObjectDetection{
				Class:      int(detections[i].Class),
				Confidence: float32(detections[i].Confidence),
				Box: nn.Rect{
					X:      int32(detections[i].Box.X),
					Y:      int32(detections[i].Box.Y),
					Width:  int32(detections[i].Box.Width),
					Height: int32(detections[i].Box.Height),
				},
			}
		}
		batchResult[i] = result
	}
	return batchResult, nil
}

func (d *Detector) Config() *nn.ModelConfig {
	return &d.config
}
