package videox

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bluenviron/mediacommon/pkg/codecs/h264"
	"github.com/bmharper/cimg/v2"
	"github.com/cyclopcam/cyclops/pkg/gen"
	"github.com/cyclopcam/logs"
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

// PayloadState tells us the state of the payload, such as whether it has been escaped for Annex-B
type PayloadFormat int8

const (
	PayloadRawBytes PayloadFormat = iota // Not escaped (RBSP)
	PayloadAnnexB                        // Annex-B escaped (SODB)
)

// Note that h264/h265 talks about the following two types of NALUs:
// * RBSP Raw Byte Sequence Payload (No start code, no emulation prevention bytes)
// * SODB String of Data Bits (Annex-B encoding. Has start code and emulation prevention bytes)
// But see the long comment above ($ANNEXB-CONFUSION).

// Codec NALU
type NALU struct {
	PayloadIsAnnexB  bool
	PayloadNoEscapes bool // True if PayloadIsAnnexB BUT we know that we have no "emulation prevention bytes", so we can avoid decoding them.
	Payload          []byte
}

// VideoPacket is what we store in our ring buffer
type VideoPacket struct {
	RawRecvID   int64     // Arbitrary monotonically increasing ID of raw received. Used to detect dropped packets, or other issues like that.
	ValidRecvID int64     // Arbitrary monotonically increasing ID of useful decoded packets. Used to detect dropped packets, or other issues like that.
	RecvTime    time.Time // Wall time when the packet was received. This is obviously subject to network jitter etc, so not a substitute for PTS
	H264NALUs   []NALU
	H264PTS     time.Duration
	WallPTS     time.Time // Reference wall time combined with the received PTS. We consider this the ground truth/reality of when the packet was recorded.
	IsBacklog   bool      // a bit of a hack to inject this state here. maybe an integer counter would suffice? (eg nBacklogPackets)
}

// A list of packets, with some helper functions
type PacketBuffer struct {
	Packets []*VideoPacket
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
			return NALU{
				Payload:          n.Payload,
				PayloadIsAnnexB:  true,
				PayloadNoEscapes: n.PayloadNoEscapes,
			}
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

// Return the NALU type
func (n *NALU) Type() h264.NALUType {
	i := n.StartCodeLen()
	if i >= len(n.Payload) {
		return h264.NALUType(0)
	}
	return h264.NALUType(n.Payload[i] & 31)
}

// Deep clone of packet buffer
func (p *VideoPacket) Clone() *VideoPacket {
	c := &VideoPacket{
		RawRecvID:   p.RawRecvID,
		ValidRecvID: p.ValidRecvID,
		RecvTime:    p.RecvTime,
		H264PTS:     p.H264PTS,
		WallPTS:     p.WallPTS,
		IsBacklog:   p.IsBacklog,
	}
	c.H264NALUs = make([]NALU, len(p.H264NALUs))
	for i, n := range p.H264NALUs {
		c.H264NALUs[i] = n.DeepClone()
	}
	return c
}

// Return true if this packet has a NALU of type t inside
func (p *VideoPacket) HasType(t h264.NALUType) bool {
	for _, n := range p.H264NALUs {
		if n.Type() == t {
			return true
		}
	}
	return false
}

// Returns true if this packet has a keyframe
func (p *VideoPacket) HasIDR() bool {
	return p.HasType(h264.NALUTypeIDR)
}

// Return true if this packet has one NALU which is an intermediate frame
func (p *VideoPacket) IsIFrame() bool {
	return len(p.H264NALUs) == 1 && p.H264NALUs[0].Type() == h264.NALUTypeNonIDR
}

// Returns the first NALU of the given type, or nil if none exists
func (p *VideoPacket) FirstNALUOfType(t h264.NALUType) *NALU {
	for i := 0; i < len(p.H264NALUs); i++ {
		if p.H264NALUs[i].Type() == t {
			return &p.H264NALUs[i]
		}
	}
	return nil
}

// Returns the number of bytes of NALU data.
// If the NALUs have annex-b prefixes, then these are included in the size.
func (p *VideoPacket) PayloadBytes() int {
	size := 0
	for _, n := range p.H264NALUs {
		size += len(n.Payload)
	}
	return size
}

func (p *VideoPacket) Summary() string {
	parts := []string{}
	for _, n := range p.H264NALUs {
		t := n.Type()
		parts = append(parts, fmt.Sprintf("%v (%v bytes)", t, len(n.Payload)))
	}
	return fmt.Sprintf("%v packets: ", len(p.H264NALUs)) + strings.Join(parts, ", ")
}

// Encode all NALUs in the packet into AnnexB format (i.e. with 00,00,01 prefix bytes)
func (p *VideoPacket) EncodeToAnnexBPacket() []byte {
	if len(p.H264NALUs) == 1 && p.H264NALUs[0].IsAnnexBWithStartCode() {
		return p.H264NALUs[0].Payload
	}

	// estimate how much space we'll need
	outLen := 0
	for _, n := range p.H264NALUs {
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
	for _, n := range p.H264NALUs {
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
func ClonePacket(nalusIn [][]byte, pts time.Duration, recvTime time.Time, wallPTS time.Time, isPayloadAnnexBEncoded bool) *VideoPacket {
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
		RecvTime:  recvTime,
		H264NALUs: nalus,
		H264PTS:   pts,
		WallPTS:   wallPTS,
	}
}

// Returns true if we have at least one keyframe in the buffer
func (r *PacketBuffer) HasIDR() bool {
	return r.FindFirstIDR() != -1
}

// Returns the index of the first keyframe in the buffer, or -1 if none found
func (r *PacketBuffer) FindFirstIDR() int {
	for i, p := range r.Packets {
		if p.HasIDR() {
			return i
		}
	}
	return -1
}

// Find the packet with the WallPTS closest to the given time
func (r *PacketBuffer) FindClosestPacketWallPTS(wallPTS time.Time, keyframeOnly bool) int {
	bestMatchDeltaT := time.Duration(1<<63 - 1)
	bestMatchIdx := -1
	for i, p := range r.Packets {
		if keyframeOnly && !p.HasIDR() {
			continue
		}
		deltaT := gen.Abs(wallPTS.Sub(p.WallPTS))
		if deltaT < bestMatchDeltaT {
			bestMatchDeltaT = deltaT
			bestMatchIdx = i
		}
	}
	return bestMatchIdx
}

// Extract saved buffer into an MPEGTS stream
func (r *PacketBuffer) SaveToMPEGTS(log logs.Log, output io.Writer) error {
	sps := r.FirstNALUOfType(h264.NALUTypeSPS)
	pps := r.FirstNALUOfType(h264.NALUTypePPS)
	if sps == nil || pps == nil {
		return fmt.Errorf("Stream has no SPS or PPS")
	}
	encoder, err := NewMPEGTSEncoder(log, output, sps.AsRBSP().Payload, pps.AsRBSP().Payload)
	if err != nil {
		return fmt.Errorf("Failed to start MPEGTS encoder: %w", err)
	}
	defer encoder.Close()

	// We don't actually need to drain the buffer - we could make
	// ringbuffer.Peek a public function, and use that to suck data
	// out of the buffer without consuming it. It doesn't really make
	// sense to drain the buffer... it's a constant memory resource...
	// no real advantage is throwing it away.
	// In addition, we could allow simultaneous acess to the ring buffer,
	// so we don't actually need to lock the whole thing. PROVIDED we
	// this draining thread is faster than the consuming thread. We might
	// need to ensure that we're some distance ahead of the writer, to
	// ensure that incoming packets don't overwrite the old frames that
	// we haven't yet written out.
	for _, packet := range r.Packets {
		// encode H264 NALUs into MPEG-TS
		log.Infof("MPGTS encode packet PTS:%v", packet.H264PTS)
		err := encoder.Encode(packet.H264NALUs, packet.H264PTS)
		if err != nil {
			log.Errorf("MPGTS Encode failed: %v", err)
			return err
		}
	}
	return encoder.Close()
}

// Decode SPS and PPS to extract header information
func (r *PacketBuffer) DecodeHeader() (width, height int, err error) {
	sps := r.FirstNALUOfType(h264.NALUTypeSPS)
	if sps == nil {
		return 0, 0, fmt.Errorf("Failed to find SPS NALU")
	}
	return ParseH264SPS(sps.AsRBSP().Payload)
}

// Returns the first NALU of the given type, or nil if none found
func (r *PacketBuffer) FirstNALUOfType(ofType h264.NALUType) *NALU {
	i, j := r.IndexOfFirstNALUOfType(ofType)
	if i == -1 {
		return nil
	}
	return &r.Packets[i].H264NALUs[j]
}

func (r *PacketBuffer) IndexOfFirstNALUOfType(ofType h264.NALUType) (packetIdx int, indexInPacket int) {
	for i, packet := range r.Packets {
		for j := range packet.H264NALUs {
			if packet.H264NALUs[j].Type() == ofType {
				return i, j
			}
		}
	}
	return -1, -1
}

func (r *PacketBuffer) SaveToMP4(filename string) error {
	width, height, err := r.DecodeHeader()
	if err != nil {
		return err
	}

	firstSPS := r.FirstNALUOfType(h264.NALUTypeSPS)
	firstPPS := r.FirstNALUOfType(h264.NALUTypePPS)
	firstIDR_i, _ := r.IndexOfFirstNALUOfType(h264.NALUTypeIDR)
	if firstSPS == nil {
		return errors.New("No SPS found")
	}
	if firstPPS == nil {
		return errors.New("No PPS found")
	}
	if firstIDR_i == -1 {
		return errors.New("No IDR found")
	}
	baseTime := r.Packets[firstIDR_i].H264PTS

	enc, err := NewVideoEncoder("mp4", filename, width, height)
	if err != nil {
		return err
	}
	defer enc.Close()

	err = enc.WriteNALU(0, 0, *firstSPS)
	if err != nil {
		return err
	}
	err = enc.WriteNALU(0, 0, *firstPPS)
	if err != nil {
		return err
	}

	for _, packet := range r.Packets[firstIDR_i:] {
		dts := packet.H264PTS - baseTime
		//pts := dts + time.Nanosecond*1000
		pts := dts
		//fmt.Printf("%v\n", pts.Milliseconds())
		for _, nalu := range packet.H264NALUs {
			err := enc.WriteNALU(dts, pts, nalu)
			if err != nil {
				return err
			}
			//dts += 10000
			//pts += 10000
		}
	}

	if err = enc.WriteTrailer(); err != nil {
		return err
	}

	enc.Close()
	return nil
}

// Dump each NALU to a .raw file
func (r *PacketBuffer) DumpBin(dir string) error {
	files, _ := filepath.Glob(dir + "/*.raw")
	for _, file := range files {
		os.Remove(file)
	}
	for i, packet := range r.Packets {
		for j := 0; j < len(packet.H264NALUs); j++ {
			if err := os.WriteFile(fmt.Sprintf("%v/%03d-%03d.%012d.raw", dir, i, j, packet.H264PTS.Nanoseconds()), packet.H264NALUs[j].AsRBSP().Payload, 0660); err != nil {
				return err
			}
		}
	}
	return nil
}

// Adjust all PTS values so that the first frame starts at time 0
func (r *PacketBuffer) ResetPTS() {
	if len(r.Packets) == 0 {
		return
	}
	offset := r.Packets[0].H264PTS
	for _, p := range r.Packets {
		p.H264PTS -= offset
	}
}

// Decode the center-most keyframe
// This is O(1), assuming no errors or funny business like no keyframes.
func (r *PacketBuffer) ExtractThumbnail() (*cimg.Image, error) {
	decoder, err := NewVideoStreamDecoder("h264")
	if err != nil {
		return nil, err
	}
	defer decoder.Close()

	// walk back from the middle until we find a keyframe
	i := len(r.Packets) / 2
	for ; i >= 0; i-- {
		if r.Packets[i].HasType(h264.NALUTypeIDR) {
			break
		}
	}
	// decode forwards until we have an image
	if i == -1 {
		i = 0
	}
	for ; i < len(r.Packets); i++ {
		//fmt.Printf("%v: %v\n", i, r.Packets[i].Summary())
		img, _ := decoder.Decode(r.Packets[i])
		if img != nil {
			return img.ToCImageRGB(), nil
		}
	}
	return nil, errors.New("No thumbnail available")
}

// This is just used for debugging and testing
func ParseBinFilename(filename string) (packetNumber int, naluNumber int, timeNS int64) {
	// filename example:
	// 026-002.002599955555.raw
	major := strings.Split(filename, ".")
	a, b, _ := strings.Cut(major[0], "-")
	packetNumber, _ = strconv.Atoi(a)
	naluNumber, _ = strconv.Atoi(b)
	timeNS, _ = strconv.ParseInt(major[1], 10, 64)
	return
}

// Opposite of RawBuffer.DumpBin
// NOTE: We don't attempt to inject SPS and PPS into RawBuffer, but would be trivial for H264.. just look at first byte of payload... (67 and 68 for SPS and PPS)
func LoadBinDir(dir string) (*PacketBuffer, error) {
	files, err := filepath.Glob(dir + "/*.raw")
	if err != nil {
		return nil, err
	}
	sort.Strings(files)

	buf := &PacketBuffer{
		Packets: []*VideoPacket{},
	}
	cPacketNumber := -1
	var cPacket *VideoPacket

	for _, rawFilename := range files {
		packetNumber, _, timeNS := ParseBinFilename(rawFilename)
		if packetNumber != cPacketNumber {
			if cPacket != nil {
				buf.Packets = append(buf.Packets, cPacket)
			}
			// NOTE: We don't populate RecvTime
			cPacket = &VideoPacket{
				H264PTS: time.Duration(timeNS) * time.Nanosecond,
			}
		}
		raw, err := os.ReadFile(rawFilename)
		if err != nil {
			return nil, err
		}
		cPacket.H264NALUs = append(cPacket.H264NALUs, WrapRawNALU(raw))
	}
	return buf, nil
}
