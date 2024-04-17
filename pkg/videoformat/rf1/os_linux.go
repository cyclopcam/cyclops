package rf1

import (
	"os"
	"syscall"
)

/*
#include <fcntl.h>
#include <errno.h>
*/
import "C"

// Pre-allocate space for a file, to avoid fragmentation
func PreallocateFile(f *os.File, size int64) error {
	fd := C.int(f.Fd())
	ret := C.posix_fallocate(fd, 0, C.off_t(size))
	if ret != 0 {
		// posix_fallocate returns the error number directly, so we convert it to a Go error.
		return syscall.Errno(ret)
	}
	return nil
}
