package camera

import (
	"fmt"
	"sync"
	"time"

	"github.com/bluenviron/mediacommon/pkg/codecs/h264"
	"github.com/bmharper/ringbuffer"
	"github.com/cyclopcam/cyclops/pkg/videox"
	"github.com/cyclopcam/logs"
)

type ExtractMethod int

// A note on Shallow vs Deep clone:
// I think that initially when I was building this, I had forgotten that I was using a garbage collected
// language, and so I made the Clone extraction a deep clone, where I make a copy of the packet contents.
// Subsequently, I realized that this is a waste of effort, we should simply use shallow clones most
// of the time, and let the garbage collector handle the memory sweep.
// So if in doubt, just use a shallow clone. The reason I leave shallow and deep here explicitly, is for
// a future proof, in case we need to be stricter about our memory consumption, and take more careful
// accounting of memory use.
const (
	ExtractMethodShallowClone ExtractMethod = iota // Make a shallow copy of the packets, leaving the camera's buffer intact
	ExtractMethodDeepClone                         // Make a deep copy of the packet contents, leaving the camera's buffer intact
	ExtractMethodDrain                             // Drain the buffer, leaving the camera's buffer empty
)

// If we're sending to a channel, and it is full, what do we do?
type FullChannelPolicy int

const (
	FullChannelPolicyDrop  FullChannelPolicy = iota // If channel is full, drop packets
	FullChannelPolicyStall                          // If channel is full, write the packet anyway, knowing that we'll block
)

// RingBufferListener is a listener that receives video packets from the ring buffer via a channel.
// The name is used only for logging and debugging. The thing that uniquely identifies the listener
// is the channel.
type RingBufferListener struct {
	Name       string
	Chan       chan *videox.VideoPacket
	Policy     FullChannelPolicy
	lastLogMsg time.Time
	nDropped   int
	nStalled   int
}

/*
SYNC-MAX-TRAIN-RECORD-TIME

This is an old thought, from before the time when I had implemented continuous
recording mode.

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

// VideoRingBuffer stores incoming packets in a fixed-size ring buffer, so
// that we always have a bit of video history to use. This is specifically
// useful when recordings are triggered by events such as motion or object
// detection. In such a case, you always want some seconds of prior history,
// from the moments before the event was detected.
//
// If you need to extract some history, and then continue receiving packets
// and guarantee that there is no gap in between those two, then you do this:
// 1. BufferLock.Lock()
// 2. ExtractRawBufferNoLock()
// 3. AddPacketListener()
// 4. BufferLock.Unlock()
//
// The above sequence is what videoRecorder uses when it starts recording.
type VideoRingBuffer struct {
	Log logs.Log

	BufferLock sync.Mutex // Guards all access to Buffer
	Buffer     ringbuffer.WeightedRingT[videox.VideoPacket]

	packetListenerLock sync.Mutex
	packetListeners    []*RingBufferListener

	incoming StreamSinkChan
}

func NewVideoRingBuffer(maxRingBufferBytes int) *VideoRingBuffer {
	return &VideoRingBuffer{
		Buffer:   ringbuffer.NewWeightedRingT[videox.VideoPacket](maxRingBufferBytes),
		incoming: make(StreamSinkChan, StreamSinkChanDefaultBufferSize),
	}
}

func (r *VideoRingBuffer) OnConnect(stream *Stream) (StreamSinkChan, error) {
	r.Log = stream.Log
	r.initializeBuffer()
	return r.incoming, nil
}

func (r *VideoRingBuffer) initializeBuffer() {
	r.Buffer = ringbuffer.NewWeightedRingT[videox.VideoPacket](r.Buffer.MaxWeight)
}

func (r *VideoRingBuffer) AddPacketListener(name string, c chan *videox.VideoPacket, policy FullChannelPolicy) {
	r.packetListenerLock.Lock()
	defer r.packetListenerLock.Unlock()
	r.packetListeners = append(r.packetListeners, &RingBufferListener{
		Name:   name,
		Chan:   c,
		Policy: policy,
	})
}

func (r *VideoRingBuffer) RemovePacketListener(c chan *videox.VideoPacket) {
	r.packetListenerLock.Lock()
	defer r.packetListenerLock.Unlock()
	for i, listener := range r.packetListeners {
		if listener.Chan == c {
			r.packetListeners = append(r.packetListeners[:i], r.packetListeners[i+1:]...)
			return
		}
	}
}

func (r *VideoRingBuffer) Close() {
	r.Log.Infof("VideoRingBuffer closed")
}

func (r *VideoRingBuffer) OnPacketRTP(packet *videox.VideoPacket) {
	//r.Log.Infof("[Packet %v] VideoRingBuffer", 0)
	r.BufferLock.Lock()
	defer r.BufferLock.Unlock()

	//r.debugAnalyzePacket(packet)

	r.Buffer.Add(packet.PayloadBytes(), packet)

	now := time.Now()

	// Send packet to our listeners (eg videoRecorder)
	r.packetListenerLock.Lock()
	defer r.packetListenerLock.Unlock()
	for _, listener := range r.packetListeners {
		if len(listener.Chan) == cap(listener.Chan) {
			if listener.Policy == FullChannelPolicyDrop {
				listener.nDropped++
				if now.Sub(listener.lastLogMsg) > time.Second*3 {
					r.Log.Warnf("%v packets dropped for %v", listener.nDropped, listener.Name)
					listener.lastLogMsg = now
					listener.nDropped = 0
				}
				continue
			} else {
				listener.nStalled++
				if now.Sub(listener.lastLogMsg) > time.Second*3 {
					r.Log.Warnf("%v stalled packets for %v", listener.nStalled, listener.Name)
					listener.lastLogMsg = now
					listener.nStalled = 0
				}
			}
		}
		listener.Chan <- packet
	}
}

// Take BufferLock, then call ExtractRawBufferNoLock
func (r *VideoRingBuffer) ExtractRawBuffer(method ExtractMethod, duration time.Duration) (*videox.PacketBuffer, error) {
	r.BufferLock.Lock()
	defer r.BufferLock.Unlock()
	return r.ExtractRawBufferNoLock(method, duration)
}

// Extract from <video_end - duration> until <video_end>.
// video_end is the PTS of the most recently received packet.
// duration is a positive number.
// You must be holding BufferLock before calling this function.
func (r *VideoRingBuffer) ExtractRawBufferNoLock(method ExtractMethod, duration time.Duration) (*videox.PacketBuffer, error) {
	bufLen := r.Buffer.Len()
	if bufLen == 0 {
		return nil, fmt.Errorf("No video available")
	}

	// Compute the starting packet for extraction
	firstPacket := -1
	{
		_, lastPacket, _ := r.Buffer.Peek(bufLen - 1)
		endPTS := lastPacket.PTS

		// Assume that all cameras always send SPS + PPS + IDR in a single packet.
		oldestIDR := -1
		oldestIDRTimeDelta := time.Duration(0)
		satisfied := false
		for i := bufLen - 1; i >= 0; i-- {
			_, packet, _ := r.Buffer.Peek(i)
			timeDelta := endPTS - packet.PTS
			//r.Log.Infof("%v < %v ?", timeDelta, duration)
			if packet.HasAbstractType(videox.AbstractNALUTypeIDR) {
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
			return nil, fmt.Errorf("Not enough video frames in buffer (bufLen = %v)", bufLen)
		} else if !satisfied {
			// We failed to find enough frames to satisfy the desired duration.
			// In this case, we issue a warning, but return the best effort.
			r.Log.Warnf("Unable to satisfy request for %.1f seconds. Resulting video has only %.1f seconds", duration.Seconds(), oldestIDRTimeDelta.Seconds())
		}
	}

	out := &videox.PacketBuffer{
		Packets: make([]*videox.VideoPacket, bufLen-firstPacket),
	}

	switch method {
	case ExtractMethodShallowClone:
		for i := firstPacket; i < bufLen; i++ {
			_, packet, _ := r.Buffer.Peek(i)
			out.Packets[i-firstPacket] = packet
		}
	case ExtractMethodDeepClone:
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

// Scan backwards in the ring buffer to find the most recent packet containing an IDR frame
// Assumes that you are holding BufferLock
// Returns the index in the buffer, or -1 if none found
func (r *VideoRingBuffer) FindLatestIDRPacketNoLock() int {
	i := r.Buffer.Len() - 1
	for ; i >= 0; i-- {
		_, packet, _ := r.Buffer.Peek(i)
		if packet.HasAbstractType(videox.AbstractNALUTypeIDR) {
			return i
		}
	}
	return -1
}

func (r *VideoRingBuffer) debugAnalyzePacket(packet *videox.VideoPacket) {
	// My Hikvision cameras always send SPS + PPS + IDR in a single packet for h264.
	// I have no idea whether other cameras do the same thing.
	if packet.Codec == videox.CodecH264 {
		if packet.HasIDR() {
			sps := packet.FirstNALUOfType264(h264.NALUTypeSPS)
			pps := packet.FirstNALUOfType264(h264.NALUTypePPS)
			idr := packet.FirstNALUOfType264(h264.NALUTypeIDR)
			if sps != nil && pps != nil && idr != nil {
				fmt.Printf("IDR packet. SPS=%v PPS=%v IDR=%v\n", len(sps.Payload), len(pps.Payload), len(idr.Payload))
			} else {
				fmt.Printf("IDR packet without SPS and PPS\n")
			}
		}
	}
}
