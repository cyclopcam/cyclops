package nn

import (
	"github.com/chewxy/math32"
	"github.com/cyclopcam/cyclops/pkg/gen"
)

type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

func (p Point) Distance(b Point) float32 {
	return math32.Sqrt(float32((p.X-b.X)*(p.X-b.X) + (p.Y-b.Y)*(p.Y-b.Y)))
}

type Rect struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

func (r Rect) Area() int {
	return r.Width * r.Height
}

func (r Rect) Intersection(b Rect) Rect {
	x1 := gen.Max(r.X, b.X)
	y1 := gen.Max(r.Y, b.Y)
	x2 := gen.Min(r.X+r.Width, b.X+b.Width)
	y2 := gen.Min(r.Y+r.Height, b.Y+b.Height)
	return Rect{
		X:      x1,
		Y:      y1,
		Width:  gen.Max(0, x2-x1),
		Height: gen.Max(0, y2-y1),
	}
}

func (r Rect) Union(b Rect) Rect {
	x1 := gen.Min(r.X, b.X)
	y1 := gen.Min(r.Y, b.Y)
	x2 := gen.Max(r.X+r.Width, b.X+b.Width)
	y2 := gen.Max(r.Y+r.Height, b.Y+b.Height)
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
