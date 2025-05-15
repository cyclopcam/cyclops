package fsv

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/cyclopcam/cyclops/pkg/videoformat/rf1"
	"github.com/cyclopcam/logs"
	"github.com/stretchr/testify/require"
)

const BaseDir = "test-fsv-tmp"

func TestReaderWriter(t *testing.T) {
	testReaderWriter(t, false)
	testReaderWriter(t, true)
}

func testReaderWriter(t *testing.T, enableWriteBuffer bool) {
	EraseArchive()
	logger := logs.NewTestingLog(t)

	maxSizeDelta := float64(0)

	initSettings := DefaultStaticSettings()
	if enableWriteBuffer {
		initSettings.MaxWriteBufferSize = 1024 * 1024
	} else {
		initSettings.MaxWriteBufferSize = 0
	}

	// System wakes up and archive is empty

	arc1, err := Open(logger, BaseDir, []VideoFormat{&VideoFormatRF1{}}, initSettings, DefaultDynamicSettings())
	require.NoError(t, err)
	require.NotNil(t, arc1)

	tbase1 := todayAtTime(4, 5, 6, 7000)                                                // time.Date(2021, time.February, 3, 4, 5, 6, 7000, time.UTC)
	tbase2 := tbase1.Add(arc1.MaxVideoFileDuration())                                   // Packets after tbase 2 will require a new file
	packets1 := copyRf1NALUstoFsv(rf1.CreateTestNALUs(tbase1, 0, 100, 10, 50, 150, 13)) // these go into the first file
	packets2 := copyRf1NALUstoFsv(rf1.CreateTestNALUs(tbase2, 0, 50, 10, 50, 150, 15))  // these go into the second file

	require.InDelta(t, 0, arc1.TotalSize(), maxSizeDelta)

	require.NoError(t, arc1.Write("stream1", map[string]TrackPayload{"videoTrack1": makeVideoPayload(packets1)}))

	// After adding fragmentation avoidance, we can't verify file sizes while files are still open.
	// rf1 files only get truncated on close.
	//require.InDelta(t, expectedFilesize(packets1), arc1.TotalSize(), maxSizeDelta)

	// No files in index yet
	require.Equal(t, 0, len(arc1.streams["stream1"].files))

	// Verify that we can read data, even though no files are in the index yet.
	// Here we're only reading from 'current' - the video file that's currently open and being written to.
	verifyRead(t, arc1, "stream1", "videoTrack1", packets1[70].PTS, packets1[80].PTS, 10, 1)

	// Because packets2 cause the max video file duration to be exceeded,
	// the following write will cause the current video file to be retired,
	// and a new current file created.
	// That will place the previous current file into the index
	require.NoError(t, arc1.Write("stream1", map[string]TrackPayload{"videoTrack1": makeVideoPayload(packets2)}))

	if !enableWriteBuffer {
		// 1 file in index (and 1 current file)
		require.Equal(t, 1, len(arc1.streams["stream1"].files))

		// Read data from a file in the index, and the current file
		verifyRead(t, arc1, "stream1", "videoTrack1", packets1[0].PTS, packets2[len(packets2)-1].PTS, len(packets1)+len(packets2), 0)
	}

	arc1.Close()

	// System goes down and comes up again
	// Verify that we can read the data we just wrote.
	// Now we expect to find 2 files in the index.

	arc2, err := Open(logger, BaseDir, []VideoFormat{&VideoFormatRF1{}}, initSettings, DefaultDynamicSettings())
	require.NoError(t, err)
	require.NotNil(t, arc2)
	streams := arc2.ListStreams()
	require.Len(t, streams, 1)
	stream0 := streams[0]
	require.Equal(t, "stream1", stream0.Name)
	require.LessOrEqual(t, AbsTimeDiff(packets1[0].PTS, stream0.StartTime), time.Millisecond)
	require.LessOrEqual(t, AbsTimeDiff(packets2[len(packets2)-1].PTS, stream0.EndTime), time.Millisecond)

	// Must verify file size after closing and re-opening (due to fragmentation avoidance in rf1)
	t.Logf("Expect %v + %v = %v", expectedFilesize(packets1), expectedFilesize(packets2), expectedFilesize(packets1)+expectedFilesize(packets2))
	t.Logf("Actual %v", arc2.TotalSize())
	require.InDelta(t, expectedFilesize(packets1)+expectedFilesize(packets2), arc2.TotalSize(), maxSizeDelta)

	// We expect two video files, from packets1 and packets2 respectively
	require.Equal(t, 2, len(arc2.streams["stream1"].files))

	// Although the rf1 format is capable of it, in situations like this we don't attempt to
	// keep writing to existing files. We just create new stream files.
	// It should be a very rare event for the system to go down.

	// A read that spans only packets1
	verifyRead(t, arc2, "stream1", "videoTrack1", packets1[70].PTS, packets1[80].PTS, 10, 1)

	// A read that spans only packets2
	verifyRead(t, arc2, "stream1", "videoTrack1", packets2[30].PTS, packets2[35].PTS, 5, 1)

	// A read that spans all packets in packets1 and packets2
	verifyRead(t, arc2, "stream1", "videoTrack1", packets1[0].PTS, packets2[len(packets2)-1].PTS, len(packets1)+len(packets2), 0)

	arc2.Close()

	// Get the sweeper to run
	withSweep := DefaultDynamicSettings()
	withSweep.MaxArchiveSize = expectedFilesize(packets2) * 103 / 100
	initWithSweep := DefaultStaticSettings()
	initWithSweep.SweepInterval = time.Millisecond
	arc3, err := Open(logger, BaseDir, []VideoFormat{&VideoFormatRF1{}}, initWithSweep, withSweep)
	require.Equal(t, expectedFilesize(packets1)+expectedFilesize(packets2), arc3.TotalSize())
	time.Sleep(time.Millisecond * 3)
	require.Equal(t, expectedFilesize(packets2), arc3.TotalSize())
}

func expectedFilesize(packets []NALU) int64 {
	indexSize := 32 + (len(packets)+1)*8
	packetSize := 0
	for _, p := range packets {
		packetSize += len(p.Payload)
	}
	return int64(indexSize + packetSize)
}

func verifyRead(t *testing.T, arc *Archive, streamName string, trackName string, startTime time.Time, endTime time.Time, numExpectedPackets, maxPacketCountDelta int) {
	tracksR, err := arc.Read(streamName, []string{trackName}, startTime, endTime, 0)
	require.NoError(t, err)
	packets := tracksR[trackName]
	require.InDelta(t, numExpectedPackets, len(packets.NALS), float64(maxPacketCountDelta))
	require.InDelta(t, startTime.UnixMilli(), packets.NALS[0].PTS.UnixMilli(), float64(time.Millisecond))
}

func makeVideoPayload(packets []NALU) TrackPayload {
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

func EraseArchive() {
	os.RemoveAll(BaseDir)
	os.MkdirAll(BaseDir, 0755)
}

func TestMain(m *testing.M) {
	// Setup
	os.RemoveAll(BaseDir)
	if err := os.MkdirAll(BaseDir, 0755); err != nil {
		fmt.Printf("Error creating fsv test directory '%v': %v\n", BaseDir, err)
		os.Exit(1)
	}

	code := m.Run()

	// Teardown
	os.RemoveAll(BaseDir)

	os.Exit(code)
}
