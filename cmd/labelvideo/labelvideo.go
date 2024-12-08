package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/akamensky/argparse"
	"github.com/cyclopcam/cyclops/pkg/nn"
	"github.com/cyclopcam/cyclops/pkg/nnaccel"
	"github.com/cyclopcam/cyclops/pkg/nnload"
	"github.com/cyclopcam/logs"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	parser := argparse.NewParser("labelvideo", "Label a video")
	input := parser.String("i", "input", &argparse.Options{Help: "Input video file", Required: true})
	output := parser.File("o", "output", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0664, &argparse.Options{Help: "Output label file", Required: true})
	minSize := parser.Int("m", "minsize", &argparse.Options{Help: "Minimum size of object, in pixels", Required: true})
	maxVideoHeight := parser.Int("", "vheight", &argparse.Options{Help: "If video height is larger than this, then scale it down to this size", Required: false, Default: 0})
	startFrame := parser.Int("", "startframe", &argparse.Options{Help: "Start processing at frame", Required: false, Default: 0})
	endFrame := parser.Int("", "endframe", &argparse.Options{Help: "Stop processing at frame", Required: false, Default: 0})
	classes := parser.String("c", "classes", &argparse.Options{Help: "Comma-separated list of named classes to detect", Required: true})
	modelDir := parser.String("", "modeldir", &argparse.Options{Help: "Path to NN model dir", Required: false, Default: "models"})
	modelName := parser.String("n", "model", &argparse.Options{Help: "NN model name", Required: false, Default: "yolov8m"})
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
		StdOutStats:    true,
		NumThreads:     1, // useless right now - this is per-image threads
	}

	// nil device = NCNN
	var device *nnaccel.Device

	model, err := nnload.LoadModel(logger, device, *modelDir, *modelName, 640, 480, nn.ThreadingModeSingle, nn.NewModelSetup())
	check(err)

	videoLabels, err := nn.RunInferenceOnVideoFile(model, *input, options)
	check(err)

	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(videoLabels)
	check(err)
}
