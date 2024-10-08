package nnload

// Package nnload wraps up our 'nn' interface layer, and has concrete references to our
// neural network implementation (eg ncnn), so that you can just call one function to
// load a model, and not need to know about the implementation details.
//
// This is also the place where we detect the presence of an NN accelerator (eg Hailo),
// and then use that if it is available.

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cyclopcam/cyclops/pkg/ncnn"
	"github.com/cyclopcam/cyclops/pkg/nn"
	"github.com/cyclopcam/cyclops/pkg/nnaccel"
	"github.com/cyclopcam/logs"
)

// If not nil, then we have successfully loaded the Hailo AI accelerator module
var hailoAccel *nnaccel.Accelerator

// Return true if we are using a hardware NN accelerator
func HaveAccelerator() bool {
	return HaveHailo()
}

// Return true if we have a Hailo accelerator
func HaveHailo() bool {
	return hailoAccel != nil
}

// LoadModel loads a neural network from disk.
// If the model consists of several files, then filenameBase is the base filename, without the extensions.
func LoadModel(logs logs.Log, modelDir, filenameBase string, threadingMode nn.ThreadingMode, modelSetup *nn.ModelSetup) (nn.ObjectDetector, error) {
	fullPathBase := filepath.Join(modelDir, filenameBase)
	config, err := nn.LoadModelConfig(fullPathBase + ".json")
	if err != nil {
		return nil, err
	}

	if hailoAccel != nil {
		model, err := hailoAccel.LoadModel(modelDir, filenameBase, modelSetup)
		if err == nil {
			return model, nil
		} else {
			logs.Warnf("Failed to load Hailo accelerated NN model '%v': %v", filenameBase, err)
			logs.Infof("Falling back to ncnn")
		}
	}

	_, eparam := os.Stat(fullPathBase + ".param")
	_, ebin := os.Stat(fullPathBase + ".bin")

	if eparam == nil && ebin == nil {
		// NCNN file
		return ncnn.NewDetector(config, threadingMode, fullPathBase+".param", fullPathBase+".bin")
	} else {
		return nil, fmt.Errorf("Unrecognized NN model type %v", fullPathBase)
	}
}

func LoadAccelerators(logs logs.Log, enableHailo bool) {
	logs.Infof("Loading NN accelerators")
	var err error
	if enableHailo {
		hailoAccel, err = nnaccel.Load("hailo")
		if err != nil {
			logs.Infof("Failed to load Hailo NN accelerator: %v", err)
		} else {
			logs.Infof("Loaded Hailo NN accelerator")
		}
	} else {
		logs.Infof("Hailo disabled - skipping")
	}
}
