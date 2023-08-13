package server

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aler9/gortsplib/pkg/h264"
	"github.com/bmharper/cyclops/pkg/dbh"
	"github.com/bmharper/cyclops/pkg/gen"
	"github.com/bmharper/cyclops/pkg/log"
	"github.com/bmharper/cyclops/server/camera"
	"github.com/bmharper/cyclops/server/configdb"
	"github.com/bmharper/cyclops/server/defs"
	"github.com/bmharper/cyclops/server/eventdb"
	"github.com/bmharper/cyclops/server/videox"
)

// When doing long recordings, we split video files into chunks of approximately this size
// While developing, it's nice to have smaller video files (so we can see them earlier),
// but in production we'll probably want this to be chunkier (eg 1GB).
const MaxVideoFileSize = 64 * 1024 * 1024

const BackgroundRecorderTickInterval = time.Second
const MaxVideoFileSizeCheckInterval = 20 * time.Second

type backgroundRecorder struct {
	instructionID int64 // ID of record_instruction record in config DB
	startAt       time.Time
	stop          atomic.Bool
	topRecording  *eventdb.Recording
}

func (s *Server) RunBackgroundRecorderLoop() {
	go func() {
		for {
			t := time.NewTimer(BackgroundRecorderTickInterval)
			shutdown := false
			select {
			case <-s.ShutdownStarted:
				shutdown = true
			case <-t.C:
			}
			t.Stop()
			if shutdown {
				break
			}
			if err := s.startStopBackgroundRecorders(); err != nil {
				s.Log.Errorf("startStopBackgroundRecorders error: %v", err)
			}
		}
	}()
}

func (s *Server) startStopBackgroundRecorders() error {
	instructions := []configdb.RecordInstruction{}
	if err := s.configDB.DB.Find(&instructions).Error; err != nil {
		return err
	}

	// figure out which recorders to stop
	removeList := []*backgroundRecorder{}
	for _, bg := range s.backgroundRecorders {
		found := false
		for _, ins := range instructions {
			if bg.instructionID == ins.ID && ins.FinishAt.Get().After(time.Now()) {
				found = true
			}
		}
		if !found {
			removeList = append(removeList, bg)
		}
	}

	// figure out which recorders to start
	now := time.Now()
	addList := []configdb.RecordInstruction{}
	for _, ins := range instructions {
		found := false
		for _, bg := range s.backgroundRecorders {
			if ins.ID == bg.instructionID {
				found = true
			}
		}
		//fmt.Printf("%v ....... %v   (now: %v), (%v), (%v)\n", ins.StartAt.Get(), ins.FinishAt.Get(), now, ins.StartAt.Get().After(now.Add(-BackgroundRecorderTickInterval)), ins.FinishAt.Get().After(now))
		// see comments in startBackgroundRecorder about why we want to start a little ahead of time
		grace := BackgroundRecorderTickInterval * 2
		if !found && ins.StartAt.Get().Before(now.Add(grace)) && ins.FinishAt.Get().After(now) {
			//fmt.Printf("Adding!\n")
			addList = append(addList, ins)
		}
	}

	// Stop recorders
	for _, bg := range removeList {
		bg.stop.Store(true)
		gen.DeleteFromSliceUnordered(s.backgroundRecorders, gen.IndexOf(s.backgroundRecorders, bg))
	}

	// Start recorders
	for _, ins := range addList {
		bg := &backgroundRecorder{
			instructionID: ins.ID,
			startAt:       ins.StartAt.Get(),
		}
		if err := bg.startBackgroundRecorder(s, defs.Resolution(ins.Resolution)); err != nil {
			s.Log.Errorf("Failed to start background recorder: %v", err)
		} else {
			s.backgroundRecorders = append(s.backgroundRecorders, bg)
		}
	}

	// Delete old instructions
	// By this stage, the Go recorder function will have stopped.
	// We wake up every 1 second, and here we're providing 24 hours grace, so that is a monumental buffer.
	longAgo := time.Now().Add(-24 * time.Hour)
	if err := s.configDB.DB.Delete(&configdb.RecordInstruction{}, "finish_at < ?", dbh.MakeIntTime(longAgo)).Error; err != nil {
		return err
	}

	return nil
}

type backgroundStream struct {
	server     *Server
	log        log.Log
	parent     *backgroundRecorder
	camera     *camera.Camera
	stream     *camera.Stream
	sink       camera.StreamSinkChan
	resolution defs.Resolution
	width      int
	height     int

	// encoderLock locks ALL of the items in this group
	encoderLock   sync.Mutex
	encoder       *videox.VideoEncoder
	ptsStart      time.Duration
	recording     *eventdb.Recording
	videoFilename string
	haveThumbnail bool
	lastSizeCheck time.Time
}

func (bg *backgroundStream) OnConnect(stream *camera.Stream) (camera.StreamSinkChan, error) {
	return bg.sink, nil
}

func (bg *backgroundStream) OnPacketRTP(packet *videox.DecodedPacket) {
	if bg.parent.stop.Load() {
		// Remove ourselves
		bg.stream.RemoveSink(bg.sink)
		bg.Close()
		return
	}

	if bg.width == 0 && packet.HasType(h264.NALUTypeSPS) {
		width, height, err := videox.ParseSPS(packet.FirstNALUOfType(h264.NALUTypeSPS).RawPayload())
		if err != nil {
			bg.log.Errorf("Failed to decode SPS: %v", err)
		} else {
			bg.width = width
			bg.height = height
			bg.log.Infof("Decoded SPS: %v x %v", width, height)
		}
	}

	bg.encoderLock.Lock()
	defer bg.encoderLock.Unlock()

	// We create our encoder after successfully decoding an SPS NALU, and we see our first keyframe
	if bg.encoder == nil {
		if !packet.HasIDR() || bg.width == 0 {
			return
		}
		if !(packet.HasType(h264.NALUTypeSPS) && packet.HasType(h264.NALUTypePPS)) {
			// If we hit this in practice, then we'll have to synthesize our very first
			// packet by joining SPS + PPS + IDR into one packet.
			// My hikvision cameras all send SPS+PPS+IDR whenever they send a keyframe,
			// but other cameras might differ.
			bg.log.Errorf("Expected IDR frame to also contain SPS and PPS NALU")
			return
		}
		bg.log.Infof("First keyframe")

		// Try decoding a thumbnail
		img, err := videox.DecodeSinglePacketToImage(packet)
		if err != nil {
			bg.log.Errorf("DecodeSinglePacketToImage failed: %v", err)
			return
		}

		recording, err := bg.server.permanentEvents.CreateRecording(
			bg.parent.topRecording.ID, eventdb.RecordTypePhysical, eventdb.RecordingOriginBackground, time.Now(), bg.camera.ID(), bg.resolution, bg.width, bg.height)
		if err != nil {
			bg.log.Errorf("CreateRecording failed: %v", err)
			return
		}
		allGood := false
		defer func() {
			if !allGood {
				// cleanup dead record
				bg.server.permanentEvents.DeleteRecordingDBRecord(recording.ID)
			}
		}()

		videoFilename := bg.server.permanentEvents.FullPath(recording.VideoFilename(bg.resolution))
		thumbnailFilename := bg.server.permanentEvents.FullPath(recording.ThumbnailFilename())
		os.MkdirAll(filepath.Dir(videoFilename), 0770)
		if err := bg.server.permanentEvents.SaveThumbnail(img, thumbnailFilename); err != nil {
			bg.log.Errorf("SaveThumbnail failed: %v", err)
			return
		}

		encoder, err := videox.NewVideoEncoder("mp4", videoFilename, bg.width, bg.height)
		if err != nil {
			bg.log.Errorf("Error creating encoder: %v", err)
			return
		}

		bg.log.Infof("Starting video file %v", videoFilename)

		// success
		bg.recording = recording
		bg.videoFilename = videoFilename
		bg.encoder = encoder
		bg.ptsStart = packet.H264PTS
		bg.haveThumbnail = false
		bg.lastSizeCheck = time.Now()
		allGood = true
	}

	pts := packet.H264PTS - bg.ptsStart
	if err := bg.encoder.WritePacket(pts, pts, packet); err != nil {
		bg.log.Errorf("WritePacket failed: %v", err)
	}

	now := time.Now()

	if now.Sub(bg.lastSizeCheck) > MaxVideoFileSizeCheckInterval {
		//bg.log.Debugf("Checking video file size...")
		bg.lastSizeCheck = now
		st, err := os.Stat(bg.videoFilename)
		if err == nil {
			if st.Size() >= MaxVideoFileSize {
				bg.log.Infof("Finishing video file %v and starting another", bg.videoFilename)
				// finish this video, and on the next keyframe, we'll start another
				bg.finishVideoNoLock()
				bg.recording = nil
			}
		}
	}
}

func (bg *backgroundStream) Close() {
	bg.log.Infof("backgroundStream.Close() start")
	bg.encoderLock.Lock()
	defer bg.encoderLock.Unlock()
	if bg.encoder != nil {
		bg.finishVideoNoLock()
	}
	bg.log.Infof("backgroundStream.Close() done")
}

// You must already be holding encoderLock before calling this
func (bg *backgroundStream) finishVideoNoLock() {
	if err := bg.encoder.WriteTrailer(); err != nil {
		bg.log.Errorf("WriteTrailer failed: %v", err)
	} else {
		bg.log.Infof("WriteTrailer done")
	}
	bg.encoder.Close()
	bg.encoder = nil
}

// Record until we see 'bg.stop' or a server shutdown
func (bg *backgroundRecorder) startBackgroundRecorder(s *Server, resolution defs.Resolution) error {
	cameras := s.LiveCameras.Cameras()

	s.Log.Infof("Start BG recorder %v (start at %v)", bg.instructionID, bg.startAt)

	// Before starting, make sure that everything looks ready
	for _, cam := range cameras {
		stream := cam.GetStream(resolution)
		if stream == nil {
			return fmt.Errorf("Stream '%v' not found in camera %v", resolution, cam.Name())
		}
		info := stream.Info()
		if info == nil {
			return fmt.Errorf("Width and height are unknown on camera %v", cam.Name())
		}
	}

	// Wait for our precise starting moment. This precision is desirable for the midnight
	// moment when we switch over from one recording to another. We want minimal overlap,
	// but also zero gaps.
	// The 3 seconds grace that we add here causes us to start recording 3 seconds before
	// our specified time. This is here to account for any delays we might have during
	// startup, and also to ensure that we have received a keyframe by the time our specified
	// recording start time hits.
	targetStartTime := bg.startAt.Add(-3 * time.Second)
	pauseBeforeStart := targetStartTime.Sub(time.Now())
	if pauseBeforeStart > 0 {
		s.Log.Infof("BG recorder start, pausing for %.3f seconds", pauseBeforeStart.Seconds())
		select {
		case <-s.ShutdownStarted:
			return fmt.Errorf("Server is shutting down")
		case <-time.After(pauseBeforeStart):
		}
	}
	startAt := time.Now()

	topRecording, err := s.permanentEvents.CreateRecording(0, eventdb.RecordTypeLogical, eventdb.RecordingOriginBackground, startAt, 0, resolution, 0, 0)
	if err != nil {
		return err
	}
	bg.topRecording = topRecording

	// Connect stream sinks to all cameras
	for _, cam := range cameras {
		stream := cam.GetStream(resolution)
		bgs := &backgroundStream{
			server:     s,
			log:        log.NewPrefixLogger(s.Log, fmt.Sprintf("BG Recorder %v: (%v) %v", bg.instructionID, resolution, cam.Name())),
			parent:     bg,
			camera:     cam,
			stream:     stream,
			sink:       make(camera.StreamSinkChan, 5),
			resolution: resolution,
		}
		if err := stream.ConnectSinkAndRun(bgs); err != nil {
			return err
		}
	}

	s.Log.Infof("Start BG recorder %v success", bg.instructionID)

	return nil
}
