package perfstats

import "time"

// Two scalars (N samples and X total amount), which can measure total and average values.
type Accumulator struct {
	Samples int64
	Total   float64
}

func (a *Accumulator) Reset() {
	a.Samples = 0
	a.Total = 0
}

func (a *Accumulator) AddSample(v float64) {
	a.Samples++
	a.Total += v
}

func (a *Accumulator) Average() float64 {
	if a.Samples == 0 {
		return 0
	}
	return a.Total / float64(a.Samples)
}

// Two scalars (N samples and X total amount), which can measure total and average values.
type Int64Accumulator struct {
	Samples int64
	Total   int64
}

func (a *Int64Accumulator) Reset() {
	a.Samples = 0
	a.Total = 0
}

func (a *Int64Accumulator) AddSample(v int64) {
	a.Samples++
	a.Total += v
}

func (a *Int64Accumulator) Average() float64 {
	if a.Samples == 0 {
		return 0
	}
	return float64(a.Total) / float64(a.Samples)
}

// Accumulate samples of how long something took
type TimeAccumulator struct {
	Samples int64
	Total   time.Duration
}

func (a *TimeAccumulator) Reset() {
	a.Samples = 0
	a.Total = 0
}

func (a *TimeAccumulator) AddSample(v time.Duration) {
	a.Samples++
	a.Total += v
}

func (a *TimeAccumulator) Average() time.Duration {
	if a.Samples == 0 {
		return 0
	}
	return time.Duration(a.Total.Nanoseconds() / a.Samples)
}
