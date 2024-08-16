package rf1

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/require"
)

const BaseDir = "test-rf1-tmp"

func TestBits(t *testing.T) {
	testIndexNALU := func(tim, location int64, flags IndexNALUFlags) {
		p := MakeIndexNALU(tim, location, flags)
		tim2, location2, flags2 := SplitIndexNALU(p)
		require.Equal(t, tim, tim2)
		require.Equal(t, location, location2)
		require.Equal(t, flags, flags2)
	}
	testIndexNALU(3, 7, IndexNALUFlagKeyFrame)
	// test limits
	testIndexNALU(0, 0, 0)
	testIndexNALU((1<<22)-1, (1<<30)-1, IndexNALUFlags(2048))
	require.Panics(t, func() {
		MakeIndexNALU(1<<22, 0, 0)
	})
	require.Panics(t, func() {
		MakeIndexNALU(0, 1<<30, 0)
	})
	require.Panics(t, func() {
		MakeIndexNALU(0, 0, IndexNALUFlags(4096))
	})
}

func TestReaderWriter(t *testing.T) {
	t.Logf("sizeof(rf1.NALU) = %v", unsafe.Sizeof(NALU{}))

	for closeAndReOpen := 0; closeAndReOpen < 2; closeAndReOpen++ {
		for enableDirtyClose := 0; enableDirtyClose < 2; enableDirtyClose++ {
			for enablePreAllocate := 0; enablePreAllocate < 2; enablePreAllocate++ {
				for enableWriteAggregate := 0; enableWriteAggregate < 2; enableWriteAggregate++ {
					for largeNALU := 0; largeNALU < 2; largeNALU++ {
						t.Logf("testReaderWriter closeAndReOpen=%v enableDirtyClose=%v, enablePreAllocate=%v, enableWriteAggregate=%v, largeNALU=%v", closeAndReOpen, enableDirtyClose, enablePreAllocate, enableWriteAggregate, largeNALU)
						testReaderWriter(t, closeAndReOpen == 1, enableDirtyClose == 1, enablePreAllocate == 1, enableWriteAggregate == 1, largeNALU == 1)
					}
				}
			}
		}
	}
}

func testReaderWriter(t *testing.T, enableCloseAndReOpen, enableDirtyClose, enablePreAllocate, enableWriteAggregate, largeNALU bool) {
	tbase := time.Date(2021, time.February, 3, 4, 5, 6, 7000, time.UTC)
	trackW, err := MakeVideoTrack("HD", tbase, CodecH264, 320, 240)
	trackW.disablePreallocate = !enablePreAllocate
	trackW.disablePreallocate = !enableWriteAggregate
	require.NoError(t, err)
	fw, err := Create(BaseDir+"/test", []*Track{trackW})
	require.NoError(t, err)
	require.NotNil(t, fw)
	nNALUs := 200
	nWritten := 0
	fps := 10.0
	minPacketSize := 50
	maxPacketSize := 150
	if largeNALU {
		minPacketSize = 50
		maxPacketSize = 10000
	}
	nalusW := CreateTestNALUs(trackW.TimeBase, 0, nNALUs, fps, minPacketSize, maxPacketSize, 12345)
	chunkSize := 11
	for i := 0; i < nNALUs; i += chunkSize {
		end := i + chunkSize
		if end > nNALUs {
			end = nNALUs
		}
		err := trackW.WriteNALUs(nalusW[i:end])
		require.NoError(t, err)
		nWritten += end - i
		require.Equal(t, nWritten, trackW.Count())
		if i == chunkSize*3 && enableCloseAndReOpen {
			// Stress the fact that we can close a file and re-open it, and then continue writing to it.
			if enableDirtyClose {
				dirtyClose(fw)
			} else {
				err = fw.Close()
				require.NoError(t, err)
			}
			fw, err = Open(BaseDir+"/test", OpenModeReadWrite)
			require.NoError(t, err)
			require.NotNil(t, fw)
			require.Equal(t, 1, len(fw.Tracks))
			trackW = fw.Tracks[0]
			require.Equal(t, nWritten, trackW.Count())
		}
		require.LessOrEqual(t, AbsTimeDiff(nalusW[end-1].PTS, trackW.TimeBase.Add(trackW.Duration())), time.Second/4096)
	}
	if enableDirtyClose {
		dirtyClose(fw)
	} else {
		err = fw.Close()
		require.NoError(t, err)
	}

	// Read
	fr, err := Open(BaseDir+"/test", OpenModeReadOnly)
	require.NoError(t, err)
	require.NotNil(t, fr)
	require.Equal(t, 1, len(fr.Tracks))
	trackR := fr.Tracks[0]
	require.Equal(t, nNALUs, trackR.Count())
	require.Equal(t, trackW.Type, trackR.Type)
	require.Equal(t, trackW.Name, trackR.Name)
	require.Equal(t, trackW.TimeBase, trackR.TimeBase)
	require.Equal(t, trackW.Codec, trackR.Codec)
	require.Equal(t, trackW.Width, trackR.Width)
	require.Equal(t, trackW.Height, trackR.Height)
	require.Equal(t, trackR.canWrite, false)
	require.Equal(t, trackR.indexCount, nNALUs)
	require.Equal(t, trackR.Count(), nNALUs)
	require.LessOrEqual(t, AbsTimeDiff(nalusW[nNALUs-1].PTS, trackR.TimeBase.Add(trackR.Duration())), time.Second/4096)
	for i := 0; i < nNALUs; i += chunkSize {
		end := i + chunkSize
		if end > nNALUs {
			end = nNALUs
		}
		nalusR, err := trackR.ReadIndex(i, end)
		require.NoError(t, err)
		require.Equal(t, end-i, len(nalusR))
		for j := 0; j < len(nalusR); j++ {
			// Our time precision is 1/4096 of a second
			require.LessOrEqual(t, AbsTimeDiff(nalusW[i+j].PTS, nalusR[j].PTS), time.Second/4096)
			require.Equal(t, nalusW[i+j].Flags, nalusR[j].Flags)
			require.EqualValues(t, len(nalusW[i+j].Payload), nalusR[j].Length)
		}
		err = trackR.ReadPayload(nalusR)
		require.NoError(t, err)
		for j := 0; j < len(nalusR); j++ {
			require.Equal(t, nalusW[i+j].Payload, nalusR[j].Payload, fmt.Sprintf("NALU payload %v", j))
		}
	}
	allNALUs, err := fr.Tracks[0].ReadIndex(0, fr.Tracks[0].Count())
	require.NoError(t, err)

	// ReadAtTime
	timesToRead := []struct {
		start     time.Time
		end       time.Time
		expectErr error
		firstIdx  int
		lastIdx   int
	}{
		{allNALUs[0].PTS, allNALUs[10].PTS, nil, 0, -1},
		{allNALUs[15].PTS, allNALUs[20].PTS, nil, -1, -1},
		{allNALUs[len(allNALUs)-30].PTS, allNALUs[len(allNALUs)-1].PTS.Add(5 * time.Second), nil, -1, len(allNALUs) - 1},
	}

	// Validate the index cache by reading once where data is cached, and a second time where it is not cached.
	for freshOpen := 0; freshOpen < 2; freshOpen++ {
		for _, tt := range timesToRead {
			if freshOpen == 1 {
				fr.Close()
				fr, _ = Open(BaseDir+"/test", OpenModeReadOnly)
			}
			result, err := fr.Tracks[0].ReadAtTime(tt.start.Sub(trackR.TimeBase), tt.end.Sub(trackR.TimeBase), 0)
			if tt.expectErr != nil {
				require.Error(t, err)
				require.Nil(t, result)
				continue
			}
			require.NoError(t, err)
			expectN := 0
			for _, n := range allNALUs {
				if (n.PTS.Equal(tt.start) || n.PTS.After(tt.start)) && n.PTS.Before(tt.end) {
					expectN++
				}
			}
			require.InDelta(t, expectN, len(result), 1)
			if tt.firstIdx >= 0 {
				// ensure that the first NALU we read is as expected
				// These assertions verify that we do indeed read the first NALU in the file, so we don't have
				// some kind of off-by-one error in that regard.
				require.Equal(t, allNALUs[tt.firstIdx].Position, result[0].Position)
			}
			if tt.lastIdx >= 0 {
				// ensure that the last NALU we read is as expected.
				// These assertions verify that we do indeed read the final NALU in the file, similar to the check above.
				require.Equal(t, allNALUs[tt.lastIdx].Position, result[len(result)-1].Position)
			}
		}
	}

}

func createTestVideo(t *testing.T, filename string) *File {
	tbase := time.Date(2021, time.February, 3, 4, 5, 6, 7000, time.UTC)
	trackW, err := MakeVideoTrack("HD", tbase, CodecH264, 320, 240)
	require.NoError(t, err)
	fw, err := Create(filepath.Join(BaseDir, filename), []*Track{trackW})
	require.NoError(t, err)
	require.NotNil(t, fw)
	return fw
}

func requireEqualNALUs(t *testing.T, expected, actual []NALU) {
	require.Equal(t, len(expected), len(actual))
	for i := range expected {
		require.LessOrEqual(t, AbsTimeDiff(expected[i].PTS, actual[i].PTS), time.Second/4096)
		require.Equal(t, expected[i].Flags, actual[i].Flags)
		require.Equal(t, expected[i].Payload, actual[i].Payload)
	}
}

func TestBigRead(t *testing.T) {
	fw := createTestVideo(t, "bigread")
	nNALUs := 10000
	nalusW := CreateTestNALUs(fw.Tracks[0].TimeBase, 0, nNALUs, 10.0, 500, 1500, 12345)
	require.NoError(t, fw.Tracks[0].WriteNALUs(nalusW))
	require.NoError(t, fw.Close())
	fw, err := Open(BaseDir+"/bigread", OpenModeReadOnly)
	require.NoError(t, err)
	nalusR, err := fw.Tracks[0].ReadIndex(0, nNALUs)
	require.NoError(t, err)
	require.Equal(t, nNALUs, len(nalusR))
	require.NoError(t, fw.Tracks[0].ReadPayload(nalusR))
	requireEqualNALUs(t, nalusW, nalusR)
}

func TestReadBackToKeyframe(t *testing.T) {
	var err error
	fw := createTestVideo(t, "keyframe")
	nNALUs := 100
	timeBase := fw.Tracks[0].TimeBase
	nalusW := CreateTestNALUs(timeBase, 0, nNALUs, 10.0, 500, 1500, 12345)
	require.NoError(t, fw.Tracks[0].WriteNALUs(nalusW))
	for iter := 0; iter < 2; iter++ {
		if iter == 1 {
			require.NoError(t, fw.Close())
			fw, err = Open(BaseDir+"/keyframe", OpenModeReadOnly)
			require.NoError(t, err)
		}
		// 0,1 are EssentialMetadata.
		// 2 is keyframe.
		// 12 is keyframe.
		// 22 is keyframe.
		// and so on.
		// If the keyframe is immediately preceded by EssentialMetadata NALUs (i.e. SPS/PPS),
		// then we want the function to continue reading back to include that EssentialMetadata.
		// So we have two different test cases - one for frames 2-12, and one for frames 12-22.
		nalus, err := fw.Tracks[0].ReadAtTime(nalusW[0].PTS.Sub(timeBase), nalusW[5].PTS.Sub(timeBase), PacketReadFlagSeekBackToKeyFrame)
		require.NoError(t, err)
		require.Equal(t, 6, len(nalus))
		require.LessOrEqual(t, AbsTimeDiff(timeBase, nalus[0].PTS), time.Second/4096)

		// This returns the exact same result as above, walking backwards through the keyframe,
		// and including the EssentialMetadata.
		nalus, err = fw.Tracks[0].ReadAtTime(nalusW[4].PTS.Sub(timeBase), nalusW[5].PTS.Sub(timeBase), PacketReadFlagSeekBackToKeyFrame)
		require.NoError(t, err)
		require.Equal(t, 6, len(nalus))
		require.LessOrEqual(t, AbsTimeDiff(timeBase, nalus[0].PTS), time.Second/4096)

		// Here we don't have an EssentialMetadata behind the keyframes, so just seek back to the keyframe
		nalus, err = fw.Tracks[0].ReadAtTime(nalusW[15].PTS.Sub(timeBase), nalusW[16].PTS.Sub(timeBase), PacketReadFlagSeekBackToKeyFrame)
		require.NoError(t, err)
		require.Equal(t, 5, len(nalus))
		require.LessOrEqual(t, AbsTimeDiff(nalusW[12].PTS, nalus[0].PTS), time.Second/4096)

		// And here we don't include the flag at all
		nalus, err = fw.Tracks[0].ReadAtTime(nalusW[15].PTS.Sub(timeBase), nalusW[16].PTS.Sub(timeBase), 0)
		require.NoError(t, err)
		require.Equal(t, 2, len(nalus))
		require.LessOrEqual(t, AbsTimeDiff(nalusW[15].PTS, nalus[0].PTS), time.Second/4096)

	}
}

// Close a file without doing the regular cleanup that we do when closing a file.
// This includes:
// 1. Not writing the index header
// 2. Not truncating the index file
func dirtyClose(f *File) {
	for _, t := range f.Tracks {
		if t.index != nil {
			t.index.Close()
		}
		if t.packets != nil {
			t.packets.Close()
		}
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
		fmt.Printf("Error creating rf1 test directory '%v': %v\n", BaseDir, err)
		os.Exit(1)
	}

	code := m.Run()

	// Teardown
	os.RemoveAll(BaseDir)

	os.Exit(code)
}
