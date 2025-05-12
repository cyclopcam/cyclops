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
	"github.com/bluenviron/mediacommon/pkg/codecs/h265"
	"github.com/bmharper/cimg/v2"
	"github.com/cyclopcam/cyclops/pkg/gen"
	"github.com/cyclopcam/logs"
)

// PacketBuffer is a list of packets, with some helper functions
type PacketBuffer struct {
	Packets []*VideoPacket
}

func (r *PacketBuffer) Codec() Codec {
	if len(r.Packets) == 0 {
		return CodecUnknown
	}
	return r.Packets[0].Codec
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
	if r.Codec() != CodecH264 {
		return fmt.Errorf("Cannot save to MPEG-TS: codec is not H264")
	}
	sps := r.FirstNALUOfType264(h264.NALUTypeSPS)
	pps := r.FirstNALUOfType264(h264.NALUTypePPS)
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
		log.Infof("MPGTS encode packet PTS:%v", packet.PTS)
		err := encoder.Encode(packet.NALUs, packet.PTS)
		if err != nil {
			log.Errorf("MPGTS Encode failed: %v", err)
			return err
		}
	}
	return encoder.Close()
}

// Decode SPS and PPS to extract header information
func (r *PacketBuffer) DecodeHeader() (width, height int, err error) {
	if r.Codec() == CodecH264 {
		sps := r.FirstNALUOfType264(h264.NALUTypeSPS)
		if sps == nil {
			return 0, 0, fmt.Errorf("Failed to find SPS NALU")
		}
		return ParseH264SPS(sps.AsRBSP().Payload)
	} else if r.Codec() == CodecH265 {
		sps := r.FirstNALUOfType265(h265.NALUType_SPS_NUT)
		if sps == nil {
			return 0, 0, fmt.Errorf("Failed to find SPS NALU")
		}
		return ParseH265SPS(sps.AsRBSP().Payload)
	}
	return 0, 0, fmt.Errorf("Codec not supported")
}

// Returns the first NALU of the given type, or nil if none found
func (r *PacketBuffer) FirstNALUOfType264(ofType h264.NALUType) *NALU {
	for _, packet := range r.Packets {
		for j := range packet.NALUs {
			if packet.NALUs[j].Type264() == ofType {
				return &packet.NALUs[j]
			}
		}
	}
	return nil
}

// Returns the first NALU of the given type, or nil if none found
func (r *PacketBuffer) FirstNALUOfType265(ofType h265.NALUType) *NALU {
	for _, packet := range r.Packets {
		for j := range packet.NALUs {
			if packet.NALUs[j].Type265() == ofType {
				return &packet.NALUs[j]
			}
		}
	}
	return nil
}

func (r *PacketBuffer) FindFirstPacketOfType(ofType AbstractNALUType) int {
	for i, packet := range r.Packets {
		if packet.HasAbstractType(ofType) {
			return i
		}
	}
	return -1
}

func (r *PacketBuffer) SaveToMP4(filename string) error {
	if r.Codec() != CodecH264 {
		return fmt.Errorf("Cannot save to MP4: codec is not H264")
	}
	width, height, err := r.DecodeHeader()
	if err != nil {
		return err
	}

	// Assume the first IDR packet also has SPS, PPS, and VPS. This has so far been true on my HikVision cameras.
	firstPacket := r.FindFirstPacketOfType(AbstractNALUTypeIDR)
	/*

		firstSPS := r.FirstNALUOfType264(h264.NALUTypeSPS)
		firstPPS := r.FirstNALUOfType264(h264.NALUTypePPS)
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
		baseTime := r.Packets[firstIDR_i].PTS

		enc, err := NewVideoEncoder(r.Codec().ToFFmpeg(), "mp4", filename, width, height, AVPixelFormatYUV420P, AVPixelFormatYUV420P, VideoEncoderTypePackets, 0)
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
	*/

	baseTime := r.Packets[firstPacket].PTS

	enc, err := NewVideoEncoder(r.Codec().ToFFmpeg(), "mp4", filename, width, height, AVPixelFormatYUV420P, AVPixelFormatYUV420P, VideoEncoderTypePackets, 0)
	if err != nil {
		return err
	}
	defer enc.Close()

	for _, packet := range r.Packets[firstPacket:] {
		dts := packet.PTS - baseTime
		//pts := dts + time.Nanosecond*1000
		pts := dts
		//fmt.Printf("%v\n", pts.Milliseconds())
		for _, nalu := range packet.NALUs {
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
		for j := 0; j < len(packet.NALUs); j++ {
			if err := os.WriteFile(fmt.Sprintf("%v/%03d-%03d.%012d.raw", dir, i, j, packet.PTS.Nanoseconds()), packet.NALUs[j].AsRBSP().Payload, 0660); err != nil {
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
	offset := r.Packets[0].PTS
	for _, p := range r.Packets {
		p.PTS -= offset
	}
}

// Decode the center-most keyframe
// This is O(1), assuming no errors or funny business like no keyframes.
func (r *PacketBuffer) ExtractThumbnail() (*cimg.Image, error) {
	decoder, err := NewVideoStreamDecoder(r.Codec())
	if err != nil {
		return nil, err
	}
	defer decoder.Close()

	// walk back from the middle until we find a keyframe
	i := len(r.Packets) / 2
	for ; i >= 0; i-- {
		if r.Packets[i].HasAbstractType(AbstractNALUTypeIDR) {
			break
		}
	}
	// decode forwards until we have an image
	if i == -1 {
		i = 0
	}
	for ; i < len(r.Packets); i++ {
		//fmt.Printf("%v: %v\n", i, r.Packets[i].Summary())
		frame, _ := decoder.Decode(r.Packets[i])
		if frame != nil {
			return frame.Image.ToCImageRGB(), nil
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
			cPacket = &VideoPacket{
				PTS: time.Duration(timeNS) * time.Nanosecond,
			}
		}
		raw, err := os.ReadFile(rawFilename)
		if err != nil {
			return nil, err
		}
		cPacket.NALUs = append(cPacket.NALUs, WrapRawNALU(raw))
	}
	return buf, nil
}
