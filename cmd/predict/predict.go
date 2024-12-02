package main

import (
	"fmt"
	"os"

	"github.com/akamensky/argparse"
	"github.com/bmharper/cimg/v2"
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
	parser := argparse.NewParser("predict", "Run prediction on a single image")
	input := parser.String("i", "input", &argparse.Options{Help: "Input image file", Required: true})
	modelName := parser.String("m", "model", &argparse.Options{Help: "Model name (eg yolov8m)", Required: true})
	enableAccel := parser.Flag("", "accel", &argparse.Options{Help: "Enable hardware accelerators (eg hailo)", Required: false, Default: true})
	nnWidth := parser.Int("", "width", &argparse.Options{Help: "NN width", Required: false, Default: 640})
	nnHeight := parser.Int("", "height", &argparse.Options{Help: "NN height", Required: false, Default: 480})
	err := parser.Parse(os.Args)
	if err != nil {
		fmt.Print(parser.Usage(err))
		os.Exit(1)
	}

	logger, _ := logs.NewLog()

	var device *nnaccel.Device
	nnload.LoadAccelerators(logger, true)
	accel := nnload.Accelerator()
	if accel != nil && *enableAccel {
		device, err = accel.OpenDevice()
		check(err)
		defer device.Close()
	}

	modelSetup := nn.NewModelSetup()
	modelSetup.BatchSize = 1
	params := nn.NewDetectionParams()

	model, err := nnload.LoadModel(logger, device, "models", *modelName, *nnWidth, *nnHeight, nn.ThreadingModeParallel, modelSetup)
	check(err)

	img, err := cimg.ReadFile(*input)
	check(err)
	img = img.ToRGB()

	batch := nn.MakeImageBatchSingle(img.Width, img.Height, 3, img.Stride, img.Pixels)
	batchResult, err := model.DetectObjects(batch, params)
	check(err)

	for i, obj := range batchResult[0] {
		cls := model.Config().Classes[obj.Class]
		fmt.Printf("Object %v: %v, %.2f%%, (%v, %v) - (%v, %v)\n", i, cls, obj.Confidence*100, obj.Box.X, obj.Box.Y, obj.Box.X2(), obj.Box.Y2())
	}

	if len(batchResult[0]) == 0 {
		fmt.Printf("No objects detected\n")
	}
}
