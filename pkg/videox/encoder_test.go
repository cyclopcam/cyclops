package videox

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// This uses a disk dump of raw camera data, so that we can iterate on ffmpeg code without needing to connect to a camera and wait for a few frames.
// i've renamed this so that it's not actually a unit test, because this was just for dev/iteration.
// BUT... I guess we should make this an actual test
func disabledTestEncoder(t *testing.T) {
	root := "/home/ben/dev/cyclops"

	enc, err := NewVideoEncoder("h264", "mp4", root+"/dump/test-go.mp4", 2048, 1536, AVPixelFormatYUV420P, AVPixelFormatYUV420P, VideoEncoderTypePackets, 10)
	require.Nil(t, err)
	defer enc.Close()

	raw, err := LoadBinDir(root + "/raw")
	require.Nil(t, err)

	width, height, err := raw.DecodeHeader()
	t.Logf("width: %v, height: %v, err: %v", width, height, err)

	for ipacket, packet := range raw.Packets {
		dts := packet.PTS
		pts := dts + time.Nanosecond
		t.Logf("Writing packet %v at dst:%v, pts:%v (size[0] %v)", ipacket, dts.Nanoseconds(), pts.Nanoseconds(), len(packet.NALUs[0].Payload))
		for _, nalu := range packet.NALUs {
			err := enc.WriteNALU(dts, pts, nalu)
			require.Nil(t, err)
			dts++
			pts++
		}
	}

	err = enc.WriteTrailer()
	require.Nil(t, err)

	enc.Close()
}
