package fsv

import (
	"testing"
	"time"

	"github.com/cyclopcam/cyclops/pkg/log"
	"github.com/cyclopcam/cyclops/pkg/videoformat/rf1"
	"github.com/stretchr/testify/require"
)

func TestSplice(t *testing.T) {
	logger := log.NewTestingLog(t)
	arc, _ := Open(logger, BaseDir, []VideoFormat{&VideoFormatRF1{}}, DefaultArchiveSettings())
	tbase := time.Date(2021, time.February, 3, 4, 5, 6, 7000, time.UTC)
	packets := rf1.CreateTestNALUs(tbase, 0, 300, 10.0, 100, 12345)

	// CreateTestNALUs inserts an IDR every 10 frames.

	// Create a test pattern that looks like this:
	// 1  packet is part of the first chunk
	// 2  packet is part of the second chunk
	// *  keyframe
	// -  non-keyframe

	// 11111111111
	// *------*------*------*
	//        222222222222222

	p1 := firstKeyFrameAtOrAfter(100, packets) + 5 // +5 to place us in between two IDRs
	p2 := firstKeyFrameAtOrBefore(p1, packets)

	require.True(t, p1 > p2)
	require.True(t, p2 > 0)
	require.True(t, p1-p2 < 20) // sanity check

	require.NoError(t, arc.Write("stream1", map[string]TrackPayload{"videoTrack1": makeVideoPayload(packets[:p1])}))
	require.NoError(t, arc.Write("stream1", map[string]TrackPayload{"videoTrack1": makeVideoPayload(packets[p2:])}))

	read, err := arc.Read("stream1", []string{"videoTrack1"}, packets[0].PTS, packets[len(packets)-1].PTS.Add(time.Second))
	require.NoError(t, err)
	rPackets := read["videoTrack1"]

	// We expect a perfect splice, because we have overlapping packet ranges.
	requireEqualNALUs(t, packets, rPackets)
}

func firstKeyFrameAtOrBefore(i int, packets []rf1.NALU) int {
	for ; i >= 0; i-- {
		if packets[i].Flags&rf1.IndexNALUFlagKeyFrame != 0 {
			return i
		}
	}
	panic("No keyframes before this point")
}

func firstKeyFrameAtOrAfter(i int, packets []rf1.NALU) int {
	for ; i < len(packets); i++ {
		if packets[i].Flags&rf1.IndexNALUFlagKeyFrame != 0 {
			return i
		}
	}
	panic("No keyframes after this point")
}

func requireEqualNALUs(t *testing.T, expected, actual []rf1.NALU) {
	require.Equal(t, len(expected), len(actual))
	for i := range expected {
		require.LessOrEqual(t, AbsTimeDiff(expected[i].PTS, actual[i].PTS), time.Second/4096)
		require.Equal(t, expected[i].Flags, actual[i].Flags)
		require.Equal(t, expected[i].Payload, actual[i].Payload)
	}
}
