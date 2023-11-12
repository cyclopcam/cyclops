package nn

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
	"unsafe"
)

// Package nn is a Neural Network interface layer
// To load a model, use the nnload package.

// Results of an NN object detection run
type DetectionResult struct {
	CameraID    int64             `json:"cameraID"`
	ImageWidth  int               `json:"imageWidth"`
	ImageHeight int               `json:"imageHeight"`
	Objects     []ObjectDetection `json:"objects"`
}

// NN object detection parameters
type DetectionParams struct {
	ProbabilityThreshold float32 // Value between 0 and 1. Lower values will find more objects. Zero value will use the default.
	NmsThreshold         float32 // Value between 0 and 1. Lower values will merge more objects together into one. Zero value will use the default.
	Unclipped            bool    // If true, don't clip boxes to the neural network boundaries
}

// Create a default DetectionParams object
func NewDetectionParams() *DetectionParams {
	return &DetectionParams{
		ProbabilityThreshold: 0.5,
		NmsThreshold:         0.45,
		Unclipped:            false,
	}
}

// ImageCrop is a crop of an image.
// In C we would represent this as a pointer and a stride, but since that's not memory safe,
// we must resort to this kind of thing. Once we get into the C world for NN inference, then
// we can use strides etc.
type ImageCrop struct {
	NChan       int    // Number of channels (eg 3 for RGB)
	Pixels      []byte // The whole image
	ImageWidth  int    // The width of the original image, held in Pixels
	ImageHeight int    // The height of the original image, held in Pixels
	CropX       int    // Origin of crop X
	CropY       int    // Origin of crop Y
	CropWidth   int    // The width of this crop
	CropHeight  int    // The height of this crop
}

// Return a pointer to the start of the crop
func (c ImageCrop) Pointer() unsafe.Pointer {
	ptr := unsafe.Pointer(&c.Pixels[0])
	ptr = unsafe.Add(ptr, (c.CropY*c.ImageWidth+c.CropX)*c.NChan)
	return ptr
}

func (c ImageCrop) Stride() int {
	return c.ImageWidth * c.NChan
}

// Return a crop of the crop (new crop is relative to existing).
// If any parameter is out of bounds, we panic
func (c ImageCrop) Crop(x1, y1, x2, y2 int) ImageCrop {
	nc := ImageCrop{
		NChan:       c.NChan,
		Pixels:      c.Pixels,
		ImageWidth:  c.ImageWidth,
		ImageHeight: c.ImageHeight,
		CropX:       c.CropX + x1,
		CropY:       c.CropY + y1,
		CropWidth:   x2 - x1,
		CropHeight:  y2 - y1,
	}
	if nc.CropX < 0 || nc.CropY < 0 || nc.CropWidth < 0 || nc.CropHeight < 0 || nc.CropX+nc.CropWidth > c.ImageWidth || nc.CropY+nc.CropHeight > c.ImageHeight {
		panic("Crop out of bounds")
	}
	return nc
}

// Return a 'crop' of the entire image
func WholeImage(nchan int, pixels []byte, width, height int) ImageCrop {
	return ImageCrop{
		NChan:       nchan,
		Pixels:      pixels,
		ImageWidth:  width,
		ImageHeight: height,
		CropX:       0,
		CropY:       0,
		CropWidth:   width,
		CropHeight:  height,
	}
}

type ThreadingMode int

const (
	ThreadingModeSingle   ThreadingMode = iota // Force the NN library to run inference on a single thread
	ThreadingModeParallel                      // Allow the NN library to run multiple threads while executing a model
)

// ObjectDetector is given an image, and returns zero or more detected objects
type ObjectDetector interface {
	// Close closes the detector (you MUST call this when finished, because it's a C++ object underneath)
	Close()

	// DetectObjects returns a list of objects detected in the image
	// nchan is expected to be 3, and image is a 24-bit RGB image.
	// You can create a default DetectionParams with NewDetectionParams()
	DetectObjects(img ImageCrop, params *DetectionParams) ([]ObjectDetection, error)

	// Model Config
	// Don't alter the model config after creating the detector.
	Config() *ModelConfig
}

// ModelConfig is saved in a JSON file along with the weights of the NN model
type ModelConfig struct {
	Architecture string   `json:"architecture"` // eg "yolov8"
	Width        int      `json:"width"`        // eg 320
	Height       int      `json:"height"`       // eg 256
	Classes      []string `json:"classes"`      // eg ["person", "bicycle", "car", ...]
}

// Load model config from a JSON file
func LoadModelConfig(filename string) (*ModelConfig, error) {
	b, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	config := &ModelConfig{}
	err = json.Unmarshal(b, config)
	if err != nil {
		return nil, err
	}
	return config, nil
}

// Load a text file with class names on each line
func LoadClassFile(filename string) ([]string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	classes := []string{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			classes = append(classes, line)
		}
	}
	return classes, nil
}

const (
	COCOPerson     = 0
	COCOBicycle    = 1
	COCOCar        = 2
	COCOMotorcycle = 3
	COCOAirplane   = 4
	COCOBus        = 5
	COCOTrain      = 6
	COCOTruck      = 7
)

// COCO classes
var COCOClasses = []string{
	"person",
	"bicycle",
	"car",
	"motorcycle",
	"airplane",
	"bus",
	"train",
	"truck",
	"boat",
	"traffic light",
	"fire hydrant",
	"stop sign",
	"parking meter",
	"bench",
	"bird",
	"cat",
	"dog",
	"horse",
	"sheep",
	"cow",
	"elephant",
	"bear",
	"zebra",
	"giraffe",
	"backpack",
	"umbrella",
	"handbag",
	"tie",
	"suitcase",
	"frisbee",
	"skis",
	"snowboard",
	"sports ball",
	"kite",
	"baseball bat",
	"baseball glove",
	"skateboard",
	"surfboard",
	"tennis racket",
	"bottle",
	"wine glass",
	"cup",
	"fork",
	"knife",
	"spoon",
	"bowl",
	"banana",
	"apple",
	"sandwich",
	"orange",
	"broccoli",
	"carrot",
	"hot dog",
	"pizza",
	"donut",
	"cake",
	"chair",
	"couch",
	"potted plant",
	"bed",
	"dining table",
	"toilet",
	"tv",
	"laptop",
	"mouse",
	"remote",
	"keyboard",
	"cell phone",
	"microwave",
	"oven",
	"toaster",
	"sink",
	"refrigerator",
	"book",
	"clock",
	"vase",
	"scissors",
	"teddy bear",
	"hair drier",
	"toothbrush",
}
