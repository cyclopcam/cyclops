package nn

// VideoLabels contains labels for each video frame
type VideoLabels struct {
	Classes []string       `json:"classes"`
	Frames  []*ImageLabels `json:"frames"`
	Width   int            `json:"width"`  // Image width. Useful when inference is run at different resolution to original image
	Height  int            `json:"height"` // Image height. Useful when inference is run at different resolution to original image
}

type ImageLabels struct {
	Frame   int               `json:"frame,omitempty"` // For video, this is the frame number
	Objects []ObjectDetection `json:"objects"`
}

// ObjectDetection is an object that a neural network has found in an image
type ObjectDetection struct {
	Class         int     `json:"class"`
	ConcreteClass int     `json:"concreteClass"` // If this is an abstract object (eg vehicle), then concreteClass is the class of the original object (eg car, truck)
	Confidence    float32 `json:"confidence"`
	Box           Rect    `json:"box"`
}
