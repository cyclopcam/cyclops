package videox

import (
	"github.com/bluenviron/mediacommon/pkg/codecs/h264"
	"github.com/bluenviron/mediacommon/pkg/codecs/h265"
)

// Note that h264/h265 talks about the following two types of NALUs:
// * RBSP Raw Byte Sequence Payload (No start code, no emulation prevention bytes)
// * SODB String of Data Bits (Annex-B encoding. Has start code and emulation prevention bytes)
// But see the long comment above ($ANNEXB-CONFUSION).
// TL;DR: Just because a NALU has no start code, doesn't mean it has no emulation prevention bytes.

// Codec NALU
type NALU struct {
	PayloadIsAnnexB  bool // True if the payload is escaped with "emulation prevention bytes", for example with 00 00 03 01 replacing 00 00 00 01
	PayloadNoEscapes bool // True if PayloadIsAnnexB BUT we know that we have no "emulation prevention bytes", so we can avoid decoding them.
	Payload          []byte
}

// Wrap a raw buffer in a NALU object. Do not clone memory, or add prefix bytes.
func WrapRawNALU(raw []byte) NALU {
	return NALU{
		Payload: raw,
	}
}

// Returns true if the NALU has a start code, and the payload is encoded with emulation prevention bytes
func (n *NALU) IsAnnexBWithStartCode() bool {
	return n.PayloadIsAnnexB && n.StartCodeLen() != 0
}

// Returns true if the NALU has no start code, and the payload is not encoded with emulation prevention bytes
func (n *NALU) IsRBSPWithNoStartCode() bool {
	return !n.PayloadIsAnnexB && n.StartCodeLen() == 0
}

// Returns only the payload, without any start code
func (n *NALU) PayloadOnly() []byte {
	return n.Payload[n.StartCodeLen():]
}

// Returns length of start code
// Possible return values:
// 0: No start code
// 3: 00 00 01
// 4: 00 00 00 01
// I can't recall precisely now, but I think this function covers all possible
// legal NALU beginnings. But I am uncomfortable now with this. We should maybe
// store a flag in the NALU indicating whether it has a start code or not.
func (n *NALU) StartCodeLen() int {
	if len(n.Payload) < 3 {
		return 0
	}
	if n.Payload[0] == 0 && n.Payload[1] == 0 && n.Payload[2] == 1 {
		return 3
	}
	if len(n.Payload) < 4 {
		return 0
	}
	if n.Payload[0] == 0 && n.Payload[1] == 0 && n.Payload[2] == 0 && n.Payload[3] == 1 {
		return 4
	}
	return 0
}

func (n *NALU) DeepClone() NALU {
	return NALU{
		Payload:          append([]byte{}, n.Payload...),
		PayloadIsAnnexB:  n.PayloadIsAnnexB,
		PayloadNoEscapes: n.PayloadNoEscapes,
	}
}

// Return payload data, but make sure it's in AnnexB format, and has a start code of 00.00.01 or 00.00.00.01
func (n *NALU) AsAnnexB() NALU {
	if n.PayloadIsAnnexB {
		// We're already in AnnexB format, so the only thing we might need to take care of is adding a start code
		if n.StartCodeLen() == 0 {
			// Add a 3 byte start code
			return NALU{
				Payload:          append(NALUStartCode(3), n.Payload...),
				PayloadIsAnnexB:  true,
				PayloadNoEscapes: n.PayloadNoEscapes,
			}
		} else {
			return *n
		}
	} else {
		// Encode to AnnexB
		startCodeLen := 3
		rawLen := len(n.Payload) - n.StartCodeLen()
		payload := EncodeAnnexB(n.Payload[n.StartCodeLen():], startCodeLen, AnnexBEncodeFlagAddEmulationPreventionBytes)
		return NALU{
			Payload:          payload,
			PayloadIsAnnexB:  true,
			PayloadNoEscapes: len(payload) == rawLen+startCodeLen,
		}
	}
}

// Return payload data, but make sure it's in RBSP format, with no start code
func (n *NALU) AsRBSP() NALU {
	if !n.PayloadIsAnnexB {
		// Content is already raw
		return NALU{
			Payload: n.Payload[n.StartCodeLen():],
		}
	} else {
		// Decode to RBSP
		if n.PayloadNoEscapes {
			// Special optimization where we know that we don't need to do any unescaping
			return NALU{
				Payload: n.Payload[n.StartCodeLen():],
			}
		} else {
			return NALU{
				Payload: DecodeAnnexB(n.Payload[n.StartCodeLen():]),
			}
		}
	}
}

func (n *NALU) AbstractType(codec Codec) AbstractNALUType {
	p := n.PayloadOnly()
	switch codec {
	case CodecH264:
		return H264ToAbstractType(p[0])
	case CodecH265:
		return H265ToAbstractType(p[0])
	}
	panic("Codec not specified")
}

// Return the NALU type (from the first byte of the header)
func (n *NALU) Type(codec Codec) byte {
	switch codec {
	case CodecH264:
		return byte(n.Type264())
	case CodecH265:
		return byte(n.Type265())
	}
	panic("Codec not specified")
}

// Return the NALU type
func (n *NALU) Type264() h264.NALUType {
	i := n.StartCodeLen()
	if i >= len(n.Payload) {
		return h264.NALUType(0)
	}
	return ReadNaluTypeH264(n.Payload[i])
}

// Return the NALU type
func (n *NALU) Type265() h265.NALUType {
	i := n.StartCodeLen()
	if i >= len(n.Payload) {
		return h265.NALUType(255)
	}
	return ReadNaluTypeH265(n.Payload[i])
}
