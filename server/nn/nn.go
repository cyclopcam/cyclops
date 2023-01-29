package nn

// nn is a Neural Network interface layer

type Rect struct {
	X      int
	Y      int
	Width  int
	Height int
}

// Detection is an object that a neural network has found in an image
type Detection struct {
	Class      int
	Confidence float32
	Box        Rect
}

// ObjectDetector is given an image, and returns zero or more detected objects
type ObjectDetector interface {
	// Close closes the detector (you MUST call this when finished, because it's a C++ object)
	Close()
	// DetectObjects returns a list of objects detected in the image
	DetectObjects(image []byte) ([]Detection, error)
}
