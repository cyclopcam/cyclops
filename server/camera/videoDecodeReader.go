package camera

import (
	"fmt"
	"image"
	"sync"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/h264"
	"github.com/bmharper/cyclops/server/log"
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

	// if present, send SPS and PPS from the SDP to the decoder
	//var sps *videox.NALU
	//var pps *videox.NALU
	sps := r.Track.SPS()
	pps := r.Track.PPS()
	if sps != nil {
		wrapped := videox.WrapRawNALU(sps)
		//r.sps = &wrapped
		decoder.Decode(wrapped)
	}
	if pps != nil {
		wrapped := videox.WrapRawNALU(pps)
		//r.pps = &wrapped
		decoder.Decode(wrapped)
	}

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

func (r *VideoDecodeReader) OnPacketRTP(ctx *gortsplib.ClientOnPacketRTPCtx) {
	r.nPackets++
	//r.Log.Infof("[Packet %v] VideoDecodeReader", r.nPackets)

	if ctx.TrackID != r.TrackID || ctx.H264NALUs == nil {
		return
	}

	for _, rawNalu := range ctx.H264NALUs {
		nalu := videox.WrapRawNALU(rawNalu)
		ntype := nalu.Type()
		//switch ntype {
		//case h264.NALUTypeSPS:
		//	r.sps = &nalu
		//case h264.NALUTypePPS:
		//	r.pps = &nalu
		//}

		if ntype == h264.NALUTypeIDR {
			// we'll assume that we've seen SPS and PPS by now... but should perhaps wait for them too
			r.ready = true
		}

		if !r.ready && videox.IsVisualPacket(ntype) {
			//r.Log.Infof("NALU %v (discard)", ntype)
			continue
		}
		//r.Log.Infof("NALU %v", ntype)

		// NOTE: To avoid the "no frame!" warnings on stdout/stderr, which ffmpeg emits, we must not send SPS
		// and PPS refresh NALUs to the decoder alone. Instead, we must join them into the next IDR, and
		// send SPS+PPS+IDR as a single packet. I HAVE NOT TESTED THIS THEORY!

		// convert H264 NALUs to RGBA frames
		img, err := r.Decoder.Decode(nalu)
		if err != nil {
			r.Log.Errorf("Failed to decode H264 NALU: %v", err)
			continue
		}

		if img == nil {
			continue
		}

		// The 'img' returned by Decode is transient, so we need make a copy of it.
		r.cloneIntoLastImg(img)
		//r.Log.Infof("[Packet %v] Decoded frame with size %v", r.nPackets, img.Bounds().Max)
	}
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
