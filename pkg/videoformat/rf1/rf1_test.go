package rf1

import (
	"fmt"
	"os"
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
		testReaderWriter(t, closeAndReOpen == 1)
	}
}

func testReaderWriter(t *testing.T, enableCloseAndReOpen bool) {
	tbase := time.Date(2021, time.February, 3, 4, 5, 6, 7000, time.UTC)
	trackW, err := MakeVideoTrack("HD", tbase, CodecH264, 320, 240)
	require.NoError(t, err)
	fw, err := Create(BaseDir+"/test", []*Track{trackW})
	require.NoError(t, err)
	require.NotNil(t, fw)
	nNALUs := 200
	fps := 10.0
	nalusW := CreateTestNALUs(trackW.TimeBase, 0, nNALUs, fps, 12345)
	chunkSize := 11
	for i := 0; i < nNALUs; i += chunkSize {
		end := i + chunkSize
		if end > nNALUs {
			end = nNALUs
		}
		err := trackW.WriteNALUs(nalusW[i:end])
		require.NoError(t, err)
		if i == chunkSize*3 && enableCloseAndReOpen {
			// Stress the fact that we can close a file and re-open it, and then continue writing to it.
			err = fw.Close()
			require.NoError(t, err)
			fw, err = Open(BaseDir+"/test", OpenModeReadWrite)
			require.NoError(t, err)
			require.NotNil(t, fw)
			require.Equal(t, 1, len(fw.Tracks))
			trackW = fw.Tracks[0]
		}
		require.LessOrEqual(t, AbsTimeDiff(nalusW[end-1].PTS, trackW.TimeBase.Add(trackW.Duration())), time.Second/4096)
	}
	err = fw.Close()
	require.NoError(t, err)

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
			require.Equal(t, nalusW[i+j].Payload, nalusR[j].Payload)
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

	for freshOpen := 0; freshOpen < 2; freshOpen++ {
		for _, tt := range timesToRead {
			if freshOpen == 1 {
				fr.Close()
				fr, _ = Open(BaseDir+"/test", OpenModeReadOnly)
			}
			result, err := fr.Tracks[0].ReadAtTime(tt.start.Sub(trackR.TimeBase), tt.end.Sub(trackR.TimeBase))
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
