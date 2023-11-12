package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/akamensky/argparse"
	"github.com/bmharper/cimg/v2"
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

	frameIdx := -1
	for {
		frame, err := decoder.NextFrame()
		if errors.Is(err, videox.ErrResourceTemporarilyUnavailable) {
			continue
		}
		if errors.Is(err, io.EOF) {
			break
		}
		check(err)
		frameIdx++
		if *endFrame > 0 && frameIdx > *endFrame {
			break
		}
		if frameIdx < *startFrame {
			continue
		}
		//fmt.Printf("%v,", frameIdx)
		rgb := frame.ToCImageRGB()

		if rgb.Height > *maxVideoHeight && *maxVideoHeight > 0 {
			aspect := float64(rgb.Width) / float64(rgb.Height)
			newHeight := *maxVideoHeight
			newWidth := int(float64(newHeight)*aspect + 0.5)
			rgb = cimg.ResizeNew(rgb, newWidth, newHeight)
		}

		// assume all frames are the same size
		videoLabels.Width = rgb.Width
		videoLabels.Height = rgb.Height

		img := nn.WholeImage(3, rgb.Pixels, rgb.Width, rgb.Height)
		objects, err := nn.TiledInference(model, img, nnParams, 1)
		check(err)

		frameLabels := &nn.ImageLabels{
			Frame: frameIdx,
		}
		for _, obj := range objects {
			outClass, ok := nnClassToOutputClass[obj.Class]
			if ok &&
				(obj.Box.Width >= *minSize || obj.Box.Height >= *minSize) {
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
