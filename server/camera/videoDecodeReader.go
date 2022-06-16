package camera

import (
	"fmt"

	"github.com/aler9/gortsplib"
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
	if track.SPS() != nil {
		decoder.Decode(track.SPS())
	}
	if track.PPS() != nil {
		decoder.Decode(track.PPS())
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

	for _, nalu := range ctx.H264NALUs {
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
