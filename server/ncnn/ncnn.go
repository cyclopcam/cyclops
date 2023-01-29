package ncnn

// ncnn is a wrapper around https://github.com/Tencent/ncnn

// #cgo CPPFLAGS: -fopenmp -I${SRCDIR}/../../ncnn/build/src -I${SRCDIR}/../../ncnn/src
// #cgo LDFLAGS: -L${SRCDIR}/../../ncnn/build/src -lncnn -lgomp
// #include <stdio.h>
// #include <stdlib.h>
// #include "ncnn.h"
import "C"

import (
	"unsafe"

	"github.com/bmharper/cyclops/server/nn"
)

type Detector struct {
	detector C.NcnnDetector
}

func NewDetector(modelType, params, bin string) *Detector {
	cModelType := C.CString(modelType)
	cParams := C.CString(params)
	cBin := C.CString(bin)
	detector := &Detector{
		detector: C.CreateDetector(cModelType, cParams, cBin),
	}
	C.free(unsafe.Pointer(cModelType))
	C.free(unsafe.Pointer(cParams))
	C.free(unsafe.Pointer(cBin))
	return detector
}

func (d *Detector) Close() {
	C.DeleteDetector(d.detector)
}

func (d *Detector) DetectObjects(nchan int, rgba []byte, width, height int) ([]nn.Detection, error) {
	detections := make([]C.Detection, 100)
	nDetections := C.int(0)
	C.DetectObjects(d.detector,
		C.int(nchan), (*C.uchar)(unsafe.Pointer(&rgba[0])), C.int(width), C.int(height), C.int(width*nchan),
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