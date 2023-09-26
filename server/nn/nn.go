package nn

// nn is a Neural Network interface layer

// Detection is an object that a neural network has found in an image
type Detection struct {
	Class      int     `json:"class"`
	Confidence float32 `json:"confidence"`
	Box        Rect    `json:"box"`
}

type DetectionResult struct {
	CameraID    int64       `json:"cameraID"`
	ImageWidth  int         `json:"imageWidth"`
	ImageHeight int         `json:"imageHeight"`
	Objects     []Detection `json:"objects"`
}

type DetectionParams struct {
	ProbabilityThreshold float32 // Value between 0 and 1. Lower values will find more objects. Zero value will use the default.
	NmsThreshold         float32 // Value between 0 and 1. Lower values will merge more objects together into one. Zero value will use the default.
}

func DefaultDetectionParams() *DetectionParams {
	return &DetectionParams{
		ProbabilityThreshold: 0.5,
		NmsThreshold:         0.45,
	}
}

// ObjectDetector is given an image, and returns zero or more detected objects
type ObjectDetector interface {
	// Close closes the detector (you MUST call this when finished, because it's a C++ object underneath)
	Close()
	// DetectObjects returns a list of objects detected in the image
	DetectObjects(nchan int, image []byte, width, height int, params *DetectionParams) ([]Detection, error)
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
