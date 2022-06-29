package camera

import (
	"fmt"
	"sync"
	"time"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/h264"
	"github.com/bmharper/cyclops/server/log"
	"github.com/bmharper/cyclops/server/videox"
	"github.com/bmharper/ringbuffer"
)

type ExtractMethod int

const (
	ExtractMethodClone ExtractMethod = iota // Make a copy of the buffer, leaving the camera's buffer intact
	ExtractMethodDrain                      // Drain the buffer, leaving the camera's buffer empty
)

// VideoDumpReader is a ring buffer that accumulates the stream in a format that can be turned into a video file.
// The video is not decoded.
type VideoDumpReader struct {
	Log     log.Log
	TrackID int
	Track   *gortsplib.TrackH264

	BufferLock sync.Mutex // Guards all access to Buffer
	Buffer     ringbuffer.WeightedRingT[videox.DecodedPacket]
}

func NewVideoDumpReader(maxRingBufferBytes int) *VideoDumpReader {
	return &VideoDumpReader{
		Buffer: ringbuffer.NewWeightedRingT[videox.DecodedPacket](maxRingBufferBytes),
	}
}

func (r *VideoDumpReader) Initialize(log log.Log, trackID int, track *gortsplib.TrackH264) error {
	r.Log = log
	r.TrackID = trackID
	r.Track = track
	r.initializeBuffer()
	return nil
}

func (r *VideoDumpReader) initializeBuffer() {
	r.Buffer = ringbuffer.NewWeightedRingT[videox.DecodedPacket](r.Buffer.MaxWeight)
}

func (r *VideoDumpReader) Close() {
	r.Log.Infof("VideoDumpReader closed")
	//r.Encoder.Close()
}

func (r *VideoDumpReader) OnPacketRTP(ctx *gortsplib.ClientOnPacketRTPCtx) {
	//r.Log.Infof("[Packet %v] VideoDumpReader", 0)
	if ctx.TrackID != r.TrackID {
		return
	}

	if ctx.H264NALUs == nil {
		return
	}

	r.BufferLock.Lock()
	defer r.BufferLock.Unlock()

	nBytes := 0
	nalus := []videox.NALU{}
	for _, buf := range ctx.H264NALUs {
		nBytes += len(buf)
		// gortsplib re-uses buffers, so we need to make a copy here.
		// while we're doing a memcpy, we might as well append the prefix bytes.
		// This saves us one additional memcpy before we send the NALUs our for
		// decoding to RGBA or saving to mp4.
		nalus = append(nalus, videox.CloneNALUWithPrefix(buf))
	}
	r.Buffer.Add(nBytes, &videox.DecodedPacket{
		H264NALUs:    nalus,
		H264PTS:      ctx.H264PTS,
		PTSEqualsDTS: ctx.PTSEqualsDTS,
	})
}

// Extract from <now - duration> until <now>.
// duration is a positive number.
func (r *VideoDumpReader) ExtractRawBuffer(method ExtractMethod, duration time.Duration) (*videox.RawBuffer, error) {
	r.BufferLock.Lock()
	defer r.BufferLock.Unlock()

	bufLen := r.Buffer.Len()
	if bufLen == 0 {
		return nil, fmt.Errorf("No video available")
	}

	// Compute the starting packet for extraction
	firstPacket := 0
	{
		_, lastPacket, _ := r.Buffer.Peek(bufLen - 1)
		presentTime := lastPacket.H264PTS
		// Keep going until all 3 are satisfied, with the extra condition that SPS and PPS must precede IDR.
		// This just happens to work, because cameras will send SPS and PPS before every IDR, to allow a listener
		// to join the stream at any time.
		haveIDR := false
		haveSPS := false
		havePPS := false
		for i := bufLen - 1; i >= 0; i-- {
			_, packet, _ := r.Buffer.Peek(i)
			timeDelta := presentTime - packet.H264PTS
			//r.Log.Infof("%v < %v ?", timeDelta, duration)
			if timeDelta < duration {
				continue
			}
			if !haveIDR && packet.HasType(h264.NALUTypeIDR) {
				haveIDR = true
				//r.Log.Infof("haveIDR")
			}
			if haveIDR && !havePPS && packet.HasType(h264.NALUTypePPS) {
				havePPS = true
				//r.Log.Infof("havePPS")
			}
			if haveIDR && !haveSPS && packet.HasType(h264.NALUTypeSPS) {
				haveSPS = true
				//r.Log.Infof("haveSPS")
			}
			if haveIDR && haveSPS && havePPS {
				firstPacket = i
				break
			}
		}
		//if firstPacket == 0 {
		//	r.Log.Infof("Failed to find appropriate starting point for extraction")
		//} else {
		//	r.Log.Infof("Starting extraction at packet %v (len %v)", firstPacket, bufLen)
		//}
		// In the case where this loop ends without ever assigning firstPacket, we fall back
		// to just emitting the entire buffer, regardless of how useful it is.
	}

	out := &videox.RawBuffer{
		// little race condition here if Track.SPS and Track.PPS don't agree.
		//SPS:     util.CopySlice(r.Track.SPS()),
		//PPS:     util.CopySlice(r.Track.PPS()),
		Packets: make([]*videox.DecodedPacket, bufLen-firstPacket),
	}

	switch method {
	case ExtractMethodClone:
		// We might be holding the lock for too long here. 100 MB copy on RPi4 is 25ms (4GB/s memory bandwidth)
		// It would be possible to incrementally lock and unlock r.BufferLock in order to reduce the duration of our lock.
		for i := firstPacket; i < bufLen; i++ {
			_, packet, _ := r.Buffer.Peek(i)
			out.Packets[i-firstPacket] = packet.Clone()
		}
	case ExtractMethodDrain:
		// Discard earlier history from the ring buffer.
		// In practice this is OK, because it means we've had a detection event, but the fact that
		// we didn't have a detection event prior to this implies that all older footage is
		// uninteresting, so discarding the old history is fine.
		// Using Drain instead of Clone is nice because we don't need to copy the memory.
		for i := 0; i < firstPacket; i++ {
			r.Buffer.Next()
		}
		for i := firstPacket; i < bufLen; i++ {
			_, packet, _ := r.Buffer.Next()
			out.Packets[i-firstPacket] = packet
		}
		if r.Buffer.Len() != 0 {
			panic("Buffer should be empty")
		}
	}
	return out, nil
}
