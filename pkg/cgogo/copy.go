package cgogo

import (
	"io"
	"unsafe"
)

// Copy from Go []byte slice to a C array.
// Note that dstLenBytes is in *bytes*, not in elements.
//func CopyToCSlice[TDst any](dst []TDst, dstLenBytes int, src []byte) {
//	dstSlice := unsafe.Slice((*byte)(unsafe.Pointer(&dst[0])), dstLenBytes)
//	copy(dstSlice, src)
//}

// Copy a block of memory from Go to C.
//func CopyToCSlice[TDst any](dst []TDst, src []byte) {
//	dstSlice := unsafe.Slice((*byte)(unsafe.Pointer(&dst[0])), int(unsafe.Sizeof(dst[0]))*len(dst))
//	copy(dstSlice, src)
//}

// Copy a block of memory from Go to C.
// To copy a Go string to C, cast the string to []byte.
func CopySlice[TDst any, TSrc any](dst []TDst, src []TSrc) {
	dstSlice := unsafe.Slice((*byte)(unsafe.Pointer(&dst[0])), int(unsafe.Sizeof(dst[0]))*len(dst))
	srcSlice := unsafe.Slice((*byte)(unsafe.Pointer(&src[0])), int(unsafe.Sizeof(src[0]))*len(src))
	copy(dstSlice, srcSlice)
}

func WriteSlice[TSrc any](w io.Writer, src []TSrc) (int, error) {
	return w.Write(unsafe.Slice((*byte)(unsafe.Pointer(&src[0])), int(unsafe.Sizeof(src[0]))*len(src)))
}

func WriteSliceAt[TSrc any](w io.WriterAt, src []TSrc, offset int64) (int, error) {
	return w.WriteAt(unsafe.Slice((*byte)(unsafe.Pointer(&src[0])), int(unsafe.Sizeof(src[0]))*len(src)), offset)
}

func ReadSlice[TDst any](r io.Reader, dst []TDst) (int, error) {
	return r.Read(unsafe.Slice((*byte)(unsafe.Pointer(&dst[0])), int(unsafe.Sizeof(dst[0]))*len(dst)))
}

func ReadSliceAt[TDst any](r io.ReaderAt, dst []TDst, offset int64) (int, error) {
	return r.ReadAt(unsafe.Slice((*byte)(unsafe.Pointer(&dst[0])), int(unsafe.Sizeof(dst[0]))*len(dst)), offset)
}

func WriteStruct[TSrc any](w io.Writer, src *TSrc) (int, error) {
	return w.Write(unsafe.Slice((*byte)(unsafe.Pointer(src)), int(unsafe.Sizeof(*src))))
}

func WriteStructAt[TSrc any](w io.WriterAt, src *TSrc, offset int64) (int, error) {
	return w.WriteAt(unsafe.Slice((*byte)(unsafe.Pointer(src)), int(unsafe.Sizeof(*src))), offset)
}

func ReadStruct[TDst any](r io.Reader, dst *TDst) (int, error) {
	return r.Read(unsafe.Slice((*byte)(unsafe.Pointer(dst)), int(unsafe.Sizeof(*dst))))
}

func ReadStructAt[TDst any](r io.ReaderAt, dst *TDst, offset int64) (int, error) {
	return r.ReadAt(unsafe.Slice((*byte)(unsafe.Pointer(dst)), int(unsafe.Sizeof(*dst))), offset)
}
