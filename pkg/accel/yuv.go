package accel

import (
	"fmt"
	"os"

	"github.com/bmharper/cimg/v2"
	"github.com/cyclopcam/cyclops/pkg/gen"
)

// Planar YUV 420 image
type YUVImage struct {
	Width  int
	Height int
	Y      []byte
	U      []byte
	V      []byte
}

// Infer our stride from the Y buffer size
func (x *YUVImage) YStride() int {
	return len(x.Y) / x.Height
}

// Infer our stride from the U buffer size
func (x *YUVImage) UStride() int {
	return len(x.U) / (x.Height / 2)
}

// Infer our stride from the V buffer size
func (x *YUVImage) VStride() int {
	return len(x.V) / (x.Height / 2)
}

// Transcode from YUV420p to RGB
func (x *YUVImage) ToCImageRGB() *cimg.Image {
	dst := cimg.NewImage(x.Width, x.Height, cimg.PixelFormatRGB)
	YUV420pToRGB(x.Width, x.Height, x.Y, x.U, x.V, x.YStride(), x.UStride(), x.VStride(), dst.Stride, dst.Pixels)
	return dst
}

// Transcode from YUV420p to RGB
// The target image must be the same size as the source, and RGB format
func (x *YUVImage) CopyToCImageRGB(dst *cimg.Image) {
	if dst.Width != x.Width || dst.Height != x.Height || dst.Format != cimg.PixelFormatRGB {
		panic("Destination image must be the same size as the source image, and PixelFormatRGB")
	}
	YUV420pToRGB(x.Width, x.Height, x.Y, x.U, x.V, x.YStride(), x.UStride(), x.VStride(), dst.Stride, dst.Pixels)
}

// Clone into a tightly packed YUV420p image
func (x *YUVImage) Clone() *YUVImage {
	dst := &YUVImage{
		Width:  x.Width,
		Height: x.Height,
		Y:      make([]byte, x.Width*x.Height),
		U:      make([]byte, x.Width*x.Height/4),
		V:      make([]byte, x.Width*x.Height/4),
	}
	dst.CopyFrom(x)
	return dst
}

func (x *YUVImage) CopyFrom(src *YUVImage) {
	width := gen.Min(x.Width, src.Width)
	height := gen.Min(x.Height, src.Height)
	srcYStride := src.YStride()
	srcUStride := src.UStride()
	srcVStride := src.VStride()
	dstYStride := x.YStride()
	dstUStride := x.UStride()
	dstVStride := x.VStride()
	for i := 0; i < height; i++ {
		copy(x.Y[i*dstYStride:], src.Y[i*srcYStride:i*srcYStride+width])
	}
	heightHalf := height / 2
	widthHalf := width / 2
	for i := 0; i < heightHalf; i++ {
		copy(x.U[i*dstUStride:], src.U[i*srcUStride:i*srcUStride+widthHalf])
	}
	for i := 0; i < heightHalf; i++ {
		copy(x.V[i*dstVStride:], src.V[i*srcVStride:i*srcVStride+widthHalf])
	}
}

func (x *YUVImage) TotalBytes() int {
	return len(x.Y) + len(x.U) + len(x.V)
}

// Load 3 raw YUV files into a YUVImage
func LoadYUVFiles(filenameY, filenameU, filenameV string, width, height int) (*YUVImage, error) {
	y, err := os.ReadFile(filenameY)
	if err != nil {
		return nil, err
	}
	u, err := os.ReadFile(filenameU)
	if err != nil {
		return nil, err
	}
	v, err := os.ReadFile(filenameV)
	if err != nil {
		return nil, err
	}
	// Assume it's 420p
	if len(y) != width*height {
		return nil, fmt.Errorf("Y buffer size is %v, expected %v", len(y), width*height)
	}
	if len(u) != width*height/4 {
		return nil, fmt.Errorf("U buffer size is %v, expected %v", len(u), width*height/4)
	}
	if len(v) != width*height/4 {
		return nil, fmt.Errorf("V buffer size is %v, expected %v", len(v), width*height/4)
	}
	return &YUVImage{
		Width:  width,
		Height: height,
		Y:      y,
		U:      u,
		V:      v,
	}, nil
}
