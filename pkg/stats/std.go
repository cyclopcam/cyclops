package stats

// Returns (mean, variance) of the given samples.
func MeanVar[T Float | Integer](samples []T) (float64, float64) {
	mean := Mean(samples)
	variance := Variance(samples, mean)
	return mean, variance
}

// Returns the mean of the given samples.
func Mean[T Float | Integer](samples []T) float64 {
	sum := 0.0
	for _, v := range samples {
		sum += float64(v)
	}
	return sum / float64(len(samples))
}

// Returns the variance of the given samples.
func Variance[T Float | Integer](samples []T, mean float64) float64 {
	sum := 0.0
	for _, v := range samples {
		diff := float64(v) - mean
		sum += diff * diff
	}
	return sum / float64(len(samples))
}

// Returns the mode and count of the most frequent element in the given samples.
func Mode[T comparable](src []T) (mode T, count int) {
	counts := make(map[T]int)
	for _, v := range src {
		counts[v]++
	}
	for k, v := range counts {
		if v > count {
			mode = k
			count = v
		}
	}
	return
}
