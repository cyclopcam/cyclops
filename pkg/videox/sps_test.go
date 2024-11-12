package videox

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func testSpsH264File(t *testing.T, filename string, expectWidth, expectHeight int) {
	raw, err := os.ReadFile(filename)
	require.NoError(t, err)
	width, height, err := ParseH264SPS(raw)
	require.NoError(t, err)
	require.Equal(t, expectWidth, width)
	require.Equal(t, expectHeight, height)
}

func TestSpsH264(t *testing.T) {
	testSpsH264File(t, "../../testdata/sps/sps-camera1-high.h264", 1920, 1080)
	testSpsH264File(t, "../../testdata/sps/sps-camera1-low.h264", 320, 240)
	testSpsH264File(t, "../../testdata/sps/sps-camera2-high.h264", 2688, 1520)
	testSpsH264File(t, "../../testdata/sps/sps-camera2-low.h264", 320, 240)
	testSpsH264File(t, "../../testdata/sps/sps-camera3-high.h264", 2048, 1536)
	testSpsH264File(t, "../../testdata/sps/sps-camera3-low.h264", 640, 480)
}

func BenchmarkSpsH264Parse(b *testing.B) {
	raw, err := os.ReadFile("../../testdata/sps/sps-camera1-high.h264")
	require.NoError(b, err)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ParseH264SPS(raw)
	}
}
