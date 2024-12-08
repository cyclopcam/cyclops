package camera

import (
	"math"
	"time"

	"github.com/bluenviron/mediacommon/pkg/codecs/h264"
	"github.com/cyclopcam/cyclops/pkg/gen"
	"github.com/cyclopcam/cyclops/pkg/videoformat/fsv"
	"github.com/cyclopcam/cyclops/pkg/videoformat/rf1"
	"github.com/cyclopcam/cyclops/pkg/videox"
	"github.com/cyclopcam/logs"
)

// VideoRecorder writes incoming packets to our 'fsv' video archive
// We operate on top of VideoRingBuffer, so that we can start recording
// at any moment, and have some history of packets to write to the archive.
// For event-triggered recording modes, this is vital because you always
// want some history that preceded the moment of the event trigger.
// For continuous recording modes this is not important.
type VideoRecorder struct {
	Log              logs.Log
	ringBuffer       *VideoRingBuffer
	archive          *fsv.Archive
	streamName       string
	stop             chan bool
	onPacket         chan *videox.VideoPacket
	videoWidth       int
	videoHeight      int
	lastWriteWarning time.Time
}

// Create a new video recorder and start recording.
// This function is expected to return very quickly. Specifically, the code inside LiveCameras
// that starts/stops recorders holds a lock while it performs this operation, assuming that
// this function will return very quickly.
func StartVideoRecorder(ringBuffer *VideoRingBuffer, streamName string, archive *fsv.Archive, includeHistory time.Duration) *VideoRecorder {
	r := &VideoRecorder{
		Log:        ringBuffer.Log,
		ringBuffer: ringBuffer,
		streamName: streamName,
		archive:    archive,
		stop:       make(chan bool),
		// Our packet receive channel must have a non-zero buffer to avoid a deadlock
		// when we stop recording. After we call RemovePacketListener(), the ringbuffer
		// might still try sending a packet or two. I can't think of a deterministic
		// way to avoid this situation. The non-zero channel buffer is a band-aid that
		// will probably work just fine in practice, but I would prefer to have a
		// bullet-proof solution.
		onPacket: make(chan *videox.VideoPacket, 10),
	}
	r.start(includeHistory)
	return r
}

// Stop recording.
// Like StartVideoRecorder(), this function is expected to return immediately.
func (r *VideoRecorder) Stop() {
	//r.Log.Infof("VideoRecorder stopped")
	close(r.stop)
}

func (r *VideoRecorder) start(includeHistory time.Duration) {
	r.stop = make(chan bool)

	go r.recordFunc(includeHistory)
}

// recordFunc runs on its own thread, consuming packets from the ring buffer (via a channel),
// and writing them to the archive.
func (r *VideoRecorder) recordFunc(includeHistory time.Duration) {
	r.Log.Infof("RecordFunc enter")
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
		if nAttempts == 0 {
			pause = 0
		}
		select {
		case <-r.stop:
			r.Log.Infof("Recorder stopped (before it started)")
			return
		case <-time.After(pause):
			// --- Lock ---
			r.ringBuffer.BufferLock.Lock()
			history, err := r.ringBuffer.ExtractRawBufferNoLock(ExtractMethodShallowClone, includeHistory)
			if err != nil {
				if nAttempts == 5 {
					r.Log.Warnf("Recorder not ready yet: %v", err)
				}
			} else {
				if err := r.extractStreamParameters(history); err != nil {
					r.Log.Warnf("Recorder failed to extract parameters (width/height): %v", err)
				} else {
					// It's crucial that we add our packet listener inside r.ringBuffer.BufferLock.
					// This guarantees that we don't miss any packets.
					waitingForIDR = false
					r.ringBuffer.AddPacketListener("VideoRecorder", r.onPacket, FullChannelPolicyDrop)
				}
			}
			r.ringBuffer.BufferLock.Unlock()
			// --- Unlock ---

			// Write our packets outside of the lock.
			if !waitingForIDR {
				// For an HD stream, it's likely that this backlog contains a lot of data. Enough data to
				// fill up the write buffer of Archive. This is why immediately after writing our backlog,
				// we trigger a flush of the write buffer.
				r.writePackets(history.Packets)
				r.archive.TriggerWriterBufferFlush()
			}
		}
	}
	startAt := time.Now()
	r.Log.Infof("Recorder starting")

	for {
		select {
		case <-r.stop:
			r.Log.Infof("Recorder stopping")
			r.ringBuffer.RemovePacketListener(r.onPacket)
			gen.DrainChannel(r.onPacket)
			r.Log.Infof("Recorder stopped after %v", time.Now().Sub(startAt))
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
	nalus := []fsv.NALU{}
	for _, p := range packets {
		for _, in := range p.H264NALUs {
			inType := in.Type()
			// We always encode as Annex-B, because this makes it very easy to
			// get video on the screen using easily available tools.
			// For example, you can take an fsv archive file and use ffmpeg do
			// extract frames, or convert it to an mp4 or whatever.
			flags := fsv.NALUFlagAnnexB
			if inType == h264.NALUTypePPS || inType == h264.NALUTypeSPS {
				flags |= fsv.NALUFlagEssentialMeta
			}
			if inType == h264.NALUTypeIDR {
				flags |= fsv.NALUFlagKeyFrame
			}
			out := fsv.NALU{
				PTS:     p.WallPTS,
				Flags:   flags,
				Payload: in.AsAnnexB().Payload,
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
			r.Log.Warnf("Recorder failed to write to archive: %v", err)
		}
	}
}
