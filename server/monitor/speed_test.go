package monitor

import (
	"testing"

	"github.com/bmharper/cimg/v2"
	"github.com/cyclopcam/cyclops/pkg/accel"
	"github.com/stretchr/testify/require"
)

// Conclusion?
// Well, my Hikvision 640x480 images are badly downsampled. SIGH. The camera should do better!
// Here we're testing downsampling that image further to 320x240. This presents an opportunity
// to fix some of the bad sampling that the camera did. But is it worth it?
// The default filter on stb_image_resize is great - not sure what it is. Probably any of the
// fancier filters besides 2x2 box would produce pretty good results. But does fancy filtering
// of badly aliased images even matter that much to our neural network? I don't know!
// But the performance difference of the "SIMD" library's 2x2 downsampler is so much better
// than stb_image_resize, that I'm going with that for now, if it happens to be a neat
// 2x downsample operation.
// See the numbers above BenchmarkImageResize640to320().

func TestImageResize640to320(t *testing.T) {
	img, err := cimg.ReadFile("../../testdata/driveway002-640.jpg")
	require.NoError(t, err)

	filters := []cimg.ResizeFilter{cimg.ResizeFilterBox, cimg.ResizeFilterPointSample, cimg.ResizeFilterDefault, cimg.ResizeFilterMitchell}
	filterNames := []string{"box", "point", "default", "mitchell"}

	for i := 0; i < len(filters); i++ {
		resizeParams := cimg.ResizeParams{
			CheapSRGBFilter: true,
			Filter:          filters[i],
		}
		resized := cimg.ResizeNew(img, img.Width/2, img.Height/2, &resizeParams)
		resized.WriteJPEG("sample-"+filterNames[i]+".jpg", cimg.MakeCompressParams(cimg.Sampling444, 99, 0), 0644)
	}

	resized := simple2x2Downsample(img)
	resized.WriteJPEG("sample-simple2x2.jpg", cimg.MakeCompressParams(cimg.Sampling444, 99, 0), 0644)

	accel.ReduceHalf(img.Width, img.Height, img.NChan(), img.Pixels, img.Stride, resized.Pixels, resized.Stride)
	resized.WriteJPEG("sample-simd2x2.jpg", cimg.MakeCompressParams(cimg.Sampling444, 99, 0), 0644)
}

// Simple sRGB-space 2x2 downsampling for comparison
func simple2x2Downsample(src *cimg.Image) *cimg.Image {
	dst := cimg.NewImage(src.Width/2, src.Height/2, cimg.PixelFormatRGB)
	for y := 0; y < src.Height; y += 2 {
		srcLine1 := src.Pixels[y*src.Stride : y*src.Stride+src.Width*3]
		srcLine2 := src.Pixels[(y+1)*src.Stride : (y+1)*src.Stride+src.Width*3]
		dstLine := dst.Pixels[y/2*dst.Stride : y/2*dst.Stride+dst.Width*3]
		i := 0
		j := 0
		for x := 0; x < src.Width; x += 2 {
			r := (int(srcLine1[i]) + int(srcLine1[i+3]) + int(srcLine2[i]) + int(srcLine2[i+3])) >> 2
			g := (int(srcLine1[i+1]) + int(srcLine1[i+4]) + int(srcLine2[i+1]) + int(srcLine2[i+4])) >> 2
			b := (int(srcLine1[i+2]) + int(srcLine1[i+5]) + int(srcLine2[i+2]) + int(srcLine2[i+5])) >> 2
			dstLine[j] = byte(r)
			dstLine[j+1] = byte(g)
			dstLine[j+2] = byte(b)
			i += 6
			j += 3
		}
	}
	return dst
}

// Ryzen 5900X Filter=Default	twoStep = true		463346 ns/op	0.46ms
// Ryzen 5900X Filter=Default	twoStep = false		422725 ns/op	0.42ms
// Ryzen 5900X Filter=Box		twoStep = false		218037 ns/op	0.22ms
// Ryzen 5900X Filter=Box		simd2x2				45122 ns/op		0.05ms   // wow!
func BenchmarkImageResize640to320(b *testing.B) {
	srcWidth, srcHeight := 640, 480
	nnWidth, nnHeight := 320, 256

	// This simulates an original code path for resizing incoming 640x480 images, for inference on a 320x256 NN
	src := cimg.NewImage(srcWidth, srcHeight, cimg.PixelFormatRGB)

	useSimdBox := true // implies twoStep=false
	twoStep := false

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resizeParams := cimg.ResizeParams{
			CheapSRGBFilter: true,
			Filter:          cimg.ResizeFilterBox,
		}
		if useSimdBox {
			newImg := cimg.NewImage(nnWidth, nnHeight, cimg.PixelFormatRGB)
			accel.ReduceHalf(srcWidth, srcHeight, 3, src.Pixels, src.Stride, newImg.Pixels, newImg.Stride)
		} else {
			if twoStep {
				resized := cimg.ResizeNew(src, srcWidth/2, srcHeight/2, &resizeParams)
				final := cimg.NewImage(nnWidth, nnHeight, cimg.PixelFormatRGB)
				final.CopyImage(resized, 0, 0)
			} else {
				cimg.ResizeNew(src, srcWidth/2, srcHeight/2, &resizeParams)
			}
		}
	}
}
