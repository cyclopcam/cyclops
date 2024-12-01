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

type Device struct {
	accelerator *Accelerator
	handle      unsafe.Pointer
}

// Load an NN accelerator.
// At present, the only accelerator we have is "hailo"
func Load(accelName string) (*Accelerator, error) {
	cwd, _ := os.Getwd()
	//fmt.Printf("nnaccel.Load cwd = %v\n", cwd)

	// relative path from the source code root
	srcCodeRelPath := "nnaccel/hailo/bin"

	if strings.HasSuffix(cwd, "/nnaccel/hailo/test") {
		// We're being run as a Go unit test inside nnaccel/hailo/test
		srcCodeRelPath = "../bin"
	} else if strings.HasSuffix(cwd, "/server/test") {
		// We're being run as a Go unit test inside server/test
		srcCodeRelPath = "../../nnaccel/hailo/bin"
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
			if strings.Index(err.Error(), fullPath) != -1 {
				// If the error contains the full path, then don't add it again.
				fmt.Fprintf(&allErrors, "%v\n", err)
			} else {
				fmt.Fprintf(&allErrors, "Loading %v: %v\n", fullPath, err)
			}
		} else {
			return &m, nil
		}
	}
	return nil, errors.New(strings.TrimRight(allErrors.String(), "\n"))
}

// Open a new device (eg a handle to a GPU or a Hailo accelerator)
// A device must be closed after using.
func (m *Accelerator) OpenDevice() (*Device, error) {
	var device unsafe.Pointer
	err := m.StatusToErr(C.NAOpenDevice(m.handle, &device))
	if err != nil {
		return nil, err
	}
	return &Device{accelerator: m, handle: device}, nil
}

// Close a device
func (d *Device) Close() {
	C.NACloseDevice(d.accelerator.handle, d.handle)
}

func (d *Device) ModelFiles() (subdir string, ext []string) {
	var cSubDir *C.char
	var cExt *C.char
	C.NAModelFiles(d.accelerator.handle, d.handle, &cSubDir, &cExt)
	subdir = C.GoString(cSubDir)
	ext = []string{C.GoString(cExt)}
	return
}

func (d *Device) LoadModel(filename string, setup *nn.ModelSetup) (*Model, error) {
	model := Model{
		device: d,
	}
	cFilename := C.CString(filename)
	cSetup := C.NNModelSetup{
		BatchSize:            C.int(setup.BatchSize),
		ProbabilityThreshold: C.float(setup.ProbabilityThreshold),
		NmsIouThreshold:      C.float(setup.NmsIouThreshold),
	}
	err := d.accelerator.StatusToErr(C.NALoadModel(d.accelerator.handle, d.handle, cFilename, &cSetup, &model.handle))
	C.free(unsafe.Pointer(cFilename))
	if err != nil {
		return nil, err
	}

	var info C.NNModelInfo
	C.NAModelInfo(d.accelerator.handle, model.handle, &info)
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
