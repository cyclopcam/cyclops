package camera

import (
	"math"
	"slices"
	"time"
)

// Given a set of consecutive frame intervals, estimate the average frames per second.
// The value is a float64 because cameras can be configured for less than 1 FPS.
// The numbers I've seen on Hikvision are 1/2, 1/4, 1/8, 1/16
func EstimateFPS(frameIntervals []time.Duration) float64 {
	if len(frameIntervals) == 0 {
		return 10
	}
	sorted := make([]time.Duration, len(frameIntervals))
	copy(sorted, frameIntervals)
	slices.Sort(sorted)
	mid := sorted[len(sorted)/2]
	if mid == 0 {
		return 10
	}
	fps := float64(time.Second) / float64(mid)
	if fps >= 0.9 {
		return math.Round(fps)
	}
	// Below 1 FPS, we round to the nearest 1/2/4/8/16
	// This is because cameras can be configured for less than 1 FPS
	secondsPerFrame := 1.0 / fps
	spfR := math.Round(secondsPerFrame)
	return 1 / spfR
}
