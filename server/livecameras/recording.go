package livecameras

import (
	"path/filepath"
	"time"

	"github.com/cyclopcam/cyclops/server/camera"
	"github.com/cyclopcam/cyclops/server/configdb"
	"github.com/cyclopcam/cyclops/server/monitor"
)

// This thread runs continuously, monitoring activity on cameras, and deciding
// when to start/stop recording on the cameras.
func (s *LiveCameras) recorderThread() {
	s.log.Infof("Recorder thread starting")

	keepRunning := true
	for keepRunning {
		select {
		case <-s.shutdown:
			keepRunning = false
			break
		case mm := <-s.allCameraMonitorMsg:
			s.processMonitorMessage(mm)
		case <-time.After(time.Second * 2):
			s.startStopRecorderForAllCameras()
		case <-s.recordThreadWake:
			s.startStopRecorderForAllCameras()
		}
	}

	s.stopAllRecorders()

	s.log.Infof("Recorder thread shutdown complete")

	close(s.recordThreadShutdown)
}

// Find or create the state for this camera.
// Assumes that you've already acquired recordStateLock
func (s *LiveCameras) getRecordState(cameraID int64) *recordState {
	state, ok := s.recordStates[cameraID]
	if !ok {
		state = &recordState{}
		s.recordStates[cameraID] = state
	}
	return state
}

// Called by recorderThread when it receives a message from the monitor.
func (s *LiveCameras) processMonitorMessage(msg *monitor.AnalysisState) {
	s.recordStateLock.Lock()
	defer s.recordStateLock.Unlock()

	state := s.getRecordState(msg.CameraID)

	// The Monitor doesn't send us messages for uninteresting object detections,
	// so if we receive this message when a non-zero object count, then we know
	// we've got something interesting and worth recording.
	if len(msg.Objects) != 0 {
		state.lastDetection = time.Now()

		// Start recording immediately (if applicable), instead of waiting
		// for the periodic wakeup function that scans all cameras.
		if state.recorder == nil && len(s.recordThreadWake) < cap(s.recordThreadWake)/2 {
			s.recordThreadWake <- true
		}
	}
}

func (s *LiveCameras) drainWakeChannel() {
	for {
		select {
		case <-s.recordThreadWake:
		default:
			return
		}
	}
}

// Start or stop recording for all cameras.
func (s *LiveCameras) startStopRecorderForAllCameras() {
	s.drainWakeChannel()
	if s.archive == nil {
		// This happens if the system is not configured correctly.
		// If we have no archive, we can't record.
		return
	}

	systemConfig := s.configDB.GetConfig()
	now := time.Now()

	// Obey the lock hierarchy and take camerasLock first
	s.camerasLock.Lock()
	defer s.camerasLock.Unlock()

	s.recordStateLock.Lock()
	defer s.recordStateLock.Unlock()

	for id, cam := range s.cameraFromID {
		// Some day we might allow individual cameras to override the global recording mode,
		// which is why I introduce this arbitrary variable here.
		cameraRecordingMode := systemConfig.Recording.Mode
		recordBefore := systemConfig.Recording.RecordBeforeEventDuration()
		recordAfter := systemConfig.Recording.RecordAfterEventDuration()

		state := s.getRecordState(id)
		isRecording := state.recorder != nil

		// If reason is not empty, then we must record
		reason := ""

		if cameraRecordingMode == configdb.RecordModeAlways {
			reason = "Continuous recording"
		}

		if reason == "" && cameraRecordingMode == configdb.RecordModeOnDetection && state != nil && state.lastDetection.Add(recordAfter).After(now) {
			reason = "Detection"
		}
		// We don't support "movement" yet, but it's in the enums as configdb.RecordModeOnMovement.

		mustRecord := reason != ""

		if mustRecord && !isRecording {
			// Start recording
			s.log.Infof("Starting recording camera %v (%v): %v", cam.ID(), cam.Name(), reason)
			streamName := filepath.Clean(cam.HighResRecordingStreamName())
			state.recorder = camera.StartVideoRecorder(cam.HighDumper, streamName, s.archive, recordBefore)
		} else if !mustRecord && isRecording {
			// Stop recording
			s.log.Infof("Stop recording camera %v (%v)", cam.ID(), cam.Name())
			state.recorder.Stop()
			state.recorder = nil
		}
	}

	// Stop and remove recorder state for cameras that no longer exist
	for id, state := range s.recordStates {
		if _, ok := s.cameraFromID[id]; !ok {
			if state.recorder != nil {
				state.recorder.Stop()
			}
			delete(s.recordStates, id)
		}
	}
}

func (s *LiveCameras) stopAllRecorders() {
	s.log.Infof("Stopping all recorders")

	s.recordStateLock.Lock()
	defer s.recordStateLock.Unlock()

	for _, state := range s.recordStates {
		if state.recorder != nil {
			state.recorder.Stop()
			state.recorder = nil
		}
	}
}
