package nnaccel

// #include "interface.h"
import "C"
import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"github.com/cyclopcam/cyclops/pkg/nn"
)

type Accelerator struct {
	handle unsafe.Pointer
}

// Load an NN accelerator.
// At present, the only accelerator we have is "hailo"
func Load(accelName string) (*Accelerator, error) {
	cwd, _ := os.Getwd()
	//fmt.Printf("cwd = %v\n", cwd)

	// relative path from the source code root
	srcCodeRelPath := "nnaccel/hailo/bin"

	if strings.HasSuffix(cwd, "/nnaccel/hailo/test") {
		// We're being run as a Go unit test inside nnaccel/hailo/test
		srcCodeRelPath = "../bin"
	}

	tryPaths := []string{
		srcCodeRelPath, // relative path from the source code root.
		"/usr/local/lib",
	}
	allErrors := strings.Builder{}
	for _, dir := range tryPaths {
		m := Accelerator{}
		fullPath := filepath.Join(dir, "libcyclops"+accelName+".so")
		cFullPath := C.CString(fullPath)
		err := CError(C.LoadNNAccel(cFullPath, &m.handle))
		C.free(unsafe.Pointer(cFullPath))
		if err != nil {
			fmt.Fprintf(&allErrors, "Loading %v: %v\n", fullPath, err)
		} else {
			return &m, nil
		}
	}
	return nil, errors.New(allErrors.String())
}

func (m *Accelerator) LoadModel(modelDir, modelName string, setup *nn.ModelSetup) (*Model, error) {
	model := Model{
		accel: m,
	}
	cModelDir := C.CString(modelDir)
	cModelName := C.CString(modelName)
	cSetup := C.NNModelSetup{
		BatchSize:            C.int(setup.BatchSize),
		ProbabilityThreshold: C.float(setup.ProbabilityThreshold),
		NmsIouThreshold:      C.float(setup.NmsIouThreshold),
	}
	err := m.StatusToErr(C.NALoadModel(m.handle, cModelDir, cModelName, &cSetup, &model.handle))
	C.free(unsafe.Pointer(cModelDir))
	C.free(unsafe.Pointer(cModelName))
	if err != nil {
		return nil, err
	}

	var info C.NNModelInfo
	C.NAModelInfo(m.handle, model.handle, &info)
	model.config.Architecture = "YOLOv8"  // ASSUMPTION
	model.config.Classes = nn.COCOClasses // ASSUMPTION
	model.config.Width = int(info.Width)
	model.config.Height = int(info.Height)

	return &model, nil
}

func (m *Accelerator) StatusToErr(status C.int) error {
	if status != 0 {
		return errors.New(C.GoString(C.NAStatusStr(m.handle, status)))
	}
	return nil
}

// Consume a C heap allocated char* and return it as a Go error.
// Before returning, free the C char*.
// If the input is NULL, then return nil.
func CError(cerr *C.char) error {
	if cerr != nil {
		err := errors.New(C.GoString(cerr))
		C.free(unsafe.Pointer(cerr))
		return err
	}
	return nil
}
