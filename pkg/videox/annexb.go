package videox

// #include "annexb.h"
// #include <stdio.h>
// #include <stdlib.h>
// #include <stdint.h>
import "C"
import (
	"unsafe"
)

// Flags that control how EncodeAnnexB works
type AnnexBEncodeFlags int

const (
	AnnexBEncodeFlagNone                        AnnexBEncodeFlags = 0 // This is nonsensical - it is simply a memcpy
	AnnexBEncodeFlagAddEmulationPreventionBytes AnnexBEncodeFlags = 1 // Add emulation prevention bytes (0x03) where necessary
)

func NALUStartCode(length int) []byte {
	if length == 0 {
		return nil
	} else if length == 3 {
		return []byte{0, 0, 1}
	} else if length == 4 {
		return []byte{0, 0, 0, 1}
	} else {
		panic("Invalid NALU start code length")
	}
}

// Encode an RBSP (Raw Byte Sequence Packet) into Annex-B format, optionally adding
// a 3 or 4 byte start code (00.00.01 or 00.00.00.01) to the beginning of the encoded byte stream.
// Also, we adds the "emulation prevention byte" (0x03) where necessary, if the relevant flag is set.
// If startCodeLen is zero, then we do not add a start code
func EncodeAnnexB(raw []byte, startCodeLen int, flags AnnexBEncodeFlags) []byte {
	// optimistic first pass, assuming 1% expansion
	// +8 is for small SPS/PPS NALs
	dst := make([]byte, startCodeLen+8+len(raw)*101/100)
	dstSize, dstOK := EncodeAnnexBInto(raw, startCodeLen, flags, dst)
	if dstOK {
		return dst[:dstSize]
	}

	// pessimistic second pass
	dst = make([]byte, startCodeLen+len(raw)*3/2)
	dstSize, dstOK = EncodeAnnexBInto(raw, startCodeLen, flags, dst)
	if !dstOK {
		panic("EncodeAnnexB failed - buffer never large enough")
	}
	return dst[:dstSize]
}

// Encode an RBSP (Raw Byte Sequence Packet) into Annex-B format, optionally adding
// a 3 byte start code (00.00.01) to the beginning of the encoded byte stream.
// This encoding adds the "emulation prevention byte" (0x03) where necessary.
func EncodeAnnexBInto(raw []byte, startCodeLen int, flags AnnexBEncodeFlags, dst []byte) (encodedSize int, bufferSizeOK bool) {
	if startCodeLen != 0 && startCodeLen != 3 && startCodeLen != 4 {
		panic("Invalid startCodeLen. Must be one of 0,3,4")
	}

	r := C.size_t(0)
	addEmulationPreventionBytes := flags&AnnexBEncodeFlagAddEmulationPreventionBytes != 0
	if startCodeLen != 0 {
		if len(dst) < startCodeLen {
			return 0, false
		}
		// start code (must be same length as NALUPrefix)
		if startCodeLen == 3 {
			dst[0] = 0
			dst[1] = 0
			dst[2] = 1
		} else {
			dst[0] = 0
			dst[1] = 0
			dst[2] = 0
			dst[3] = 1
		}
		if len(raw) != 0 {
			if addEmulationPreventionBytes {
				r = C.EncodeAnnexB(unsafe.Pointer(&raw[0]), C.size_t(len(raw)), unsafe.Pointer(&dst[startCodeLen]), C.size_t(len(dst)-startCodeLen))
			} else {
				r = C.EncodeAnnexB_Memcpy(unsafe.Pointer(&raw[0]), C.size_t(len(raw)), unsafe.Pointer(&dst[startCodeLen]), C.size_t(len(dst)-startCodeLen))
			}
		}
	} else {
		if len(raw) != 0 {
			if addEmulationPreventionBytes {
				r = C.EncodeAnnexB(unsafe.Pointer(&raw[0]), C.size_t(len(raw)), unsafe.Pointer(&dst[0]), C.size_t(len(dst)))
			} else {
				r = C.EncodeAnnexB_Memcpy(unsafe.Pointer(&raw[0]), C.size_t(len(raw)), unsafe.Pointer(&dst[0]), C.size_t(len(dst)))
			}
		}
	}
	if r == 0 && len(raw) != 0 {
		// Need more buffer space
		return 0, false
	} else {
		// Success
		return startCodeLen + int(r), true
	}
}

// Decode an Annex-B encoded packet into a Raw Byte Sequence Payload (RBSP).
// We assume that you're handling the 3 or 4 byte NALU prefix outside of this function.
func DecodeAnnexB(encoded []byte) []byte {
	decoded := make([]byte, len(encoded))
	if len(encoded) != 0 {
		r := C.DecodeAnnexB(unsafe.Pointer(&encoded[0]), C.size_t(len(encoded)), unsafe.Pointer(&decoded[0]), C.size_t(len(decoded)))
		if r == 0 && len(encoded) != 0 {
			panic("Decode NALU Annex-B failed")
		}
		return decoded[:int(r)]
	}
	return decoded
}

// Return the number of bytes needed to decode an Annex-B encoded packet.
// This function is for analysis of camera streams.
// In ordinary usage, we just call DecodeAnnexB().
func DecodeAnnexBSize(encoded []byte) int {
	if len(encoded) != 0 {
		r := C.DecodeAnnexB_Size(unsafe.Pointer(&encoded[0]), C.size_t(len(encoded)))
		return int(r)
	}
	return 0
}

func FirstLikelyAnnexBEncodedIndex(encoded []byte) int {
	if len(encoded) < 3 {
		return -1
	}
	// Look for 00.00.03.XX where XX is one of 00,01,02,03
	sum := int(encoded[0]) + int(encoded[1]) + int(encoded[2])
	for i := 3; i < len(encoded)-1; i++ {
		sum = sum - int(encoded[i-3]) + int(encoded[i])
		if sum == 3 && encoded[i-2] == 0 && encoded[i-1] == 0 && encoded[i+1] <= 3 {
			return i - 2
		}
	}
	return -1
}

// Return the worst case size of an Annex-B encoded packet, given the size of the raw packet (including a 3 byte start code).
func AnnexBWorstSize(startCodeLen, rawLen int) int {
	return startCodeLen + rawLen*3/2
}
