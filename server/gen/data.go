// Package gen contains a bunch of generic functions that will probably be in the Go std lib someday
package gen

// Return a copy of the slice
func CopySlice[T any](src []T) []T {
	dst := make([]T, len(src))
	copy(dst, src)
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
