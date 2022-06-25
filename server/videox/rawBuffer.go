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

	"github.com/bmharper/cyclops/server/log"
	"github.com/bmharper/cyclops/server/util"
)

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
	SPS     []byte
	PPS     []byte
}

// Clone a raw NALU, but add prefix bytes to the clone
func CloneNALUWithPrefix(raw []byte) NALU {
	return NALU{
		PrefixLen: 3,
		Payload:   append([]byte{0x00, 0x00, 0x01}, raw...),
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
		Payload:   util.CopySlice(n.Payload),
	}
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

// Extract saved buffer into an MPEGTS stream
func (r *RawBuffer) SaveToMPEGTS(log log.Log, output io.Writer) error {
	encoder, err := NewMPEGTSEncoder(log, output, r.SPS, r.PPS)
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

func (r *RawBuffer) SaveToMP4(filename string) error {
	enc, err := NewVideoEncoder("mp4", filename, 2048, 1536)
	if err != nil {
		return err
	}
	defer enc.Close()

	for _, packet := range r.Packets {
		dts := packet.H264PTS
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
