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

// Drain the channel and discard the contents
func DrainChannel[T any](ch chan T) {
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				// The channel is closed
				return
			}
		default:
			// The channel is empty
			return
		}
	}
}

func IsChannelClosed[T any](ch chan T) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

func WaitForChannelToClose[T any](ch chan T) {
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
		}
	}
}
