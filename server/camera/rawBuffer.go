package camera

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/bmharper/cyclops/server/log"
	"github.com/bmharper/cyclops/server/videox"
)

// DecodedPacket is what we store in our ring buffer
type DecodedPacket struct {
	H264NALUs    [][]byte
	H264PTS      time.Duration
	PTSEqualsDTS bool
}

type RawBuffer struct {
	Packets []*DecodedPacket
	SPS     []byte
	PPS     []byte
}

// Deep clone of packet buffer
func (p *DecodedPacket) Clone() *DecodedPacket {
	c := &DecodedPacket{
		H264PTS:      p.H264PTS,
		PTSEqualsDTS: p.PTSEqualsDTS,
	}
	c.H264NALUs = make([][]byte, len(p.H264NALUs))
	for i, nalus := range p.H264NALUs {
		nalu := make([]byte, len(nalus))
		copy(nalu, nalus)
		c.H264NALUs[i] = nalu
	}
	return c
}

// Extract saved buffer into an MPEGTS stream
func (r *RawBuffer) SaveToMPEGTS(log log.Log, output io.Writer) error {
	encoder, err := videox.NewMPEGTSEncoder(log, output, r.SPS, r.PPS)
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

// Dump each NALU to a .raw file
func (r *RawBuffer) DumpBin(dir string) error {
	files, _ := filepath.Glob(dir + "/*.raw")
	for _, file := range files {
		os.Remove(file)
	}
	for i, packet := range r.Packets {
		for j := 0; j < len(packet.H264NALUs); j++ {
			if err := os.WriteFile(fmt.Sprintf("%v/%03d-%03d.%012d.raw", dir, i, j, packet.H264PTS.Nanoseconds()), packet.H264NALUs[j], 0660); err != nil {
				return err
			}
		}
	}
	return nil
}
