package gen

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
