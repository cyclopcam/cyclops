// Package perfstats is a single place where we record the performance of various
// operations, so that it's easy to compare different solutions and
// the performance of different hardware.
package perfstats

import (
	"fmt"
	"strings"
	"sync/atomic"
)

/*
Using ffmpeg for YUV2RGB

Ryzen 5900X 3.7 Ghz
YUV420ToRGB 320x240 frame: 0.0227 ms

Raspberry Pi 4
YUV420ToRGB 320x240 frame: 0.5466 ms  (ffmpeg swscale)
YUV420ToRGB 320x240 frame: 0.3920 ms  (Simd library)

To get ms per second, we multiply by 10 * 4 = 40 (for 10 frames per second, and 4 cameras).
0.0227 * 40 =  1 ms overhead per second, to decode YUV to RGB (Ryzen 5900X).
0.5466 * 40 = 22 ms overhead per second, to decode YUV to RGB (Raspberry Pi).
	BUT.. if we have 4 cores, then we should think of it as 22/4 = 5.5ms decode overhead, or 0.5% of runtime.

*/

type PerfStats struct {
	YUV420ToRGB_NanosecondsPerKibiPixel atomic.Uint64
}

var Stats = PerfStats{}

func Update(stat *atomic.Uint64, value int64) {
	vu := uint64(value)
	// We don't bother about strict correctness here, with CompareAndSwap,
	// because this is just sampled stats, and it's OK to miss one or two samples.
	if stat.Load() == 0 {
		stat.Store(vu)
	} else {
		stat.Store((stat.Load()*63 + vu) >> 6)
	}
}

func (s *PerfStats) String() string {
	b := &strings.Builder{}
	fmt.Fprintf(b, "YUV420ToRGB 320x240 frame: %0.4f ms", float64(s.YUV420ToRGB_NanosecondsPerKibiPixel.Load())*(320*240/1024)/1000000)
	return b.String()
}
