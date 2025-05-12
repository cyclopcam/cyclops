package videox

import (
	"fmt"
	"strings"
	"time"

	"github.com/bluenviron/mediacommon/pkg/codecs/h264"
)

// Topic: $ANNEXB-CONFUSION
// Here's the story:
// When we receive packets from Hikvision cameras, via github.com/bluenviron/gortsplib, the packets
// are supposedly NALUFormatRBSP, aka raw data bits, with no start codes, and no emulation prevention bytes.
// The codecs seem to want packets in SODB (aka AnnexB) encoding, so we dutifully encode the raw packets
// into AnnexB, with emulation prevention bytes added. HOWEVER, when we activate this code path,
// we get sporadic errors from ffmpeg, telling us that we've got bad frames. If we comment out the
// code that does the emulation prevention byte injection, then these errors go away.
// To be clear, we must inject the start codes. This is unambiguous. It's the emulation prevention bytes
// that cause errors.
// This confusion is the reason for this constant. At some point we'll hopefully learn more, and make
// better sense of this.
// Right now the culprit could be any one of these:
// 1. HikVision cameras
// 2. gortsplib
// 3. The way I'm using the h264 codec in ffmpeg
// 4. My SODB/Annex-B encoder
// 5. My understanding
// ------------------------
// UPDATE WITH ANSWER
// ------------------------
// I have come to the conclusion that my Hikvision cameras are sending data with emulation prevention bytes
// added to the byte stream, but without start codes.
// So this has led me to store two pieces of state with each NALU:
// 1. Does it have a start code?
// 2. How is the payload encoded?
// I initially thought that the presence of a start code should be synonymous with the presence of emulation
// prevention bytes, but I've learned that this is not the case.
const EnableEmulationPreventBytesEscaping = true

// VideoPacket is one or more NALUs that were received together.
// There is generally structure to this. For example, a keyframe packet from a camera
// will likely contain a SPS, PPS, and IDR NALU. For H265, it may also contain a VPS.
type VideoPacket struct {
	ValidRecvID int64         // Arbitrary monotonically increasing ID of useful decoded packets. Used to detect dropped packets, or other issues like that.
	Codec       Codec         // h264 or h265
	NALUs       []NALU        // NALUs in the packet.
	PTS         time.Duration // Raw packet PTS received from RTSP reader. Subtracted from a reference time to compute WallPTS.
	WallPTS     time.Time     // Reference wall time combined with the received PTS. We consider this the ground truth/reality of when the packet was recorded on the camera.
	IsBacklog   bool          // a bit of a hack to inject this state here. maybe an integer counter would suffice? (eg nBacklogPackets)
}

// Deep clone of packet buffer
func (p *VideoPacket) Clone() *VideoPacket {
	c := &VideoPacket{
		ValidRecvID: p.ValidRecvID,
		PTS:         p.PTS,
		WallPTS:     p.WallPTS,
		IsBacklog:   p.IsBacklog,
	}
	c.NALUs = make([]NALU, len(p.NALUs))
	for i, n := range p.NALUs {
		c.NALUs[i] = n.DeepClone()
	}
	return c
}

// Return true if this packet has a NALU of type t inside
func (p *VideoPacket) HasAbstractType(t AbstractNALUType) bool {
	for _, n := range p.NALUs {
		if n.AbstractType(p.Codec) == t {
			return true
		}
	}
	return false
}

// Returns true if this packet has a keyframe
func (p *VideoPacket) HasIDR() bool {
	return p.HasAbstractType(AbstractNALUTypeIDR)
}

// Return true if this packet has one NALU which is an intermediate frame
func (p *VideoPacket) IsIFrame() bool {
	//return len(p.NALUs) == 1 && p.NALUs[0].Type() == h264.NALUTypeNonIDR
	return len(p.NALUs) == 1 && p.NALUs[0].AbstractType(p.Codec) == AbstractNALUTypeNonIDR
}

// Returns the first NALU of the given type, or nil if none exists
func (p *VideoPacket) FirstNALUOfType264(t h264.NALUType) *NALU {
	for i := 0; i < len(p.NALUs); i++ {
		if p.NALUs[i].Type264() == t {
			return &p.NALUs[i]
		}
	}
	return nil
}

// Returns the number of bytes of NALU data.
// If the NALUs have annex-b prefixes, then these are included in the size.
func (p *VideoPacket) PayloadBytes() int {
	size := 0
	for _, n := range p.NALUs {
		size += len(n.Payload)
	}
	return size
}

func (p *VideoPacket) Summary() string {
	parts := []string{}
	for _, n := range p.NALUs {
		t := n.Type(p.Codec)
		parts = append(parts, fmt.Sprintf("%v (%v bytes)", t, len(n.Payload)))
	}
	return fmt.Sprintf("%v packets: ", len(p.NALUs)) + strings.Join(parts, ", ")
}

// Encode all NALUs in the packet into AnnexB format (i.e. with 00,00,01 prefix bytes, and emulation prevention bytes)
func (p *VideoPacket) EncodeToAnnexBPacket() []byte {
	if len(p.NALUs) == 1 && p.NALUs[0].IsAnnexBWithStartCode() {
		return p.NALUs[0].Payload
	}

	// estimate how much space we'll need
	outLen := 0
	for _, n := range p.NALUs {
		outLen += 4 // worst start code size
		if !n.PayloadIsAnnexB {
			outLen += AnnexBWorstSize(0, len(n.Payload))
		} else {
			outLen += len(n.Payload)
		}
	}
	// build up a contiguous buffer
	out := make([]byte, outLen)
	used := 0
	for _, n := range p.NALUs {
		if !n.IsAnnexBWithStartCode() {
			flags := AnnexBEncodeFlagNone
			if EnableEmulationPreventBytesEscaping && !n.PayloadIsAnnexB {
				flags |= AnnexBEncodeFlagAddEmulationPreventionBytes
			}
			encSize, encOK := EncodeAnnexBInto(n.Payload[n.StartCodeLen():], 3, flags, out[used:])
			if !encOK {
				panic("Ran out of space packing NALUs into Annex-B")
			}
			used += encSize
		} else {
			copy(out[used:], n.Payload)
			used += len(n.Payload)
		}
	}
	return out[:used]
}

// Clone a packet of NALUs and return the cloned packet
// NOTE: gortsplib re-uses buffers, which is why we copy the payloads.
// NOTE2: I think that after upgrading gortsplib in Jan 2024, it no longer re-uses buffers,
// so I should revisit the requirement of our deep clone here.
func ClonePacket(nalusIn [][]byte, codec Codec, pts time.Duration, recvTime time.Time, wallPTS time.Time, isPayloadAnnexBEncoded bool) *VideoPacket {
	nalus := []NALU{}
	for _, buf := range nalusIn {
		// While we're doing a memcpy, we might as well append the start codes.
		// This saves us one additional memcpy before we send the NALUs out for
		// decoding to RGBA, saving to mp4, or sending to the browser.
		// UPDATE 1: Now that we're actually doing the Annex-B encoding, and it has
		// a non-zero cost, I'm opting to rather delay the Annex-B encoding until
		// necessary. This is largely irrelevant for low resolution streams, but
		// it does have a small but non-zero performance impact on high res streams.
		// About 1% of CPU time on an Rpi5, with 4 cameras.
		// UPDATE 2: Now that I've discovered that some cameras (eg Hikvision)
		// send packets that are Annex-B encoded, but without start codes, I figure
		// we might as well add the start codes here, if the packets are already
		// Annex-B encoded. With the start codes in place, the packets are ready
		// to send to ffmpeg and our archive, without any further conversion or
		// memory copies.
		n := WrapRawNALU(buf)
		n.PayloadIsAnnexB = isPayloadAnnexBEncoded
		if isPayloadAnnexBEncoded && n.StartCodeLen() == 0 {
			// In this case, add the start codes, because it's virtually free to do
			// so, since we have to clone the incoming buffer anyway.
			n.Payload = append(NALUStartCode(3), n.Payload...)
		} else {
			n = n.DeepClone()
		}
		nalus = append(nalus, n)
	}
	return &VideoPacket{
		Codec:   codec,
		NALUs:   nalus,
		PTS:     pts,
		WallPTS: wallPTS,
	}
}
