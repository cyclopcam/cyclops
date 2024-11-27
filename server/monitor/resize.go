package monitor

import (
	"time"

	"github.com/bmharper/cimg/v2"
	"github.com/cyclopcam/cyclops/pkg/accel"
	"github.com/cyclopcam/cyclops/pkg/nn"
	"github.com/cyclopcam/cyclops/server/perfstats"
)

type ResizeQuality int

const (
	ResizeQualityLow ResizeQuality = iota
	ResizeQualityHigh
)

// Perform image format conversions and resizing so that we can send to our NN.
// We should consider having the resizing done by ffmpeg.
// Images returned are (originalRgb, nnScaledRgb)
func (m *Monitor) prepareImageForNN(yuv *accel.YUVImage, rgbNNMemory []byte, resizeQuality ResizeQuality) (nn.ResizeTransform, *cimg.Image, *cimg.Image) {
	start := time.Now()
	nnConfig := m.detector.Config()
	nnWidth := nnConfig.Width
	nnHeight := nnConfig.Height
	nnStride := nnWidth * 3

	xform := nn.IdentityResizeTransform()

	// Expensive-ish operation #1
	// Convert YUV to RGB, and store the RGB image, so that the client can access the most recently decoded frame.
	rgb := cimg.NewImage(yuv.Width, yuv.Height, cimg.PixelFormatRGB)
	yuv.CopyToCImageRGB(rgb)
	if (rgb.Width > nnWidth || rgb.Height > nnHeight) && m.hasShownResolutionWarning.CompareAndSwap(false, true) {
		m.Log.Warnf("Camera image size %vx%v is larger than NN input size %vx%v", rgb.Width, rgb.Height, nnWidth, nnHeight)
	}

	// This 'originalRgb' is the image straight from the camera, without any resizing.
	// We need to preserve this for the client.
	originalRgb := rgb

	// Likely expensive operation #2
	// RGB size being different to NN size is the expected code path.
	// The low res streams of cameras come in various flavours, like 640x360, 640x480, 320x240.
	// Even if we were to bake in many NN sizes, we'd still be constrained by their size being a multiple of
	// 32, which means our small NN must be 320x256 instead of 320x240.
	if rgb.Width == nnWidth && rgb.Height == nnHeight {
		// As mentioned above, this code path is not expected in reality, but we must support it.
		// This is a memory copy into the batch image buffer.
		copy(rgbNNMemory, rgb.Pixels)
	} else {
		// Resize the image to the NN size.
		// We pad with blackness on the right or bottom edge if the aspect ratios of camera and NN are different.
		scaleX := float32(nnWidth) / float32(rgb.Width)
		scaleY := float32(nnHeight) / float32(rgb.Height)
		scale := min(scaleX, scaleY)
		xform.ScaleX = scale
		xform.ScaleY = scale
		scaledWidth := int(float32(rgb.Width)*scale + 0.5)
		scaledHeight := int(float32(rgb.Height)*scale + 0.5)
		if resizeQuality == ResizeQualityLow && (scaledWidth == rgb.Width/2 && scaledHeight == rgb.Height/2) {
			// Exactly 2x downsize, so we can use our cheap and fast "SIMD" filter.
			// A practical case where you'll get this is when camera is set to 640x480 and we're using NCNN at 320x256.
			// This code path is much faster (4x) than stbir with fast (i.e. Box/Triangle) filter.
			accel.ReduceHalf(rgb.Width, rgb.Height, 3, rgb.Pixels, rgb.Stride, rgbNNMemory, nnStride)
		} else if scale == 1 {
			// If scale is 1, then the rgb image is smaller than the NN
			nnWrap := cimg.WrapImageStrided(nnWidth, nnHeight, cimg.PixelFormatRGB, rgbNNMemory, nnStride)
			nnWrap.CopyImageRect(rgb, 0, 0, rgb.Width, rgb.Height, 0, 0)
		} else {
			// For consistency with 2x downsize, use a box filter here too.
			// For discussion of quality/performance tradeoffs, see speed_test.go
			resizeParams := cimg.ResizeParams{CheapSRGBFilter: true}
			if resizeQuality == ResizeQualityHigh {
				// Of all the stbir filters I've tried, CatmullRom seems to be the sharpest.
				resizeParams.Filter = cimg.ResizeFilterCatmullRom
			} else if scale < 1 {
				// We use box filter for downsampling, in case we have a massive ratio
				resizeParams.Filter = cimg.ResizeFilterBox
			} else {
				// Triangle is bilinear on upsampling
				resizeParams.Filter = cimg.ResizeFilterTriangle
			}
			nnWrap := cimg.WrapImageStrided(scaledWidth, scaledHeight, cimg.PixelFormatRGB, rgbNNMemory, nnStride)
			cimg.Resize(rgb, nnWrap, &resizeParams)
		}
		if scaledWidth != nnWidth {
			// Fill the right edge with black
			for y := 0; y < nnHeight; y++ {
				clear(rgbNNMemory[y*nnStride+3*scaledWidth : y*nnStride+3*nnWidth])
			}
		} else if scaledHeight != nnHeight {
			// Fill the bottom edge with black
			for y := scaledHeight; y < nnHeight; y++ {
				clear(rgbNNMemory[y*nnStride : y*nnStride+3*nnWidth])
			}
		}
	}
	perfstats.UpdateMovingAverage(&m.avgTimeNSPerFrameNNPrep, time.Now().Sub(start).Nanoseconds())
	return xform, originalRgb, cimg.WrapImage(nnWidth, nnHeight, cimg.PixelFormatRGB, rgbNNMemory)
}
