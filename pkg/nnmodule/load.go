package nnmodule

// #include "interface.h"
import "C"
import (
	"errors"
	"unsafe"
)

type NNModule struct {
	handle unsafe.Pointer
}

type Model struct {
	module *NNModule
	handle unsafe.Pointer
}

type ModelSetup struct {
	BatchSize int
}

func Load(moduleName string) (*NNModule, error) {
	m := NNModule{}
	cModuleName := C.CString(moduleName)
	err := CError(C.LoadNNModule(cModuleName, &m.handle))
	C.free(unsafe.Pointer(cModuleName))
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (m *NNModule) LoadModel(filename string, setup *ModelSetup) (*Model, error) {
	model := Model{
		module: m,
	}
	cFilename := C.CString(filename)
	cSetup := C.NNModelSetup{
		BatchSize: C.int(setup.BatchSize),
	}
	err := m.StatusToErr(C.NMLoadModel(m.handle, cFilename, &cSetup, &model.handle))
	C.free(unsafe.Pointer(cFilename))
	if err != nil {
		return nil, err
	}
	return &model, nil
}

func (m *NNModule) StatusToErr(status C.int) error {
	if status != 0 {
		return errors.New(C.GoString(C.NMStatusStr(m.handle, status)))
	}
	return nil

}

func (m *Model) Close() {
	C.NMCloseModel(m.module.handle, m.handle)
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
