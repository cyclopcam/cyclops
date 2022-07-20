package videox

import "image"

func cloneImage(srcImg image.Image) image.Image {
	src, ok := srcImg.(*image.RGBA)
	if !ok {
		panic("Expected srcImg to be RGBA")
	}
	dst := image.NewRGBA(srcImg.Bounds())
	h := src.Rect.Dy()
	for i := 0; i < h; i++ {
		copy(dst.Pix[i*dst.Stride:(i+1)*dst.Stride], src.Pix[i*src.Stride:(i+1)*src.Stride])
	}
	return dst
}
