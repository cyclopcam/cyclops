package camera

import (
	"fmt"

	"github.com/aler9/gortsplib"
	"github.com/bmharper/cyclops/server/log"
	"github.com/bmharper/cyclops/server/videox"
)

// VideoDumpReader is a ring buffer that accumulates the stream in a format that can be turned into a video file.
// The video is not decoded.
type VideoDumpReader struct {
	Log      log.Log
	TrackID  int
	Track    *gortsplib.TrackH264
	Encoder  *videox.MPGTSEncoder
	Filename string
}

func (r *VideoDumpReader) Initialize(log log.Log, trackID int, track *gortsplib.TrackH264) error {
	r.Log = log
	r.TrackID = trackID
	r.Track = track

	// setup H264->MPEGTS encoder
	encoder, err := videox.NewMPEGTSEncoder(log, r.Filename, track.SPS(), track.PPS())
	if err != nil {
		return fmt.Errorf("Failed to start MPEGTS encoder: %w", err)
	}
	r.Encoder = encoder
	return nil
}

func (r *VideoDumpReader) Close() {
	r.Log.Infof("VideoDumpReader closed")
	r.Encoder.Close()
}

func (r *VideoDumpReader) OnPacketRTP(ctx *gortsplib.ClientOnPacketRTPCtx) {
	//r.Log.Infof("[Packet %v] VideoDumpReader", 0)
	if ctx.TrackID != r.TrackID {
		return
	}

	if ctx.H264NALUs == nil {
		return
	}

	// encode H264 NALUs into MPEG-TS
	err := r.Encoder.Encode(ctx.H264NALUs, ctx.H264PTS)
	if err != nil {
		r.Log.Errorf("MPGTS Encode failed: %v", err)
		return
	}
}
