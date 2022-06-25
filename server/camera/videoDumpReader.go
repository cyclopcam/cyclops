package camera

import (
	"sync"

	"github.com/aler9/gortsplib"
	"github.com/bmharper/cyclops/server/log"
	"github.com/bmharper/cyclops/server/util"
	"github.com/bmharper/cyclops/server/videox"
	"github.com/bmharper/ringbuffer"
)

type ExtractMethod int

const (
	ExtractMethodClone ExtractMethod = iota // Make a copy of the buffer, leaving the camera's buffer intact
	ExtractMethodSteal                      // Steal the buffer, leaving the camera's buffer empty
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

func (r *VideoDumpReader) ExtractRawBuffer(method ExtractMethod) *videox.RawBuffer {
	r.BufferLock.Lock()
	defer r.BufferLock.Unlock()

	bufLen := r.Buffer.Len()
	out := &videox.RawBuffer{
		// little race condition here if Track.SPS and Track.PPS don't agree
		SPS:     util.CopySlice(r.Track.SPS()),
		PPS:     util.CopySlice(r.Track.PPS()),
		Packets: make([]*videox.DecodedPacket, bufLen),
	}

	switch method {
	case ExtractMethodClone:
		// We might be holding the lock for too long here. 100 MB copy on RPi4 is 25ms (4GB/s memory bandwidth)
		// It would be possible to incrementally lock and unlock r.BufferLock in order to reduce the duration of our lock.
		for i := 0; i < bufLen; i++ {
			_, packet, _ := r.Buffer.Peek(i)
			out.Packets[i] = packet.Clone()
		}
	case ExtractMethodSteal:
		for i := 0; i < bufLen; i++ {
			_, packet, _ := r.Buffer.Next()
			out.Packets[i] = packet
		}
		if r.Buffer.Len() != 0 {
			panic("Buffer should be empty")
		}
	}
	return out
}
