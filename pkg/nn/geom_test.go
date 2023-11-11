package nn

import (
	"testing"
)

func TestIOU(t *testing.T) {
	a := Rect{
		X:      0,
		Y:      0,
		Width:  10,
		Height: 10,
	}
	b := Rect{
		X:      5,
		Y:      5,
		Width:  10,
		Height: 10,
	}
	if a.IOU(b) != 0.25/(0.75+1) {
		t.Errorf("IOU is %v, not 0.25", a.IOU(b))
	}
}
