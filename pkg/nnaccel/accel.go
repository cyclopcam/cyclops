package nnaccel

// #include "interface.h"
import "C"
import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"github.com/cyclopcam/cyclops/pkg/nn"
)

type NNAccel struct {
	handle unsafe.Pointer
}

type Model struct {
	accel  *NNAccel       // The accelerator that ran the job
	handle unsafe.Pointer // Handle to the model
}

type ModelSetup struct {
	BatchSize int
}

type AsyncJob struct {
	accel  *NNAccel       // The accelerator that ran the job
	handle unsafe.Pointer // Handle to the job
}

// Load an NN accelerator.
// At present, the only accelerator we have is "hailo"
func Load(accelName string) (*NNAccel, error) {
	cwd, _ := os.Getwd()
	//fmt.Printf("cwd = %v\n", cwd)

	// relative path from the source code root
	srcCodeRelPath := "nnaccel/hailo/bin"

	if strings.HasSuffix(cwd, "/nnaccel/hailo/test") {
		// We're being run as a Go unit test
		srcCodeRelPath = "../bin"
	}

	tryPaths := []string{
		// When we get to binary deployment time, then we'll figure out where to place
		// our loadable libraries.
		srcCodeRelPath, // relative path from the source code root.
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
		accel: m,
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
	C.NACloseModel(m.accel.handle, m.handle)
}

func (m *Model) Run(batchSize, width, height, nchan int, data unsafe.Pointer) (*AsyncJob, error) {
	job := &AsyncJob{
		accel: m.accel,
	}
	err := m.accel.StatusToErr(C.NARunModel(m.accel.handle, m.handle, C.int(batchSize), C.int(width), C.int(height), C.int(nchan), data, &job.handle))
	if err != nil {
		return nil, err
	}
	return job, nil
}

// Returns true if the job is finished
func (j *AsyncJob) Wait(wait time.Duration) bool {
	milliseconds := wait / time.Millisecond
	if milliseconds > math.MaxInt32 {
		milliseconds = math.MaxInt32
	}
	return C.NAWaitForJob(j.accel.handle, j.handle, C.uint32_t(milliseconds)) == 0
}

func (j *AsyncJob) GetObjectDetections() ([]nn.ObjectDetection, error) {
	maxDetections := 1000
	var detections *C.NNAObjectDetection
	var numDetections C.size_t
	C.NAGetObjectDetections(j.accel.handle, j.handle, C.size_t(maxDetections), &detections, &numDetections)
	dets := unsafe.Slice(detections, int(numDetections))
	out := make([]nn.ObjectDetection, len(dets))
	for i := 0; i < len(dets); i++ {
		out[i].Class = int(dets[i].ClassID)
		out[i].Confidence = float32(dets[i].Confidence)
		out[i].Box.X = int(dets[i].X)
		out[i].Box.Y = int(dets[i].Y)
		out[i].Box.Width = int(dets[i].Width)
		out[i].Box.Height = int(dets[i].Height)
	}
	C.free(unsafe.Pointer(detections))
	return out, nil
}

func (j *AsyncJob) Close() {
	//fmt.Printf("fooXXX\n")
	C.NACloseJob(j.accel.handle, j.handle)
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
