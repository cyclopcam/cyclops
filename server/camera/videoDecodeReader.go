package camera

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cyclopcam/cyclops/pkg/accel"
	"github.com/cyclopcam/cyclops/pkg/videox"
	"github.com/cyclopcam/logs"
)

// VideoDecodeReader decodes the video stream and emits frames
// NOTE: Our lastImg is a copy of the most recent frame.
// This memcpy might be a substantial waste if you're decoding
// a high res stream, and only need access to the latest frame
// occasionally. Such a scenario might be better suited by
// a blocking call which waits for a new frame to be decoded,
// depending upon the acceptable latency.
type VideoDecodeReader struct {
	Log logs.Log
	//TrackID int
	//Track   *gortsplib.TrackH264
	Decoder *videox.VideoDecoder

	incoming     StreamSinkChan
	nPackets     int64
	lastPacketAt atomic.Int64 // Time when last packet was received (unix nanoseconds)
	ready        bool

	lastImgLock sync.Mutex
	lastImg     *accel.YUVImage // We store the YUV image, so that we can run motion analysis on Y only, and only pay YUV -> RGB cost on demand
	lastImgID   int64           // If zero, then no frames decoded. The first decoded frame is 1, and it increases with each new frame
	lastImgPTS  time.Time       // Presentation time of the last image
}

func NewVideoDecodeReader() *VideoDecodeReader {
	return &VideoDecodeReader{
		incoming: make(StreamSinkChan, StreamSinkChanDefaultBufferSize),
	}
}

func (r *VideoDecodeReader) OnConnect(stream *Stream) (StreamSinkChan, error) {
	r.Log = stream.Log

	decoder, err := videox.NewVideoStreamDecoder(stream.Codec)
	if err != nil {
		return nil, fmt.Errorf("Failed to start '%v' decoder: %w", stream.Codec, err)
	}

	// UPDATE: Sending SPS and PPS now doesn't actually help. avcodec wants SPS+PPS+IDR to start decoding,
	// and with my IP cameras, those always come in a packet together. In other words,
	// the moment we see our first keyframe, we will also see SPS and PPS, so there's
	// no use in trying to inject them now.
	/*
		// if present, send SPS and PPS from the SDP to the decoder
		sps := r.Track.SPS()
		pps := r.Track.PPS()
		if sps != nil {
			wrapped := videox.WrapRawNALU(sps)
			decoder.Decode(wrapped)
		}
		if pps != nil {
			wrapped := videox.WrapRawNALU(pps)
			decoder.Decode(wrapped)
		}
	*/

	r.Decoder = decoder
	return r.incoming, nil
}

// Return a copy of the latest image and its ID, if it's different to the given ID
func (r *VideoDecodeReader) GetLastImageIfDifferent(ifNotEqualTo int64) (*accel.YUVImage, int64, time.Time) {
	r.lastImgLock.Lock()
	defer r.lastImgLock.Unlock()
	if r.lastImg == nil || r.lastImgID == ifNotEqualTo {
		return nil, 0, time.Time{}
	}
	return r.lastImg.Clone(), r.lastImgID, r.lastImgPTS
}

// Return the time when the last packet was received
func (r *VideoDecodeReader) LastPacketAt() time.Time {
	t := r.lastPacketAt.Load()
	if t == 0 {
		return time.Time{}
	} else {
		return time.Unix(0, t)
	}
}

// Return a copy of the most recently decoded frame (or nil, if there is none available yet), and the frame ID
func (r *VideoDecodeReader) LastImageCopy() (*accel.YUVImage, int64) {
	r.lastImgLock.Lock()
	defer r.lastImgLock.Unlock()
	if r.lastImg == nil {
		return nil, 0
	}
	return r.lastImg.Clone(), r.lastImgID
}

func (r *VideoDecodeReader) Close() {
	r.Log.Infof("VideoDecodeReader closed")
	if r.Decoder != nil {
		r.Decoder.Close()
		r.Decoder = nil
	}
}

func (r *VideoDecodeReader) OnPacketRTP(packet *videox.VideoPacket) {
	r.nPackets++
	//r.Log.Infof("[Packet %v] VideoDecodeReader", r.nPackets)

	r.lastPacketAt.Store(time.Now().UnixNano())

	if packet.HasAbstractType(videox.AbstractNALUTypeIDR) {
		// In my experience so far, the packets with IDRs also have SPS,PPS,etc
		r.ready = true
	}

	if !r.ready && packet.IsIFrame() {
		//r.Log.Infof("NALU %v (discard)", ntype)
		return
	}
	//r.Log.Infof("NALU %v", ntype)

	// NOTE: To avoid the "no frame!" warnings on stdout/stderr, which ffmpeg emits, we must not send SPS
	// and PPS refresh NALUs to the decoder alone. Instead, we must join them into the next IDR, and
	// send SPS+PPS+IDR as a single packet.
	// That's why we join all NALUs into a single packet and send that to avcodec.

	// convert NALUs to RGB frames
	frame, err := r.Decoder.DecodeDeepRef(packet)
	if err != nil {
		r.Log.Errorf("Failed to decode %v NALU: %v", packet.Codec, err)
		return
	}

	if frame == nil {
		return
	}

	// Experiment to see if we need to drain frames for h265. This wasn't necessary for h264.
	drain, _ := r.Decoder.ReceiveFrameDeepRef()
	if drain != nil {
		r.Log.Warnf("Successfully drained frame")
		frame = drain
	}

	// The 'img' returned by Decode is transient, so we need make a copy of it.
	// See comment about the potential wastefulness of this memcpy at the top of VideoDecodeReader
	r.cloneIntoLastImg(frame.Image, packet.WallPTS)
	//r.Log.Infof("[Packet %v] Decoded frame with size %v", r.nPackets, img.Bounds().Max)
}

func (r *VideoDecodeReader) cloneIntoLastImg(latest *accel.YUVImage, pts time.Time) {
	r.lastImgLock.Lock()
	if r.lastImg == nil ||
		r.lastImg.Width != latest.Width ||
		r.lastImg.Height != latest.Height {
		r.lastImg = latest.Clone()
	} else {
		r.lastImg.CopyFrom(latest)
	}
	r.lastImgID++
	r.lastImgPTS = pts
	r.lastImgLock.Unlock()
}
