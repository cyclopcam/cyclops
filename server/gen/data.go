// Package gen contains a bunch of generic functions that will probably be in the Go std lib someday
package gen

// Return a copy of the slice
func CopySlice[T any](src []T) []T {
	dst := make([]T, len(src))
	copy(dst, src)
	return dst
}

// Return a copy of the map
func CopyMap[K comparable, V any](src map[K]V) map[K]V {
	dst := make(map[K]V, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// Finds an element in the slice and returns a copy with that element removed.
// If the element does not exist, returns the original slice.
// If the element exists more than once, removes only the first one.
func DeleteFirst[T comparable](slice []T, elem T) []T {
	for i := 0; i < len(slice); i++ {
		if slice[i] == elem {
			return append(slice[0:i], slice[i+1:]...)
		}
	}
	return slice
}

// Deletes the i'th element of the slice, and returns a new slice.
// Preserves order, but is much slower than DeleteFromSliceUnordered.
func DeleteFromSliceOrdered[T any](src []T, i int) []T {
	return append(src[:i], src[i+1:]...)
}

// Deletes the i'th element of the slice, and returns a new slice.
// Does not preserve order, but is much faster than DeleteFromSliceOrdered.
func DeleteFromSliceUnordered[T any](src []T, i int) []T {
	src[i] = src[len(src)-1]
	return src[:len(src)-1]
}

// Returns the index of the first 'v' in 'src', or -1 if not found
func IndexOf[T comparable](src []T, v T) int {
	for i := range src {
		if src[i] == v {
			return i
		}
	}
	return -1
}
