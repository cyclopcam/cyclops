package videox

import (
	"errors"
	"fmt"
	"image"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/h264"
	"github.com/bmharper/cyclops/pkg/gen"
	"github.com/bmharper/cyclops/pkg/log"
)

// This is the prefix that we add whenever we need to encode into AnnexB
var NALUPrefix = []byte{0x00, 0x00, 0x01}

// A NALU, with optional annex-b prefix bytes
// NOTE: We do not add the 0x03 "Emulation Prevention" byte, so this is going to bite us every now and then. We SHOULD DO IT.
type NALU struct {
	// If zero, then no prefix.
	// If 3 or 4, then the first N bytes of Payload are 00 00 01 or 00 00 00 01 respectively.
	// No values beside 0,3,4 are valid for PrefixLen.
	PrefixLen int
	Payload   []byte
}

// DecodedPacket is what we store in our ring buffer
// This thing probably wants a better name...
// Maybe VideoPacket?
type DecodedPacket struct {
	RecvID       int64     // Arbitrary monotonically increasing ID. Used to detect dropped packets, or other issues like that.
	RecvTime     time.Time // Wall time when the packet was received. This is obviously subject to network jitter etc, so not a substitute for PTS
	H264NALUs    []NALU
	H264PTS      time.Duration
	PTSEqualsDTS bool
	IsBacklog    bool // a bit of a hack to inject this state here. maybe an integer counter would suffice? (eg nBacklongPackets)
}

type RawBuffer struct {
	Packets []*DecodedPacket
}

// Clone a raw NALU, but add prefix bytes to the clone
func CloneNALUWithPrefix(raw []byte) NALU {
	return NALU{
		PrefixLen: len(NALUPrefix),
		Payload:   append(NALUPrefix, raw...),
	}
}

// Returns a clone with prefix bytes added
func (n *NALU) CloneWithPrefix() NALU {
	if n.PrefixLen != 0 {
		return n.Clone()
	}
	return NALU{
		PrefixLen: len(NALUPrefix),
		Payload:   append(NALUPrefix, n.Payload...),
	}
}

// Wrap a raw buffer in a NALU object. Do not clone memory, or add prefix bytes.
func WrapRawNALU(raw []byte) NALU {
	return NALU{
		PrefixLen: 0,
		Payload:   raw,
	}
}

// Returns the payload with the prefix bytes stripped out
func (n *NALU) RawPayload() []byte {
	return n.Payload[n.PrefixLen:]
}

// Return a deep clone of a NALU
func (n *NALU) Clone() NALU {
	return NALU{
		PrefixLen: n.PrefixLen,
		Payload:   gen.CopySlice(n.Payload),
	}
}

// Return the NALU type
func (n *NALU) Type() h264.NALUType {
	i := n.PrefixLen
	if i >= len(n.Payload) {
		return h264.NALUType(0)
	}
	return h264.NALUType(n.Payload[i] & 31)
}

// Deep clone of packet buffer
func (p *DecodedPacket) Clone() *DecodedPacket {
	c := &DecodedPacket{
		RecvID:       p.RecvID,
		RecvTime:     p.RecvTime,
		H264PTS:      p.H264PTS,
		PTSEqualsDTS: p.PTSEqualsDTS,
		IsBacklog:    p.IsBacklog,
	}
	c.H264NALUs = make([]NALU, len(p.H264NALUs))
	for i, n := range p.H264NALUs {
		c.H264NALUs[i] = n.Clone()
	}
	return c
}

// Return true if this packet has a NALU of type t inside
func (p *DecodedPacket) HasType(t h264.NALUType) bool {
	for _, n := range p.H264NALUs {
		if n.Type() == t {
			return true
		}
	}
	return false
}

// Returns true if this packet has a keyframe
func (p *DecodedPacket) HasIDR() bool {
	return p.HasType(h264.NALUTypeIDR)
}

// Return true if this packet has one NALU which is an intermediate frame
func (p *DecodedPacket) IsIFrame() bool {
	return len(p.H264NALUs) == 1 && p.H264NALUs[0].Type() == h264.NALUTypeNonIDR
}

// Returns the first NALU of the given type, or nil if none exists
func (p *DecodedPacket) FirstNALUOfType(t h264.NALUType) *NALU {
	for i := 0; i < len(p.H264NALUs); i++ {
		if p.H264NALUs[i].Type() == t {
			return &p.H264NALUs[i]
		}
	}
	return nil
}

// Returns the number of bytes of NALU data.
// If the NALUs have annex-b prefixes, then this number of included in the size.
func (p *DecodedPacket) PayloadBytes() int {
	size := 0
	for _, n := range p.H264NALUs {
		size += len(n.Payload)
	}
	return size
}

func (p *DecodedPacket) Summary() string {
	parts := []string{}
	for _, n := range p.H264NALUs {
		t := n.Type()
		parts = append(parts, fmt.Sprintf("%v (%v bytes)", t, len(n.Payload)))
	}
	return fmt.Sprintf("%v packets: ", len(p.H264NALUs)) + strings.Join(parts, ", ")
}

// Encode all NALUs in the packet into AnnexB format (i.e. with 00,00,01 prefix bytes)
func (p *DecodedPacket) EncodeToAnnexBPacket() []byte {
	if len(p.H264NALUs) == 1 && p.H264NALUs[0].PrefixLen != 0 {
		return p.H264NALUs[0].Payload
	}

	// measure how much space we'll need
	outLen := 0
	for _, n := range p.H264NALUs {
		outLen += len(n.Payload)
		if n.PrefixLen == 0 {
			outLen += len(NALUPrefix)
		}
	}
	// build up a contiguous buffer
	out := make([]byte, 0, outLen)
	for _, n := range p.H264NALUs {
		if n.PrefixLen == 0 {
			out = append(out, NALUPrefix...)
		}
		out = append(out, n.Payload...)
	}
	return out
}

// Clone a packet of NALUs and return the cloned packet
func ClonePacket(ctx *gortsplib.ClientOnPacketRTPCtx, recvTime time.Time) *DecodedPacket {
	nalus := []NALU{}
	for _, buf := range ctx.H264NALUs {
		// While we're doing a memcpy, we might as well append the prefix bytes.
		// This saves us one additional memcpy before we send the NALUs out for
		// decoding to RGBA, saving to mp4, or sending to the browser.
		nalus = append(nalus, CloneNALUWithPrefix(buf))
	}
	return &DecodedPacket{
		RecvTime:     recvTime,
		H264NALUs:    nalus,
		H264PTS:      ctx.H264PTS,
		PTSEqualsDTS: ctx.PTSEqualsDTS,
	}
}

// Wrap a packet of NALUs into our own data structure.
// WARNING: gortsplib re-uses buffers, so the memory buffers inside the NALUs here are only valid until your function returns.
func WrapPacket(ctx *gortsplib.ClientOnPacketRTPCtx, recvTime time.Time) *DecodedPacket {
	nalus := make([]NALU, 0, len(ctx.H264NALUs))
	for _, buf := range ctx.H264NALUs {
		nalus = append(nalus, WrapRawNALU(buf))
	}
	return &DecodedPacket{
		RecvTime:     recvTime,
		H264NALUs:    nalus,
		H264PTS:      ctx.H264PTS,
		PTSEqualsDTS: ctx.PTSEqualsDTS,
	}
}

// Returns true if the packet has an IDR (with my Hikvisions this always implies IPS + PPS + IDR)
func PacketHasIDR(ctx *gortsplib.ClientOnPacketRTPCtx) bool {
	for _, p := range ctx.H264NALUs {
		t := h264.NALUType(p[0] & 31)
		if t == h264.NALUTypeIDR {
			return true
		}
	}
	return false
}

// Extract saved buffer into an MPEGTS stream
func (r *RawBuffer) SaveToMPEGTS(log log.Log, output io.Writer) error {
	sps := r.FirstNALUOfType(h264.NALUTypeSPS)
	pps := r.FirstNALUOfType(h264.NALUTypePPS)
	if sps == nil || pps == nil {
		return fmt.Errorf("Stream has no SPS or PPS")
	}
	encoder, err := NewMPEGTSEncoder(log, output, sps.RawPayload(), pps.RawPayload())
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
func (r *RawBuffer) DecodeHeader() (width, height int, err error) {
	sps := r.FirstNALUOfType(h264.NALUTypeSPS)
	if sps == nil {
		return 0, 0, fmt.Errorf("Failed to find SPS NALU")
	}
	return ParseSPS(sps.RawPayload())
}

// Returns the first NALU of the given type, or nil if none found
func (r *RawBuffer) FirstNALUOfType(ofType h264.NALUType) *NALU {
	i, j := r.IndexOfFirstNALUOfType(ofType)
	if i == -1 {
		return nil
	}
	return &r.Packets[i].H264NALUs[j]
}

func (r *RawBuffer) IndexOfFirstNALUOfType(ofType h264.NALUType) (packetIdx int, indexInPacket int) {
	for i, packet := range r.Packets {
		for j := range packet.H264NALUs {
			if packet.H264NALUs[j].Type() == ofType {
				return i, j
			}
		}
	}
	return -1, -1
}

func (r *RawBuffer) SaveToMP4(filename string) error {
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
func (r *RawBuffer) DumpBin(dir string) error {
	files, _ := filepath.Glob(dir + "/*.raw")
	for _, file := range files {
		os.Remove(file)
	}
	for i, packet := range r.Packets {
		for j := 0; j < len(packet.H264NALUs); j++ {
			if err := os.WriteFile(fmt.Sprintf("%v/%03d-%03d.%012d.raw", dir, i, j, packet.H264PTS.Nanoseconds()), packet.H264NALUs[j].RawPayload(), 0660); err != nil {
				return err
			}
		}
	}
	return nil
}

// Adjust all PTS values so that the first frame start at time 0
func (r *RawBuffer) ResetPTS() {
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
func (r *RawBuffer) ExtractThumbnail() (image.Image, error) {
	decoder, err := NewH264Decoder()
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
			return cloneImage(img), nil
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
func LoadBinDir(dir string) (*RawBuffer, error) {
	files, err := filepath.Glob(dir + "/*.raw")
	if err != nil {
		return nil, err
	}
	sort.Strings(files)

	buf := &RawBuffer{
		Packets: []*DecodedPacket{},
	}
	cPacketNumber := -1
	var cPacket *DecodedPacket

	for _, rawFilename := range files {
		packetNumber, _, timeNS := ParseBinFilename(rawFilename)
		if packetNumber != cPacketNumber {
			if cPacket != nil {
				buf.Packets = append(buf.Packets, cPacket)
			}
			// NOTE: We don't populate RecvTime
			cPacket = &DecodedPacket{
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
