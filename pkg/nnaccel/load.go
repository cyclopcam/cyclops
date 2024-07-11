package nnaccel

// #include "interface.h"
import "C"
import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"unsafe"
)

type NNAccel struct {
	handle unsafe.Pointer
}

type Model struct {
	module *NNAccel
	handle unsafe.Pointer
}

type ModelSetup struct {
	BatchSize int
}

func Load(accelName string) (*NNAccel, error) {
	tryPaths := []string{
		// When we get to binary deployment time, then we'll figure out where to place
		// our loadable libraries.
		"nnaccel/hailo/bin/", // relative path from the source code root.
	}
	allErrors := strings.Builder{}
	for _, dir := range tryPaths {
		m := NNAccel{}
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

func (m *NNAccel) LoadModel(filename string, setup *ModelSetup) (*Model, error) {
	model := Model{
		module: m,
	}
	cFilename := C.CString(filename)
	cSetup := C.NNModelSetup{
		BatchSize: C.int(setup.BatchSize),
	}
	err := m.StatusToErr(C.NALoadModel(m.handle, cFilename, &cSetup, &model.handle))
	C.free(unsafe.Pointer(cFilename))
	if err != nil {
		return nil, err
	}
	return &model, nil
}

func (m *NNAccel) StatusToErr(status C.int) error {
	if status != 0 {
		return errors.New(C.GoString(C.NAStatusStr(m.handle, status)))
	}
	return nil

}

func (m *Model) Close() {
	C.NACloseModel(m.module.handle, m.handle)
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
