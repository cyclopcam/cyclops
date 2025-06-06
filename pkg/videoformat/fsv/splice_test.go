package fsv

import (
	"slices"
	"testing"
	"time"

	"github.com/cyclopcam/cyclops/pkg/videoformat/rf1"
	"github.com/cyclopcam/logs"
	"github.com/stretchr/testify/require"
)

// Notes for all of the splice tests:

// CreateTestNALUs inserts an IDR every 10 frames, so we need to be mindful of that
// when picking a packet index that is in between IDRs.

// Key for splice diagrams:
// 1  packet is part of the first payload
// 2  packet is part of the second payload
// *  keyframe
// -  non-keyframe

func staticSettingsHugeWritebuffer() StaticSettings {
	s := DefaultStaticSettings()
	s.MaxWriteBufferTime = 10000 * time.Hour
	s.MaxWriteBufferSize = 1024 * 1024 * 100
	return s
}

// Return now() but at the given hour,minute,second,nsec UTC
// For tests that involve the write buffer, it's important that the
// packet times are relatively close to 'now', because the write
// buffer uses the delta between 'now' and the packet PTS to determine
// if it should flush that buffer.
func todayAtTime(hour, minute, second, nsec int) time.Time {
	t := time.Now().UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), hour, minute, second, nsec, time.UTC)
}

func TestSplicePerfect(t *testing.T) {
	EraseArchive()
	staticSettings := staticSettingsHugeWritebuffer()
	arc, err := Open(logs.NewTestingLog(t), BaseDir, []VideoFormat{&VideoFormatRF1{}}, staticSettings, DefaultDynamicSettings())
	require.NoError(t, err)
	packets := copyRf1NALUstoFsv(rf1.CreateTestNALUs(todayAtTime(3, 4, 5, 7000), 0, 300, 10, 50, 150, 12345))

	// Pattern:
	//
	// 11111111111
	// *------*------*------*
	//        222222222222222

	p1 := firstKeyFrameNALU_AtOrAfter(100, packets) + 5 // +5 to place us in between two IDRs
	p2 := firstKeyFrameNALU_AtOrBefore(p1, packets)

	require.True(t, p1 > p2)
	require.True(t, p2 > 0)
	require.True(t, p1-p2 < 20) // sanity check

	require.NoError(t, arc.Write("stream1", map[string]TrackPayload{"videoTrack1": makeVideoPayload(packets[:p1])}))
	require.NoError(t, arc.Write("stream1", map[string]TrackPayload{"videoTrack1": makeVideoPayload(packets[p2:])}))

	read, err := arc.Read("stream1", []string{"videoTrack1"}, packets[0].PTS, packets[len(packets)-1].PTS.Add(time.Second), 0)
	require.NoError(t, err)
	rPackets := read["videoTrack1"]

	// We expect a perfect splice, because we have overlapping packet ranges.
	requireEqualNALUs(t, packets, rPackets.NALS)
}

func TestSpliceImperfect(t *testing.T) {
	EraseArchive()
	disableWriteBuffer := DefaultStaticSettings()
	disableWriteBuffer.MaxWriteBufferSize = 0
	arc, err := Open(logs.NewTestingLog(t), BaseDir, []VideoFormat{&VideoFormatRF1{}}, disableWriteBuffer, DefaultDynamicSettings())
	require.EqualValues(t, 0, arc.TotalSize())
	packets1 := copyRf1NALUstoFsv(rf1.CreateTestNALUs(time.Date(2021, time.February, 3, 4, 5, 6, 7000, time.UTC), 0, 300, 10, 50, 150, 12345))
	packets2 := slices.Clone(packets1)

	// Add just enough delay to make packets no longer equal
	for i := range packets2 {
		packets2[i].PTS = packets2[i].PTS.Add(time.Millisecond)
	}

	// Pattern:
	//
	//           A   B
	// 11111111111
	// *------*------*------*
	//        222222222222222

	// The key difference here is that we modify packets in chunk '2', so that they are not the same
	// packets as those in '1'. The splicer will be unable to find the meeting point. That will cause
	// it to leave a gap in between A and B.

	p1 := firstKeyFrameNALU_AtOrAfter(100, packets1) + 5 // +5 to place us in between two IDRs
	p2 := firstKeyFrameNALU_AtOrBefore(p1, packets2)

	require.True(t, p1 > p2)
	require.True(t, p2 > 0)
	require.True(t, p1-p2 < 20) // sanity check

	require.NoError(t, arc.Write("stream1", map[string]TrackPayload{"videoTrack1": makeVideoPayload(packets1[:p1])}))
	require.NoError(t, arc.Write("stream1", map[string]TrackPayload{"videoTrack1": makeVideoPayload(packets2[p2:])}))

	// Read everything
	read, err := arc.Read("stream1", []string{"videoTrack1"}, packets1[0].PTS, packets1[len(packets1)-1].PTS.Add(time.Second), 0)
	require.NoError(t, err)
	rPackets := read["videoTrack1"]

	// We expect a gap in between A and B.
	// A = p1
	// B = the first keyframe in packets2 that is after p2.
	expectedPackets := slices.Clone(packets1[:p1])
	p3 := firstKeyFrameNALU_AtOrAfter(p2+1, packets2)

	// reverse 2 to include the 2 EssentialMeta NALUs (eg SPS,PPS)
	// See SYNC-TEST-META-COUNT for where this is controlled.
	p3 -= 2

	expectedPackets = append(expectedPackets, packets2[p3:]...)

	requireEqualNALUs(t, expectedPackets, rPackets.NALS)
}

func firstKeyFrameNALU_AtOrBefore(i int, packets []NALU) int {
	for ; i >= 0; i-- {
		if packets[i].Flags&NALUFlagKeyFrame != 0 {
			return i
		}
	}
	panic("No keyframes before this point")
}

func firstKeyFrameNALU_AtOrAfter(i int, packets []NALU) int {
	for ; i < len(packets); i++ {
		if packets[i].Flags&NALUFlagKeyFrame != 0 {
			return i
		}
	}
	panic("No keyframes after this point")
}

func requireEqualNALUs(t *testing.T, expected, actual []NALU) {
	require.Equal(t, len(expected), len(actual))
	for i := range expected {
		require.LessOrEqual(t, AbsTimeDiff(expected[i].PTS, actual[i].PTS), time.Second/4096)
		require.Equal(t, expected[i].Flags, actual[i].Flags)
		require.Equal(t, expected[i].Payload, actual[i].Payload)
	}
}
