package nnload

import (
	"fmt"
	"os"

	"github.com/cyclopcam/cyclops/pkg/ncnn"
	"github.com/cyclopcam/cyclops/pkg/nn"
)

// Package nnload wraps up our 'nn' interface layer, and has concrete references to our
// neural network implementation (eg ncnn), so that you can just call one function to
// load a model, and not need to know about the implementation details.

// LoadModel loads a neural network from disk.
// If the model consists of several files, then filenameBase is the base filename, without the extensions.
func LoadModel(filenameBase string, threadingMode nn.ThreadingMode) (nn.ObjectDetector, error) {
	config, err := nn.LoadModelConfig(filenameBase + ".json")
	if err != nil {
		return nil, err
	}

	_, eparam := os.Stat(filenameBase + ".param")
	_, ebin := os.Stat(filenameBase + ".bin")

	if eparam == nil && ebin == nil {
		// NCNN file
		return ncnn.NewDetector(config, threadingMode, filenameBase+".param", filenameBase+".bin")
	} else {
		return nil, fmt.Errorf("Unrecognized NN model type %v", filenameBase)
	}
}
