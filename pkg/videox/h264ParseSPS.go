package videox

import "unsafe"

// #include "h264ParseSPS.h"
import "C"

// Parse a raw SPS NALU (not annex-b)
func ParseH264SPS(nalu []byte) (width, height int, err error) {
	var cwidth C.int
	var cheight C.int
	C.ParseH264SPS(unsafe.Pointer(&nalu[0]), C.ulong(len(nalu)), &cwidth, &cheight)
	width = int(cwidth)
	height = int(cheight)
	return
}
