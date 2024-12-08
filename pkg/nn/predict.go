package nn

import (
	"errors"
	"fmt"
	"io"

	"github.com/bmharper/cimg/v2"
	"github.com/cyclopcam/cyclops/pkg/videox"
)

// NN analysis options for RunInferenceOnVideoFile
type InferenceOptions struct {
	MinSize        int      // Minimum size of object, in pixels. If max(width, height) >= MinSize, then use the object
	MaxVideoHeight int      // If video height is larger than this, then scale it down to this size (0 = no scaling)
	StartFrame     int      // Start processing at frame (0 = start at beginning)
	EndFrame       int      // Stop processing at frame (0 = process to end)
	Classes        []string // List of class names to detect (eg ["person", "car", "bear"]). Classes not included in the list are ignored.
	StdOutProgress bool     // Emit progress to stdout
	StdOutStats    bool     // Print final stats to stdout
	NumThreads     int      // Number of threads performing inference
}

// Run NN inference on every frame of a video
func RunInferenceOnVideoFile(model ObjectDetector, inputFile string, options InferenceOptions) (*VideoLabels, error) {
	if len(options.Classes) == 0 {
		return nil, errors.New("No classes specified")
	}
	if options.NumThreads < 1 {
		options.NumThreads = 1
	}

	modelConfig := model.Config()

	// Build a dictionary of the class indices that we're interested in
	nnClassToIndex := map[string]int{}
	for i, class := range modelConfig.Classes {
		nnClassToIndex[class] = i
	}

	// How many times each class occurs
	nnClassCount := map[string]int{}

	nnClassToOutputClass := map[int]int{}

	for iOut, class := range options.Classes {
		iIn, ok := nnClassToIndex[class]
		if !ok {
			panic(fmt.Sprintf("Class '%v' not found in model\n", class))
		}
		nnClassToOutputClass[iIn] = iOut
	}

	decoder, err := videox.NewVideoFileDecoder(inputFile)
	if err != nil {
		return nil, err
	}

	nnParams := NewDetectionParams()

	videoLabels := VideoLabels{
		Classes: options.Classes,
	}

	//for i := 0; i < 1000; i++ {
	//	_, err = decoder.NextFrame()
	//	fmt.Printf("decode: %v\n", err)
	//}

	frameIdx := -1
	for {
		frame, err := decoder.NextFrame()
		// Since rewriting decoder in C++, I haven't implemented ErrResourceTemporarilyUnavailable.
		// If you need that, implement it now.
		//if errors.Is(err, videox.ErrResourceTemporarilyUnavailable) {
		//	continue
		//}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		frameIdx++
		if options.EndFrame > 0 && frameIdx > options.EndFrame {
			break
		}
		if frameIdx < options.StartFrame {
			continue
		}
		//fmt.Printf("%v,", frameIdx)
		rgb := frame.Image.ToCImageRGB()

		if rgb.Height > options.MaxVideoHeight && options.MaxVideoHeight > 0 {
			aspect := float64(rgb.Width) / float64(rgb.Height)
			newHeight := options.MaxVideoHeight
			newWidth := int(float64(newHeight)*aspect + 0.5)
			rgb = cimg.ResizeNew(rgb, newWidth, newHeight, nil)
		}

		// assume all frames are the same size
		videoLabels.Width = rgb.Width
		videoLabels.Height = rgb.Height

		img := WholeImage(rgb.NChan(), rgb.Pixels, rgb.Width, rgb.Height)
		objects, err := TiledInference(model, img, nnParams, options.NumThreads)
		if err != nil {
			return nil, err
		}

		frameLabels := &ImageLabels{
			Frame: frameIdx,
		}
		for _, obj := range objects {
			outClass, ok := nnClassToOutputClass[obj.Class]
			if ok &&
				(int(obj.Box.Width) >= options.MinSize || int(obj.Box.Height) >= options.MinSize) {
				obj.Class = outClass
				frameLabels.Objects = append(frameLabels.Objects, obj)
				nnClassCount[modelConfig.Classes[obj.Class]]++
			}
		}
		if len(frameLabels.Objects) != 0 {
			videoLabels.Frames = append(videoLabels.Frames, frameLabels)
		}
		if options.StdOutProgress {
			fmt.Printf("%v: %v\n", frameIdx, frameLabels.Objects)
		}
	}

	if options.StdOutStats {
		for key, val := range nnClassCount {
			fmt.Printf("%v: %v\n", key, val)
		}
	}

	return &videoLabels, nil
}
