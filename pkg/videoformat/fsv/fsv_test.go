package fsv

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/cyclopcam/cyclops/pkg/log"
	"github.com/cyclopcam/cyclops/pkg/videoformat/rf1"
	"github.com/stretchr/testify/require"
)

const BaseDir = "test-fsv-tmp"

func TestReaderWriter(t *testing.T) {
	logger := log.NewTestingLog(t)

	// System wakes up and archive is empty

	arc1, err := Open(logger, BaseDir, []VideoFormat{&VideoFormatRF1{}})
	require.NoError(t, err)
	require.NotNil(t, arc1)

	tbase1 := time.Date(2021, time.February, 3, 4, 5, 6, 7000, time.UTC)
	tbase2 := tbase1.Add(arc1.MaxVideoFileDuration()) // requires a new file
	packets1 := rf1.CreateTestNALUs(tbase1, 0, 100, 10.0, 13)
	packets2 := rf1.CreateTestNALUs(tbase2, 0, 50, 10.0, 15)
	require.NoError(t, arc1.Write("stream1", map[string]TrackPayload{"videoTrack1": makeVideoPayload(packets1)}))

	// No files in index yet
	require.Equal(t, 0, len(arc1.streams["stream1"].files))

	// This will cause the current file to be retired, and a new current file created.
	// That will place the previous current file into the index
	require.NoError(t, arc1.Write("stream1", map[string]TrackPayload{"videoTrack1": makeVideoPayload(packets2)}))

	require.Equal(t, 1, len(arc1.streams["stream1"].files))

	arc1.Close()

	// System goes down and comes up again
	// Verify that we can read the data we just wrote.

	arc2, err := Open(logger, BaseDir, []VideoFormat{&VideoFormatRF1{}})
	require.NoError(t, err)
	require.NotNil(t, arc2)
	streams := arc2.ListStreams()
	require.Len(t, streams, 1)
	stream0 := streams[0]
	require.Equal(t, "stream1", stream0.Name)
	require.LessOrEqual(t, AbsTimeDiff(packets1[0].PTS, stream0.StartTime), time.Millisecond)
	require.LessOrEqual(t, AbsTimeDiff(packets2[len(packets2)-1].PTS, stream0.EndTime), time.Millisecond)

	require.Equal(t, 2, len(arc2.streams["stream1"].files))

	// Although the rf1 format is capable of it, in situations like this we don't attempt to
	// keep writing to existing files. We just create new stream files.
	// It should be a very rare event for the system to go down.

}

func makeVideoPayload(packets []rf1.NALU) TrackPayload {
	return TrackPayload{
		TrackType:   rf1.TrackTypeVideo,
		Codec:       rf1.CodecH264,
		VideoWidth:  320,
		VideoHeight: 240,
		NALUs:       packets,
	}
}

func AbsTimeDiff(t1, t2 time.Time) time.Duration {
	diff := t1.Sub(t2)
	if diff < 0 {
		return -diff
	}
	return diff
}

func TestMain(m *testing.M) {
	// Setup
	if err := os.MkdirAll(BaseDir, 0755); err != nil {
		fmt.Printf("Error creating fsv test directory '%v': %v\n", BaseDir, err)
		os.Exit(1)
	}

	code := m.Run()

	// Teardown
	os.RemoveAll(BaseDir)

	os.Exit(code)
}
