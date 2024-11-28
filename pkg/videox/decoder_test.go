package videox

import (
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecoder2(t *testing.T) {
	decoder, err := NewVideoFileDecoder("../../testdata/tracking/0001-LD.mp4")
	require.NoError(t, err)

	nframes := 0
	for {
		frame, err := decoder.NextFrame()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)
		require.Equal(t, 320, frame.Image.Width)
		require.Equal(t, 240, frame.Image.Height)
		// The following snippet is useful as a sanity check,
		// but I'm not too worried about fleshing this test out too much, because
		// we've already got a test of the underlying C code decoder_test.cpp
		//if nframes == 30 {
		//	b, _ := cimg.Compress(img.ToCImageRGB(), cimg.CompressParams{Quality: 90})
		//	os.WriteFile("frame30.jpg", b, 0644)
		//}
		nframes++
	}
	require.Equal(t, 64, nframes)
	decoder.Close()
}
