package nn

// VideoLabels contains labels for each video frame
type VideoLabels struct {
	Classes []string       `json:"classes"`
	Frames  []*ImageLabels `json:"frames"`
}

type ImageLabels struct {
	Frame   int               `json:"frame,omitempty"` // For video, this is the frame number
	Objects []ObjectDetection `json:"objects"`
}

// ObjectDetection is an object that a neural network has found in an image
type ObjectDetection struct {
	Class      int     `json:"class"`
	Confidence float32 `json:"confidence"`
	Box        Rect    `json:"box"`
}
