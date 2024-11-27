package monitor

import (
	"fmt"
	"testing"

	"github.com/bmharper/cimg/v2"
	"github.com/cyclopcam/cyclops/pkg/accel"
	"github.com/cyclopcam/cyclops/pkg/nn"
	"github.com/cyclopcam/cyclops/pkg/nnaccel"
	"github.com/cyclopcam/logs"
	"github.com/stretchr/testify/require"
)

type dummyDetector struct {
	ModelConfig nn.ModelConfig
}

func (d *dummyDetector) Close() {
}

func (d *dummyDetector) DetectObjects(batch nn.ImageBatch, params *nn.DetectionParams) ([][]nn.ObjectDetection, error) {
	return nil, nil
}

func (d *dummyDetector) Config() *nn.ModelConfig {
	return &d.ModelConfig
}

func testImageResizeForNNAt(t *testing.T, rgbWidth, rgbHeight, nnWidth, nnHeight int) {
	batchSize := 2
	batchStride := nnBatchImageStride(nnWidth, nnHeight)
	wholeBatchImage := nnaccel.PageAlignedAlloc(batchSize * batchStride)

	// Fill with non-black so that we can verify that our image prep is adding the
	// appropriate black padding on the bottom or right edge of the image.
	// You should expect to see black in the test image padding, not gray.
	for i := 0; i < len(wholeBatchImage); i++ {
		wholeBatchImage[i] = 190
	}

	m := Monitor{}
	m.Log = logs.NewTestingLog(t)
	m.detector = &dummyDetector{
		ModelConfig: nn.ModelConfig{
			Width:  nnWidth,
			Height: nnHeight,
		},
	}

	src, err := cimg.ReadFile("../../testdata/yard-640x640.jpg")
	require.NoError(t, err)

	// squash src so that it matches our test conditions
	// non-uniform scaling will be a bit confusing for tests...
	src = cimg.ResizeNew(src, rgbWidth, rgbHeight, nil)
	srcYUV := rgbToYUV420p(src.Pixels, src.Width, src.Height)

	for batchEl := 0; batchEl < batchSize; batchEl++ {
		// Change quality based on batch element, so you can flip between the two sample images and compare
		quality := ResizeQualityLow
		if batchEl == 1 {
			quality = ResizeQualityHigh
		}
		nnBlock := wholeBatchImage[batchEl*batchStride : (batchEl+1)*batchStride]
		//xformRgbToNN, rgbPure, rgbNN := m.prepareImageForNN(srcYUV, nnBlock)
		_, rgbPure, rgbNN := m.prepareImageForNN(srcYUV, nnBlock, quality)
		rgbPure.WriteJPEG(fmt.Sprintf("test-%02d-pure.jpg", batchEl), cimg.MakeCompressParams(cimg.Sampling444, 99, 0), 0644)
		rgbNN.WriteJPEG(fmt.Sprintf("test-%02d-nn.jpg", batchEl), cimg.MakeCompressParams(cimg.Sampling444, 99, 0), 0644)
	}
}

func TestImageResizeForNN(t *testing.T) {
	//                            rgb        nn
	testImageResizeForNNAt(t, 640, 480, 640, 640) // scale = 1, but aspect ratio is different. black padding on bottom
	testImageResizeForNNAt(t, 480, 640, 640, 640) // scale = 1, but aspect ratio is different. black padding on right
	testImageResizeForNNAt(t, 640, 480, 320, 256) // scale = 0.5, invoke SIMD library downscaling when filter quality is Low
	testImageResizeForNNAt(t, 640, 480, 360, 256) // scale ~= 0.5, invoke stbir
	testImageResizeForNNAt(t, 320, 240, 640, 640) // upscaling
	testImageResizeForNNAt(t, 640, 640, 640, 640) // 1:1
}

func rgbToYUV420p(rgb []byte, width, height int) *accel.YUVImage {
	yuv := &accel.YUVImage{
		Width:  width,
		Height: height,
		Y:      make([]byte, width*height),
		U:      make([]byte, width*height/4),
		V:      make([]byte, width*height/4),
	}

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r := int(rgb[(y*width+x)*3])
			g := int(rgb[(y*width+x)*3+1])
			b := int(rgb[(y*width+x)*3+2])

			yuv.Y[y*width+x] = byte((19595*r + 38470*g + 7471*b) >> 16)
		}
	}

	for y := 0; y < height/2; y++ {
		for x := 0; x < width/2; x++ {
			r := int(rgb[(y*2*width+x*2)*3])
			g := int(rgb[(y*2*width+x*2)*3+1])
			b := int(rgb[(y*2*width+x*2)*3+2])

			yuv.U[y*width/2+x] = byte(((-11056*r - 21712*g + 32768*b) >> 16) + 128)

			r = int(rgb[(y*2*width+x*2+1)*3])
			g = int(rgb[(y*2*width+x*2+1)*3+1])
			b = int(rgb[(y*2*width+x*2+1)*3+2])

			yuv.V[y*width/2+x] = byte(((32768*r - 27440*g - 5328*b) >> 16) + 128)
		}
	}

	return yuv
}
