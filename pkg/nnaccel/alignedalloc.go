package nnaccel

import (
	"unsafe"

	"golang.org/x/sys/unix"
)

// System page size. Read at startup.
var pageSize uintptr

// Allocate 'size' bytes of memory, aligned to a page boundary.
func PageAlignedAlloc(size int) []byte {
	raw := make([]byte, size+int(pageSize))
	offset := pageSize - (uintptr(unsafe.Pointer(&raw[0])) % pageSize)
	return raw[offset : int(offset)+size]
}

// Returns the system page size
func PageSize() int {
	return int(pageSize)
}

// Round size up to the nearest page size
func RoundUpToPageSize(size int) int {
	return int((uintptr(size) + pageSize - 1) & ^(pageSize - 1))
}

func init() {
	pageSize = uintptr(unix.Getpagesize())
}
