package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/akamensky/argparse"
	"github.com/cyclopcam/cyclops/pkg/nn"
	"github.com/cyclopcam/cyclops/pkg/nnload"
	"github.com/cyclopcam/logs"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	parser := argparse.NewParser("predict", "Label a video stream")
	input := parser.String("i", "input", &argparse.Options{Help: "Input video file", Required: true})
	output := parser.File("o", "output", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0664, &argparse.Options{Help: "Output label file", Required: true})
	minSize := parser.Int("m", "minsize", &argparse.Options{Help: "Minimum size of object, in pixels", Required: true})
	maxVideoHeight := parser.Int("", "vheight", &argparse.Options{Help: "If video height is larger than this, then scale it down to this size", Required: false, Default: 0})
	startFrame := parser.Int("", "startframe", &argparse.Options{Help: "Start processing at frame", Required: false, Default: 0})
	endFrame := parser.Int("", "endframe", &argparse.Options{Help: "Stop processing at frame", Required: false, Default: 0})
	classes := parser.String("c", "classes", &argparse.Options{Help: "Comma-separated list of named classes to detect", Required: true})
	modelFile := parser.String("n", "model", &argparse.Options{Help: "Path to NN model file", Required: true})
	err := parser.Parse(os.Args)
	if err != nil {
		fmt.Print(parser.Usage(err))
		os.Exit(1)
	}

	logger, _ := logs.NewLog()

	options := nn.InferenceOptions{
		MinSize:        *minSize,
		MaxVideoHeight: *maxVideoHeight,
		StartFrame:     *startFrame,
		EndFrame:       *endFrame,
		Classes:        strings.Split(*classes, ","),
		StdOutProgress: true,
	}

	model, err := nnload.LoadModel(logger, filepath.Dir(*modelFile), filepath.Base(*modelFile), nn.ThreadingModeParallel, nn.NewModelSetup())
	check(err)

	videoLabels, err := nn.RunInferenceOnVideoFile(model, *input, options)
	check(err)

	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(videoLabels)
	check(err)
}
