package camera

import (
	"github.com/aler9/gortsplib"
	"github.com/bmharper/cyclops/server/log"
)

type VideoReader interface {
	Initialize(log log.Log, trackID int, track *gortsplib.TrackH264) error
	Close()
	OnPacketRTP(ctx *gortsplib.ClientOnPacketRTPCtx)
}
