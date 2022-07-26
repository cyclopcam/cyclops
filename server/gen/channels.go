package gen

// DrainChannelIntoSlice reads from a channel until it is empty, and returns all items in a slice
func DrainChannelIntoSlice[T any](ch chan T) []T {
	done := false
	slice := make([]T, 0, len(ch)) // optimize for the common case where we're the only reader
	for !done {
		select {
		case v := <-ch:
			slice = append(slice, v)
		default:
			done = true
		}
	}
	return slice
}
