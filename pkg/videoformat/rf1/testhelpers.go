package rf1

import (
	"math/rand"
	"time"
)

// The first 2 NALUs are EssentialMetadata
// The 3rd NALU is a keyframe, and every 10th frame thereafter is a keyframe
func TestNALUFlags(naluIdx int) IndexNALUFlags {
	if naluIdx < 2 {
		return IndexNALUFlagEssentialMeta
	} else if naluIdx%10 == 2 {
		return IndexNALUFlagKeyFrame
	}
	return 0
}

// Generate the range of frames [startFrame, endFrame)
// Frame flags are controlled by TestNALUFlags()
// seed should be a prime number
func CreateTestNALUs(timeBase time.Time, startFrame, endFrame int, fps float64, minPacketSize, maxPacketSize int, seed int) []NALU {
	if seed <= 1 {
		panic("seed must be a prime number")
	}
	if maxPacketSize < minPacketSize {
		panic("maxPacketSize must be greater than or equal to minPacketSize")
	}
	nalus := make([]NALU, endFrame-startFrame)
	rng := rand.New(rand.NewSource(int64(seed)))
	for i := startFrame; i < endFrame; i++ {
		pts := time.Duration(float64(i) * float64(time.Second) / fps)
		nalu := NALU{
			PTS: timeBase.Add(pts),
		}
		nalu.Flags = TestNALUFlags(i)
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
		nalus[i-startFrame] = nalu
	}
	return nalus
}
