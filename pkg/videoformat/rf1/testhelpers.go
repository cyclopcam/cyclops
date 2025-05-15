package rf1

import (
	"math/rand"
	"time"
)

// Number of frames between keyframes
const TestKeyframeInterval = 30
const TestNALUKeyframeInterval = TestKeyframeInterval + 2

// Every 32 NALUs is a keyframe.
// A keyframe is two EssentialMeta NALUs followed by a keyframe.
// SYNC-TEST-META-COUNT
func CreateTestNALU(naluIdx int, fps int) (IndexNALUFlags, time.Duration) {
	// It's vital that the PTS of all the NALUs that belong to the keyframe are the same,
	// so that's why our PTS computation looks a bit complicated.
	// When we seek back to a keyframe, we use the PTS to bind it to the metadata NALUs that precede it.
	// We initially used the EssentialMeta flag, and that would still work. But PTS just seemed simpler.

	// Number of NALUs between keyframes is 30 + 2, because we have an additional 2 for the SPS and PPS NALUs.
	const naluKeyframeInterval = TestKeyframeInterval + 2

	div := naluIdx / naluKeyframeInterval
	res := naluIdx % naluKeyframeInterval

	flags := IndexNALUFlagAnnexB
	if res == 0 {
		flags |= IndexNALUFlagEssentialMeta
	} else if res == 1 {
		flags |= IndexNALUFlagEssentialMeta
	} else if res == 2 {
		flags |= IndexNALUFlagKeyFrame
	}

	// Time of the most recent keyframe
	keyframeTimeBase := float64(div*TestKeyframeInterval) / float64(fps)
	offset := max(0, float64(res-2)) / float64(fps)
	pts := time.Duration((keyframeTimeBase + offset) * float64(time.Second))
	return flags, pts
}

// Generate the range of NALUs [startNALU, endNALU)
// NALU flags are controlled by TestNALUFlags()
// seed should be a prime number
func CreateTestNALUs(timeBase time.Time, startNALU, endNALU int, fps int, minPacketSize, maxPacketSize int, seed int) []NALU {
	if seed <= 1 {
		panic("seed must be a prime number")
	}
	if maxPacketSize < minPacketSize {
		panic("maxPacketSize must be greater than or equal to minPacketSize")
	}
	nalus := make([]NALU, endNALU-startNALU)
	rng := rand.New(rand.NewSource(int64(seed)))
	for i := startNALU; i < endNALU; i++ {
		flags, pts := CreateTestNALU(i, fps)
		nalu := NALU{
			PTS:   timeBase.Add(pts),
			Flags: flags,
		}
		rng.Seed(int64(i+1) * int64(seed))
		packetSize := minPacketSize
		if maxPacketSize > minPacketSize {
			packetSize = rng.Intn(maxPacketSize-minPacketSize) + minPacketSize
		}
		nalu.Payload = make([]byte, packetSize)
		_, err := rng.Read(nalu.Payload)
		if err != nil {
			panic(err)
		}
		nalus[i-startNALU] = nalu
	}
	return nalus
}
