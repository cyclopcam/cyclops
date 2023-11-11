package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/akamensky/argparse"
	"github.com/cyclopcam/cyclops/pkg/nn"
	"github.com/cyclopcam/cyclops/pkg/nnload"
	"github.com/cyclopcam/cyclops/pkg/videox"
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
	minSize := parser.Int("s", "size", &argparse.Options{Help: "Minimum size of object, in pixels", Required: true})
	classes := parser.String("c", "classes", &argparse.Options{Help: "Comma-separated list of named classes to detect", Required: true})
	modelFile := parser.String("m", "model", &argparse.Options{Help: "Path to NN model file", Required: true})
	err := parser.Parse(os.Args)
	if err != nil {
		fmt.Print(parser.Usage(err))
		os.Exit(1)
	}

	model, err := nnload.LoadModel(*modelFile, nn.ThreadingModeParallel)
	check(err)

	modelConfig := model.Config()

	// Build a dictionary of the class indices that we're interested in
	nnClassToIndex := map[string]int{}
	for i, class := range modelConfig.Classes {
		nnClassToIndex[class] = i
	}

	outputClassNames := strings.Split(*classes, ",")
	nnClassToOutputClass := map[int]int{}

	for iOut, class := range outputClassNames {
		iIn, ok := nnClassToIndex[class]
		if !ok {
			panic(fmt.Sprintf("Class '%v' not found in model\n", class))
		}
		nnClassToOutputClass[iIn] = iOut
	}

	decoder, err := videox.NewH264FileDecoder(*input)
	check(err)

	nnParams := nn.NewDetectionParams()

	videoLabels := nn.VideoLabels{
		Classes: outputClassNames,
	}

	//for i := 0; i < 1000; i++ {
	//	_, err = decoder.NextFrame()
	//	fmt.Printf("decode: %v\n", err)
	//}

	frameIdx := 0
	for {
		frame, err := decoder.NextFrame()
		if errors.Is(err, videox.ErrResourceTemporarilyUnavailable) {
			continue
		}
		if errors.Is(err, io.EOF) {
			break
		}
		//if frameIdx > 10 {
		//	break
		//}
		check(err)
		frameIdx++
		//fmt.Printf("%v,", frameIdx)
		rgb := frame.ToCImageRGB()
		img := nn.WholeImage(3, rgb.Pixels, rgb.Width, rgb.Height)
		objects, err := nn.TiledInference(model, img, nnParams, 1)
		check(err)

		frameLabels := &nn.ImageLabels{
			Frame: frameIdx,
		}
		for _, obj := range objects {
			outClass, ok := nnClassToOutputClass[obj.Class]
			if ok &&
				obj.Box.Width >= *minSize &&
				obj.Box.Height >= *minSize {
				obj.Class = outClass
				frameLabels.Objects = append(frameLabels.Objects, obj)
			}
		}
		if len(frameLabels.Objects) != 0 {
			videoLabels.Frames = append(videoLabels.Frames, frameLabels)
		}
		fmt.Printf("%v: %v\n", frameIdx, frameLabels.Objects)
	}

	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(videoLabels)
	check(err)
}
