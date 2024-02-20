package rf1

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"testing"
	"time"

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

// The first 2 NALUs are EssentialMetadata
// The 3rd NALU is a keyframe, and every 10th frame thereafter is a keyframe
func testNALUFlags(naluIdx int) IndexNALUFlags {
	if naluIdx <= 2 {
		return IndexNALUFlagEssentialMeta
	} else if naluIdx%10 == 3 {
		return IndexNALUFlagKeyFrame
	}
	return 0
}

// Frame flags are controlled by testNALUFlags()
func createTestNALUs(track *Track, nFrames int, fps float64) []NALU {
	nalus := make([]NALU, nFrames)
	rng := rand.New(rand.NewSource(12345))
	for i := 0; i < nFrames; i++ {
		pts := time.Duration(float64(i) * float64(time.Second) / fps)
		nalu := NALU{
			PTS: track.TimeBase.Add(pts),
		}
		nalu.Flags = testNALUFlags(i)
		packetSize := rng.Intn(100) + 50
		nalu.Payload = make([]byte, packetSize)
		_, err := rng.Read(nalu.Payload)
		if err != nil {
			panic(err)
		}
		nalus[i] = nalu
	}
	return nalus
}

func TestReaderWriter(t *testing.T) {
	tbase := time.Date(2021, time.February, 3, 4, 5, 6, 7000, time.UTC)
	trackW, err := MakeVideoTrack("HD", tbase, CodecH264, 320, 240)
	require.NoError(t, err)
	w, err := NewWriter(BaseDir+"/test", []*Track{trackW})
	require.NoError(t, err)
	require.NotNil(t, w)
	nNALUs := 200
	fps := 10.0
	nalusW := createTestNALUs(trackW, nNALUs, fps)
	chunkSize := 11
	for i := 0; i < nNALUs; i += chunkSize {
		end := i + chunkSize
		if end > nNALUs {
			end = nNALUs
		}
		err := trackW.WriteNALUs(nalusW[i:end])
		require.NoError(t, err)
	}
	err = w.Close()
	require.NoError(t, err)

	// Read
	r, err := Open(BaseDir + "/test")
	require.NoError(t, err)
	require.NotNil(t, r)
	require.Equal(t, 1, len(r.Tracks))
	trackR := r.Tracks[0]
	require.Equal(t, nNALUs, trackR.Count())
	require.Equal(t, trackW.IsVideo, trackR.IsVideo)
	require.Equal(t, trackW.Name, trackR.Name)
	require.Equal(t, trackW.TimeBase, trackR.TimeBase)
	require.Equal(t, trackW.Codec, trackR.Codec)
	require.Equal(t, trackW.Width, trackR.Width)
	require.Equal(t, trackW.Height, trackR.Height)
	require.Equal(t, trackR.isWriting, false)
	require.Equal(t, trackR.indexCount, nNALUs)
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
			timeDiff := math.Abs(float64(nalusW[i+j].PTS.Sub(nalusR[j].PTS)))
			require.Less(t, timeDiff, float64(time.Second)/4096)
			require.Equal(t, nalusW[i+j].Flags, nalusR[j].Flags)
			require.EqualValues(t, len(nalusW[i+j].Payload), nalusR[j].Length)
		}
		err = trackR.ReadPayload(nalusR)
		require.NoError(t, err)
		for j := 0; j < len(nalusR); j++ {
			require.Equal(t, nalusW[i+j].Payload, nalusR[j].Payload)
		}
	}
}

func TestMain(m *testing.M) {
	// Setup
	if err := os.MkdirAll(BaseDir, 0755); err != nil {
		fmt.Printf("Error creating test directory '%v': %v\n", BaseDir, err)
		os.Exit(1)
	}

	code := m.Run()

	// Teardown
	os.RemoveAll(BaseDir)

	os.Exit(code)
}
