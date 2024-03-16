package camera

import (
	"math"
	"time"

	"github.com/bluenviron/mediacommon/pkg/codecs/h264"
	"github.com/cyclopcam/cyclops/pkg/log"
	"github.com/cyclopcam/cyclops/pkg/videoformat/fsv"
	"github.com/cyclopcam/cyclops/pkg/videoformat/rf1"
	"github.com/cyclopcam/cyclops/pkg/videox"
)

// VideoRecorder writes incoming packets to our 'fsv' video archive
// We operate on top of VideoRingBuffer, so that we can start recording
// at any moment, and have some history of packets to write to the archive.
// For event-triggered recording modes, this is vital because you always
// want some history that preceded the moment of the event trigger.
// For continuous recording modes this is not important.
type VideoRecorder struct {
	Log              log.Log
	ringBuffer       *VideoRingBuffer
	archive          *fsv.Archive
	streamName       string
	stop             chan bool
	onPacket         chan *videox.VideoPacket
	videoWidth       int
	videoHeight      int
	lastWriteWarning time.Time
}

func StartVideoRecorder(ringBuffer *VideoRingBuffer, streamName string, archive *fsv.Archive, includeHistory time.Duration) *VideoRecorder {
	r := &VideoRecorder{
		Log:        ringBuffer.Log,
		ringBuffer: ringBuffer,
		streamName: streamName,
		archive:    archive,
		stop:       make(chan bool),
		onPacket:   make(chan *videox.VideoPacket),
	}
	r.start(includeHistory)
	return r
}

func (r *VideoRecorder) Stop() {
	r.Log.Infof("VideoRecorder stopped")
	close(r.stop)
}

func (r *VideoRecorder) start(includeHistory time.Duration) {
	r.stop = make(chan bool)

	go r.recordFunc(includeHistory)
}

// recordFunc runs on its own thread, consuming packets from the ring buffer (via a channel),
// and writing them to the archive.
func (r *VideoRecorder) recordFunc(includeHistory time.Duration) {
	// Before we can start writing to our archive, we need to make sure that we have
	// a SPS + PPS + IDR packet, otherwise the codec can't start.
	// If we've been connected to the camera for a few seconds, then this phase will
	// usually complete instantaneously. The only expected case where this will take
	// a few seconds to complete, is if the recorder is started as soon as the system
	// starts up. This happens if we're in continuous recording mode.
	waitingForIDR := true
	nAttempts := 0
	for waitingForIDR {
		maxWaitPower := min(nAttempts, 7) // 6 = pause of 2.2 seconds, 7 = 3.4 seconds.
		pause := time.Millisecond * 200 * time.Duration(math.Pow(1.5, float64(maxWaitPower)))
		select {
		case <-r.stop:
			r.Log.Infof("Stream %v recorder stopped (before it started)", r.streamName)
			return
		case <-time.After(pause):
			// --- Lock ---
			r.ringBuffer.BufferLock.Lock()
			history, err := r.ringBuffer.ExtractRawBufferNoLock(ExtractMethodShallowClone, includeHistory)
			if err != nil {
				if nAttempts == 5 {
					r.Log.Warnf("Stream %v recorder not ready yet: %v", r.streamName, err)
				}
			} else {
				if err := r.extractStreamParameters(history); err != nil {
					r.Log.Warnf("Recorder of stream %v failed to extract parameters (width/height): %v", r.streamName, err)
				} else {
					// It's crucial that we add our packet listener inside r.ringBuffer.BufferLock.
					// This guarantees that we don't miss any packets.
					waitingForIDR = false
					r.ringBuffer.AddPacketListener(r.onPacket)
				}
			}
			r.ringBuffer.BufferLock.Unlock()
			// --- Unlock ---

			// Write our packets outside of the lock
			if !waitingForIDR {
				r.writePackets(history.Packets)
			}
		}
	}
	startAt := time.Now()
	r.Log.Infof("Stream %v recorder starting")

	for {
		select {
		case <-r.stop:
			r.Log.Infof("Stream %v recorder stopped after %v", r.streamName, time.Now().Sub(startAt))
			return
		case packet := <-r.onPacket:
			r.writePackets([]*videox.VideoPacket{packet})
		}
	}
}

func (r *VideoRecorder) extractStreamParameters(buffer *videox.PacketBuffer) error {
	width, height, err := buffer.DecodeHeader()
	if err != nil {
		return err
	}
	r.videoWidth = width
	r.videoHeight = height
	return nil
}

func (r *VideoRecorder) writePackets(packets []*videox.VideoPacket) {
	nalus := []rf1.NALU{}
	for _, p := range packets {
		for _, in := range p.H264NALUs {
			inType := in.Type()
			var flags rf1.IndexNALUFlags
			if inType == h264.NALUTypePPS || inType == h264.NALUTypeSPS {
				flags |= rf1.IndexNALUFlagEssentialMeta
			}
			if inType == h264.NALUTypeIDR {
				flags |= rf1.IndexNALUFlagKeyFrame
			}
			out := rf1.NALU{
				PTS:     p.WallPTS,
				Flags:   flags,
				Payload: in.Payload,
			}
			nalus = append(nalus, out)
		}
	}
	tracks := map[string]fsv.TrackPayload{
		"video": fsv.MakeVideoPayload(rf1.CodecH264, r.videoWidth, r.videoHeight, nalus),
	}
	if err := r.archive.Write(r.streamName, tracks); err != nil {
		now := time.Now()
		if now.Sub(r.lastWriteWarning) > time.Second*30 {
			r.lastWriteWarning = now
			r.Log.Warnf("Stream %v recorder failed to write to archive: %v", r.streamName, err)
		}
	}
}
