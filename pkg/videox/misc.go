package videox

// #include "misc.h"
// #include <stdio.h>
// #include <stdlib.h>
// #include <stdint.h>
import "C"
import (
	"unsafe"
)

// This is the prefix that we add whenever we need to encode into AnnexB
// This must remain in sync with the behaviour inside EncodeAnnexB()
var NALUPrefix = []byte{0x00, 0x00, 0x01}

// Encode an RBSP (Raw Byte Sequence Packet) into Annex-B format, optionally adding
// a 3 byte start code (00.00.01) to the beginning of the encoded byte stream.
// This encoding adds the "emulation prevention byte" (0x03) where necessary.
func EncodeAnnexB(raw []byte, addNALUStartCode bool) []byte {
	startCodeSize := 0
	if addNALUStartCode {
		startCodeSize = 3
	}

	// optimistic first pass, assuming 1% expansion
	// +8 is for small SPS/PPS NALs
	dst := make([]byte, startCodeSize+8+len(raw)*101/100)
	dstSize, dstOK := EncodeAnnexBInto(raw, addNALUStartCode, dst)
	if dstOK {
		return dst[:dstSize]
	}

	// pessimistic second pass
	dst = make([]byte, startCodeSize+len(raw)*3/2)
	dstSize, dstOK = EncodeAnnexBInto(raw, addNALUStartCode, dst)
	if !dstOK {
		panic("EncodeAnnexB failed - buffer never large enough")
	}
	return dst[:dstSize]
}

// Encode an RBSP (Raw Byte Sequence Packet) into Annex-B format, optionally adding
// a 3 byte start code (00.00.01) to the beginning of the encoded byte stream.
// This encoding adds the "emulation prevention byte" (0x03) where necessary.
func EncodeAnnexBInto(raw []byte, addNALUPrefix bool, dst []byte) (encodedSize int, bufferSizeOK bool) {
	r := C.ulong(0)
	if addNALUPrefix {
		if len(dst) < 3 {
			return 0, false
		}
		// start code (must be same length as NALUPrefix)
		dst[0] = 0
		dst[1] = 0
		dst[2] = 1
		if len(raw) != 0 {
			r = C.EncodeAnnexB(unsafe.Pointer(&raw[0]), C.ulong(len(raw)), unsafe.Pointer(&dst[3]), C.ulong(len(dst)-3))
		}
	} else {
		if len(raw) != 0 {
			r = C.EncodeAnnexB(unsafe.Pointer(&raw[0]), C.ulong(len(raw)), unsafe.Pointer(&dst[0]), C.ulong(len(dst)))
		}
	}
	if r == 0 && len(raw) != 0 {
		// Need more buffer space
		return 0, false
	} else {
		// Success
		if addNALUPrefix {
			return 3 + int(r), true
		} else {
			return int(r), true
		}
	}
}

// Decode an Annex-B encoded packet into a Raw Byte Sequence Payload (RBSP).
// We assume that you're handling the 3 or 4 byte NALU prefix outside of this function.
func DecodeAnnexB(encoded []byte) []byte {
	decoded := make([]byte, len(encoded))
	if len(encoded) != 0 {
		r := C.DecodeAnnexB(unsafe.Pointer(&encoded[0]), C.ulong(len(encoded)), unsafe.Pointer(&decoded[0]), C.ulong(len(decoded)))
		if r == 0 && len(encoded) != 0 {
			panic("Decode NALU Annex-B failed")
		}
		return decoded[:int(r)]
	}
	return decoded
}

// Return the worst case size of an Annex-B encoded packet, given the size of the raw packet (including a 3 byte start code).
func AnnexBWorstSize(rawLen int) int {
	return len(NALUPrefix) + rawLen*3/2
}
