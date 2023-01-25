package ncnn

// ncnn is a wrapper around https://github.com/Tencent/ncnn

// #cgo CPPFLAGS: -fopenmp -I${SRCDIR}/../../ncnn/build/src -I${SRCDIR}/../../ncnn/src
// #cgo LDFLAGS: -L${SRCDIR}/../../ncnn/build/src -lncnn -lgomp
// #include "ncnn.h"
import "C"

//func Hello() {
//	C.CreateDetector(C.CString("test"), C.CString("test"), C.CString("test"))
//}

type Detector struct {
	detector C.NcnnDetector
}

func NewDetector(modelPath string) *Detector {
	return &Detector{
		detector: C.CreateDetector(C.CString("test"), C.CString("test"), C.CString("test")),
	}
}

func (d *Detector) Close() {
	C.DeleteDetector(d.detector)
}
