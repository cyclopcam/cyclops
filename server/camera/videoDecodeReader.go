package camera

import (
	"fmt"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/h264"
	"github.com/bmharper/cyclops/server/log"
	"github.com/bmharper/cyclops/server/videox"
)

// VideoDecodeReader decodes the video stream and emits frames
type VideoDecodeReader struct {
	Log     log.Log
	TrackID int
	Track   *gortsplib.TrackH264
	Decoder *videox.H264Decoder

	nPackets int64
	ready    bool
	//sps      *videox.NALU
	//pps      *videox.NALU
}

func (r *VideoDecodeReader) Initialize(log log.Log, trackID int, track *gortsplib.TrackH264) error {
	r.Log = log
	r.TrackID = trackID
	r.Track = track

	decoder, err := videox.NewH264Decoder()
	if err != nil {
		return fmt.Errorf("Failed to start H264 decoder: %w", err)
	}

	// if present, send SPS and PPS from the SDP to the decoder
	//var sps *videox.NALU
	//var pps *videox.NALU
	sps := track.SPS()
	pps := track.PPS()
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
	return nil
}

func (r *VideoDecodeReader) Close() {
	r.Log.Infof("VideoDecodeReader closed")
	r.Decoder.Close()
}

func (r *VideoDecodeReader) OnPacketRTP(ctx *gortsplib.ClientOnPacketRTPCtx) {
	r.nPackets++
	//r.Log.Infof("[Packet %v] VideoDecodeReader", r.nPackets)

	if ctx.TrackID != r.TrackID {
		return
	}

	if ctx.H264NALUs == nil {
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

		// wait for a frame
		if img == nil {
			continue
		}

		//r.Log.Infof("[Packet %v] Decoded frame with size %v", r.nPackets, img.Bounds().Max)
	}
}
