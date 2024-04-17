package rf1

// For some reason having an os_windows.go file causes the Go language server to break on linux/WSL

//import (
//	"os"
//)
//
//// Pre-allocate space for a file, to avoid fragmentation
//func PreallocateFile(f *os.File, size int64) error {
//	// I have no idea whether this helps on Windows
//	return f.Truncate(size)
//}
