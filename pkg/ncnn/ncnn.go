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

func (d *Detector) DetectObjects(img nn.ImageCrop, params *nn.DetectionParams) ([]nn.ObjectDetection, error) {
	detections := make([]C.Detection, 100)
	nDetections := C.int(0)
	flags := C.int(0)
	if params.Unclipped {
		flags |= C.DetectFlagNoClip
	}
	C.DetectObjects(d.detector,
		C.int(img.NChan), (*C.uchar)(img.Pointer()), C.int(img.CropWidth), C.int(img.CropHeight), C.int(img.Stride()),
		flags, C.float(params.ProbabilityThreshold), C.float(params.NmsThreshold),
		C.int(len(detections)), (*C.Detection)(unsafe.Pointer(&detections[0])), &nDetections)
	result := make([]nn.ObjectDetection, nDetections)

	for i := 0; i < int(nDetections); i++ {
		result[i] = nn.ObjectDetection{
			Class:      int(detections[i].Class),
			Confidence: float32(detections[i].Confidence),
			Box: nn.Rect{
				X:      int(detections[i].Box.X),
				Y:      int(detections[i].Box.Y),
				Width:  int(detections[i].Box.Width),
				Height: int(detections[i].Box.Height),
			},
		}
	}
	return result, nil
}

func (d *Detector) Config() *nn.ModelConfig {
	return &d.config
}
