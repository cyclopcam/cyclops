package nnaccel

import (
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"
)

func TestAlignedAlloc(t *testing.T) {
	for _, size := range []int{1, 2, 3, 4, 5, 99, 100, 4095, 4096, 4097, 16384, 16385, 16386, 300000} {
		buf := PageAlignedAlloc(size)
		require.Equal(t, size, len(buf))
		require.Equal(t, 0, int(uintptr(unsafe.Pointer(&buf[0]))%pageSize))
	}
}

// I'm just curious to see how long it takes to allocate a buffer for a 640x640x3 image.
// On Rpi5 it is 198 microseconds, aka 0.2 milliseconds. That's quite a penalty!
// See the mem clear benchmark below
func BenchmarkAlignedAlloc(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = PageAlignedAlloc(640 * 640 * 3)
	}
}

// This is 70 microseconds - i.e. 3x as fast as the allocation.
// So my conclusion is that we should keep a memory pool around for images
// that we're sending off to the NN.
func BenchmarkClearMem(b *testing.B) {
	buf := PageAlignedAlloc(640 * 640 * 3)
	for i := 0; i < b.N; i++ {
		clear(buf)
	}
}
