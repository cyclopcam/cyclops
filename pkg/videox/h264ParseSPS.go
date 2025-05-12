package videox

import "unsafe"

// #include "h264ParseSPS.h"
import "C"

// Parse a raw SPS NALU (not annex-b!!!)
// On Rpi5, this takes 305ns for a 50 byte SPS packet, which is typical on my Hikvisions.
// On AMD Ryzen 9 5900X, this takes 94ns
func ParseH264SPS(nalu []byte) (width, height int, err error) {
	var cwidth C.int
	var cheight C.int
	C.ParseH264SPS(unsafe.Pointer(&nalu[0]), C.ulong(len(nalu)), &cwidth, &cheight)
	width = int(cwidth)
	height = int(cheight)
	return
}

// Parse a raw SPS NALU (not annex-b!!!)
func ParseH265SPS(nalu []byte) (width, height int, err error) {
	var cwidth C.int
	var cheight C.int
	C.ParseH265SPS(unsafe.Pointer(&nalu[0]), C.ulong(len(nalu)), &cwidth, &cheight)
	width = int(cwidth)
	height = int(cheight)
	return
}
