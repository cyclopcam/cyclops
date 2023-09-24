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

	"github.com/bmharper/cyclops/server/nn"
)

type Detector struct {
	detector C.NcnnDetector
}

func NewDetector(modelType, params, bin string, width, height int) (*Detector, error) {
	cModelType := C.CString(modelType)
	cParams := C.CString(params)
	cBin := C.CString(bin)
	cdet := C.CreateDetector(cModelType, cParams, cBin, C.int(width), C.int(height))
	C.free(unsafe.Pointer(cModelType))
	C.free(unsafe.Pointer(cParams))
	C.free(unsafe.Pointer(cBin))
	if cdet == nil {
		return nil, fmt.Errorf("Failed to create NN detector (%v, '%v', '%v')", modelType, params, bin)
	}
	return &Detector{
		detector: cdet,
	}, nil
}

func (d *Detector) Close() {
	C.DeleteDetector(d.detector)
}

func (d *Detector) DetectObjects(nchan int, image []byte, width, height int) ([]nn.Detection, error) {
	detections := make([]C.Detection, 100)
	nDetections := C.int(0)
	C.DetectObjects(d.detector,
		C.int(nchan), (*C.uchar)(unsafe.Pointer(&image[0])), C.int(width), C.int(height), C.int(width*nchan),
		C.int(len(detections)), (*C.Detection)(unsafe.Pointer(&detections[0])), &nDetections)
	result := make([]nn.Detection, nDetections)

	for i := 0; i < int(nDetections); i++ {
		result[i] = nn.Detection{
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
