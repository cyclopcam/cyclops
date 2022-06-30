package videox

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/aler9/gortsplib/pkg/h264"
	"github.com/bmharper/cyclops/server/gen"
	"github.com/bmharper/cyclops/server/log"
)

// #include "h264ParseSPS.h"
import "C"

const NALUPrefixLen = 4

var NALUPrefix = []byte{0x00, 0x00, 0x00, 0x01}

type NALU struct {
	PrefixLen int // If zero, then no prefix. If 3 or 4, then 00 00 01 or 00 00 00 01.
	Payload   []byte
}

// DecodedPacket is what we store in our ring buffer
type DecodedPacket struct {
	H264NALUs    []NALU
	H264PTS      time.Duration
	PTSEqualsDTS bool
}

type RawBuffer struct {
	Packets []*DecodedPacket
	//SPS     []byte
	//PPS     []byte
}

// Clone a raw NALU, but add prefix bytes to the clone
func CloneNALUWithPrefix(raw []byte) NALU {
	return NALU{
		PrefixLen: NALUPrefixLen,
		Payload:   append(NALUPrefix, raw...),
	}
}

// Returns a clone with prefix bytes added
func (n *NALU) CloneWithPrefix() NALU {
	if n.PrefixLen != 0 {
		return n.Clone()
	}
	return NALU{
		PrefixLen: NALUPrefixLen,
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
		H264PTS:      p.H264PTS,
		PTSEqualsDTS: p.PTSEqualsDTS,
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
	raw := sps.RawPayload()
	var cwidth C.int
	var cheight C.int
	C.ParseSPS(unsafe.Pointer(&raw[0]), C.ulong(len(raw)), &cwidth, &cheight)
	width = int(cwidth)
	height = int(cheight)
	return

	/*
		sps := r.FirstNALUOfType(h264.NALUTypeSPS)
		pps := r.FirstNALUOfType(h264.NALUTypePPS)
		if sps == nil || pps == nil {
			return 0, 0, fmt.Errorf("Failed to find SPS and/or PPS")
		}

		decoder, err := NewH264Decoder()
		if err != nil {
			return
		}
		defer decoder.Close()

		if err = decoder.DecodeAndDiscard(*sps); err != nil {
			//return 0, 0, fmt.Errorf("Failed to decode SPS: %w", err)
		}
		if err = decoder.DecodeAndDiscard(*pps); err != nil {
			//return 0, 0, fmt.Errorf("Failed to decode PPS: %w", err)
		}
		if err = decoder.DecodeAndDiscard(r.Packets[2].H264NALUs[0]); err != nil {
			//return 0, 0, fmt.Errorf("Failed to decode PPS: %w", err)
		}

		return decoder.Width(), decoder.Height(), nil
	*/
}

// Returns the first SPS NALU, or nil if none found
func (r *RawBuffer) FirstNALUOfType(ofType h264.NALUType) *NALU {
	for _, packet := range r.Packets {
		for i := range packet.H264NALUs {
			if packet.H264NALUs[i].Type() == ofType {
				return &packet.H264NALUs[i]
			}
		}
	}
	return nil
}

func (r *RawBuffer) SaveToMP4(filename string) error {
	width, height, err := r.DecodeHeader()
	if err != nil {
		return err
	}

	//enc, err := NewVideoEncoder("mp4", filename, 2048, 1536)
	enc, err := NewVideoEncoder("mp4", filename, width, height)
	if err != nil {
		return err
	}
	defer enc.Close()

	baseTime := r.Packets[0].H264PTS

	for _, packet := range r.Packets {
		dts := packet.H264PTS - baseTime
		pts := dts + time.Nanosecond*1000
		for _, nalu := range packet.H264NALUs {
			err := enc.WritePacket(dts, pts, nalu)
			if err != nil {
				return err
			}
			dts += 10000
			pts += 10000
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
