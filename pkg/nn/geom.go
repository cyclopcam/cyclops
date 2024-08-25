package nn

import (
	"github.com/chewxy/math32"
	"github.com/cyclopcam/cyclops/pkg/gen"
)

type Point struct {
	X int32 `json:"x"`
	Y int32 `json:"y"`
}

func (p Point) Distance(b Point) float32 {
	return math32.Sqrt(float32((p.X-b.X)*(p.X-b.X) + (p.Y-b.Y)*(p.Y-b.Y)))
}

type Rect struct {
	X      int32 `json:"x"`
	Y      int32 `json:"y"`
	Width  int32 `json:"width"`
	Height int32 `json:"height"`
}

func MakeRect(x, y, width, height int) Rect {
	return Rect{
		X:      int32(x),
		Y:      int32(y),
		Width:  int32(width),
		Height: int32(height),
	}
}

func (r Rect) X2() int32 {
	return r.X + r.Width
}

func (r Rect) Y2() int32 {
	return r.Y + r.Height
}

func (r Rect) Area() int {
	return int(r.Width * r.Height)
}

func (r Rect) Intersection(b Rect) Rect {
	x1 := max(r.X, b.X)
	y1 := max(r.Y, b.Y)
	x2 := min(r.X+r.Width, b.X+b.Width)
	y2 := min(r.Y+r.Height, b.Y+b.Height)
	return Rect{
		X:      x1,
		Y:      y1,
		Width:  max(0, x2-x1),
		Height: max(0, y2-y1),
	}
}

func (r Rect) Union(b Rect) Rect {
	x1 := min(r.X, b.X)
	y1 := min(r.Y, b.Y)
	x2 := max(r.X+r.Width, b.X+b.Width)
	y2 := max(r.Y+r.Height, b.Y+b.Height)
	return Rect{
		X:      x1,
		Y:      y1,
		Width:  x2 - x1,
		Height: y2 - y1,
	}
}

// Intersection over Union
func (r Rect) IOU(b Rect) float32 {
	intersection := r.Intersection(b)
	return float32(intersection.Area()) / float32(r.Area()+b.Area()-intersection.Area())
}

func (r Rect) Center() Point {
	return Point{
		X: r.X + r.Width/2,
		Y: r.Y + r.Height/2,
	}
}

func (r *Rect) Offset(dx, dy int) {
	r.X += int32(dx)
	r.Y += int32(dy)
}

func (r *Rect) MaxDelta(b Rect) int {
	maxP := max(gen.Abs(r.X-b.X), gen.Abs(r.Y-b.Y))
	maxS := max(gen.Abs(r.Width-b.Width), gen.Abs(r.Height-b.Height))
	return int(max(maxP, maxS))
}

// ResizeTransform expresses a transformation that we've made on an image (eg resizing, or resizing + moving)
// When applying forward, we first scale and then offset.
type ResizeTransform struct {
	OffsetX int32
	OffsetY int32
	ScaleX  float32
	ScaleY  float32
}

func IdentityResizeTransform() ResizeTransform {
	return ResizeTransform{
		OffsetX: 0,
		OffsetY: 0,
		ScaleX:  1,
		ScaleY:  1,
	}
}

func (r *ResizeTransform) ApplyForward(detections []ObjectDetection) {
	for i := range detections {
		detections[i].Box.X = int32(float32(detections[i].Box.X)*r.ScaleX) + r.OffsetX
		detections[i].Box.Y = int32(float32(detections[i].Box.Y)*r.ScaleY) + r.OffsetY
		detections[i].Box.Width = int32(float32(detections[i].Box.Width) * r.ScaleX)
		detections[i].Box.Height = int32(float32(detections[i].Box.Height) * r.ScaleY)
	}
}

func (r *ResizeTransform) ApplyBackward(detections []ObjectDetection) {
	for i := range detections {
		detections[i].Box.X = int32(float32(detections[i].Box.X-r.OffsetX) / r.ScaleX)
		detections[i].Box.Y = int32(float32(detections[i].Box.Y-r.OffsetY) / r.ScaleY)
		detections[i].Box.Width = int32(float32(detections[i].Box.Width) / r.ScaleX)
		detections[i].Box.Height = int32(float32(detections[i].Box.Height) / r.ScaleY)
	}
}
