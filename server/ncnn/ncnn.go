package ncnn

// ncnn is a wrapper around https://github.com/Tencent/ncnn

// #cgo CPPFLAGS: -fopenmp -I${SRCDIR}/../../ncnn/build/src -I${SRCDIR}/../../ncnn/src
// #cgo LDFLAGS: -L${SRCDIR}/../../ncnn/build/src -lncnn -lgomp
// #include <stdio.h>
// #include <stdlib.h>
// #include "ncnn.h"
import "C"
import "unsafe"

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
