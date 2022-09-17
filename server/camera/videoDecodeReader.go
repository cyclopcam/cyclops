package camera

import (
	"fmt"
	"image"
	"sync"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/h264"
	"github.com/bmharper/cyclops/pkg/log"
	"github.com/bmharper/cyclops/server/videox"
)

// VideoDecodeReader decodes the video stream and emits frames
// NOTE: Our lastImg is a copy of the most recent frame.
// This memcpy might be a substantial waste if you're decoding
// a high res stream, and only need access to the latest frame
// occasionally. Such a scenario might be better suited by
// a blocking call which waits for a new frame to be decoded.
type VideoDecodeReader struct {
	Log     log.Log
	TrackID int
	Track   *gortsplib.TrackH264
	Decoder *videox.H264Decoder

	incoming StreamSinkChan
	nPackets int64
	ready    bool

	lastImgLock sync.Mutex
	lastImg     image.Image
}

func NewVideoDecodeReader() *VideoDecodeReader {
	return &VideoDecodeReader{
		incoming: make(StreamSinkChan, StreamSinkChanDefaultBufferSize),
	}
}

func (r *VideoDecodeReader) OnConnect(stream *Stream) (StreamSinkChan, error) {
	r.Log = stream.Log
	r.TrackID = stream.H264TrackID
	r.Track = stream.H264Track

	decoder, err := videox.NewH264Decoder()
	if err != nil {
		return nil, fmt.Errorf("Failed to start H264 decoder: %w", err)
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

func (r *VideoDecodeReader) LastImage() image.Image {
	r.lastImgLock.Lock()
	defer r.lastImgLock.Unlock()
	return r.lastImg
}

func (r *VideoDecodeReader) Close() {
	r.Log.Infof("VideoDecodeReader closed")
	if r.Decoder != nil {
		r.Decoder.Close()
		r.Decoder = nil
	}
}

func (r *VideoDecodeReader) OnPacketRTP(packet *videox.DecodedPacket) {
	r.nPackets++
	//r.Log.Infof("[Packet %v] VideoDecodeReader", r.nPackets)

	if packet.HasType(h264.NALUTypeIDR) {
		// we'll assume that we've seen SPS and PPS by now... but should perhaps wait for them too
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

	// convert H264 NALUs to RGBA frames
	img, err := r.Decoder.Decode(packet)
	if err != nil {
		r.Log.Errorf("Failed to decode H264 NALU: %v", err)
		return
	}

	if img == nil {
		return
	}

	// The 'img' returned by Decode is transient, so we need make a copy of it.
	// See comment about the potential wastefulness of this memcpy at the top of VideoDecodeReader
	r.cloneIntoLastImg(img)
	//r.Log.Infof("[Packet %v] Decoded frame with size %v", r.nPackets, img.Bounds().Max)
}

func (r *VideoDecodeReader) cloneIntoLastImg(latest image.Image) {
	r.lastImgLock.Lock()
	if r.lastImg == nil || !r.lastImg.Bounds().Eq(latest.Bounds()) {
		r.lastImg = image.NewRGBA(latest.Bounds())
	}
	src := latest.(*image.RGBA)
	dst := r.lastImg.(*image.RGBA)
	h := src.Rect.Dy()
	for i := 0; i < h; i++ {
		copy(dst.Pix[i*dst.Stride:(i+1)*dst.Stride], src.Pix[i*src.Stride:(i+1)*src.Stride])
	}
	r.lastImgLock.Unlock()
}
