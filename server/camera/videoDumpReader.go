package camera

import (
	"fmt"
	"sync"
	"time"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/h264"
	"github.com/bmharper/ringbuffer"
	"github.com/cyclopcam/cyclops/pkg/log"
	"github.com/cyclopcam/cyclops/pkg/videox"
)

type ExtractMethod int

const (
	ExtractMethodClone ExtractMethod = iota // Make a copy of the buffer, leaving the camera's buffer intact
	ExtractMethodDrain                      // Drain the buffer, leaving the camera's buffer empty
)

/*
SYNC-MAX-TRAIN-RECORD-TIME

On my hikvision cameras at 320 x 240, an IDR is about 20KB, and an ad-hoc sampling of non-IDR frames
puts them at 100 bytes per frame (a high estimate)..
So at 1 IDR/second, and 10 FPS, that's 20*1000 + 9 * 100 = 21000 bytes/second.
Let's be conservative and double that to 40 KB / second.
To record 60 seconds, we need 2400 KB.
This is just so small, compared to the higher res streams, that we shouldn't
think about it too much. If we want to re-use the low res dumper stream for the
recording buffer, then this is fine, because 60 seconds seems like an excessive
amount of recording time. And even if we wanted to raise it, it wouldn't cost
much. I thought about dynamically raising the weighted ring buffer size while
recording, but it doesn't seem worth the extra complexity. Even a two minute
buffer would be only 4800 KB, and that would be an insanely long recording.
*/

// VideoDumpReader is a ring buffer that accumulates the stream in a format that can be turned into a video file.
// The video is not decoded.
type VideoDumpReader struct {
	Log     log.Log
	TrackID int
	Track   *gortsplib.TrackH264
	HaveIDR bool

	BufferLock sync.Mutex // Guards all access to Buffer
	Buffer     ringbuffer.WeightedRingT[videox.DecodedPacket]

	incoming StreamSinkChan
}

func NewVideoDumpReader(maxRingBufferBytes int) *VideoDumpReader {
	return &VideoDumpReader{
		Buffer:   ringbuffer.NewWeightedRingT[videox.DecodedPacket](maxRingBufferBytes),
		incoming: make(StreamSinkChan, StreamSinkChanDefaultBufferSize),
	}
}

func (r *VideoDumpReader) OnConnect(stream *Stream) (StreamSinkChan, error) {
	r.Log = stream.Log
	r.TrackID = stream.H264TrackID
	r.Track = stream.H264Track
	r.HaveIDR = false
	r.initializeBuffer()
	return r.incoming, nil
}

func (r *VideoDumpReader) initializeBuffer() {
	r.Buffer = ringbuffer.NewWeightedRingT[videox.DecodedPacket](r.Buffer.MaxWeight)
}

func (r *VideoDumpReader) Close() {
	r.Log.Infof("VideoDumpReader closed")
}

func (r *VideoDumpReader) OnPacketRTP(packet *videox.DecodedPacket) {
	//r.Log.Infof("[Packet %v] VideoDumpReader", 0)
	r.BufferLock.Lock()
	defer r.BufferLock.Unlock()

	r.Buffer.Add(packet.PayloadBytes(), packet)
}

// Extract from <video_end - duration> until <video_end>.
// video_end is the PTS of the most recently received packet.
// duration is a positive number.
func (r *VideoDumpReader) ExtractRawBuffer(method ExtractMethod, duration time.Duration) (*videox.RawBuffer, error) {
	r.BufferLock.Lock()
	defer r.BufferLock.Unlock()

	bufLen := r.Buffer.Len()
	if bufLen == 0 {
		return nil, fmt.Errorf("No video available")
	}

	// Compute the starting packet for extraction
	firstPacket := -1
	{
		_, lastPacket, _ := r.Buffer.Peek(bufLen - 1)
		endPTS := lastPacket.H264PTS

		// Assume that all cameras always send SPS + PPS + IDR in a single packet.
		oldestIDR := -1
		oldestIDRTimeDelta := time.Duration(0)
		satisfied := false
		for i := bufLen - 1; i >= 0; i-- {
			_, packet, _ := r.Buffer.Peek(i)
			timeDelta := endPTS - packet.H264PTS
			//r.Log.Infof("%v < %v ?", timeDelta, duration)
			if packet.HasType(h264.NALUTypeIDR) {
				oldestIDR = i
				oldestIDRTimeDelta = timeDelta
				if timeDelta >= duration {
					satisfied = true
					break
				}
			}
		}
		firstPacket = oldestIDR
		if firstPacket == -1 {
			r.Log.Warnf("Not enough video frames in buffer")
			return nil, fmt.Errorf("Not enough video frames in buffer")
		} else if !satisfied {
			// We failed to find enough frames to satisfy the desired duration.
			// In this case, we issue a warning, but return the best effort.
			r.Log.Warnf("Unable to satisfy request for %.1f seconds. Resulting video has only %.1f seconds", duration.Seconds(), oldestIDRTimeDelta.Seconds())
		}
	}

	out := &videox.RawBuffer{
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
		//
		// Note that this chain of reasoning won't work while making a new recording,
		// so we'll need to remember to disable the AI processing while recording
		// new training data.

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

// Scan back in the ring buffer to find the most recent packet containing an IDR frame
// Assumes that you are holding BufferLock
// Returns the index in the buffer, or -1 if none found
func (r *VideoDumpReader) FindLatestIDRPacketNoLock() int {
	i := r.Buffer.Len() - 1
	for ; i >= 0; i-- {
		_, packet, _ := r.Buffer.Peek(i)
		if packet.HasType(h264.NALUTypeIDR) {
			return i
		}
	}
	return -1
}
