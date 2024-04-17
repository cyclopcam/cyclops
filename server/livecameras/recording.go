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

	lastStartStop := time.Now()

	keepRunning := true
	for keepRunning {
		// Every 2 seconds, check if we need to start/stop any cameras
		if time.Now().Sub(lastStartStop) >= 2*time.Second {
			s.startStopRecorderForAllCameras()
			lastStartStop = time.Now()
		}
		select {
		case <-s.shutdown:
			keepRunning = false
		case mm := <-s.allCameraMonitorMsg:
			s.processMonitorMessage(mm)
		case <-time.After(time.Second * 2):
		case <-s.recordThreadWake:
		}
	}

	s.stopAllRecorders()

	s.log.Infof("Recorder thread shutdown complete")

	close(s.recordThreadShutdown)
}

// Find or create the state for this camera.
// Assumes that you've already acquired recordStateLock
func (s *LiveCameras) getRecordState(cameraID int64) *cameraRecordState {
	state, ok := s.recordStates[cameraID]
	if !ok {
		state = &cameraRecordState{}
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
	// so if we receive this message with a non-zero object count, then we know
	// we've got something interesting and worth recording.
	if len(msg.Objects) != 0 {
		state.lastDetection = time.Now()

		// Start recording immediately (if applicable), instead of waiting
		// for the periodic wakeup function that scans all cameras.
		if !state.isRecording() && len(s.recordThreadWake) < cap(s.recordThreadWake)/2 {
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

	// It's useful to activate this log message to verify the sanity of the wakeup system.
	// We expect to see this log message appear once every 2 seconds.
	//s.log.Warnf("Camera recording mode: %v", systemConfig.Recording.Mode)

	for id, cam := range s.cameraFromID {
		// Some day we might allow individual cameras to override the global recording mode,
		// which is why I introduce this arbitrary variable here.
		cameraRecordingMode := systemConfig.Recording.Mode
		recordBefore := systemConfig.Recording.RecordBeforeEventDuration()
		recordAfter := systemConfig.Recording.RecordAfterEventDuration()

		state := s.getRecordState(id)

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

		if mustRecord && !state.isRecording() {
			// Start recording
			s.log.Infof("Starting recording on %v (%v): %v", cam.ID(), cam.Name(), reason)
			state.recorderHD = camera.StartVideoRecorder(cam.HighDumper, filepath.Clean(cam.HighResRecordingStreamName()), s.archive, recordBefore)
			state.recorderLD = camera.StartVideoRecorder(cam.LowDumper, filepath.Clean(cam.LowResRecordingStreamName()), s.archive, recordBefore)
		} else if !mustRecord && state.isRecording() {
			// Stop recording
			s.log.Infof("Motion/Detection is gone - stopping recording %v (%v)", cam.ID(), cam.Name())
			s.stopRecorder(state)
		}
	}

	// Stop and remove recorder state for cameras that no longer exist
	for id, state := range s.recordStates {
		if _, ok := s.cameraFromID[id]; !ok {
			s.log.Infof("Stopping recording (if any) of camera %v because it no longer exists", id)
			s.stopRecorder(state)
			delete(s.recordStates, id)
		}
	}
}

func (s *LiveCameras) stopAllRecorders() {
	s.log.Infof("Stopping all recorders")

	s.recordStateLock.Lock()
	defer s.recordStateLock.Unlock()

	for _, state := range s.recordStates {
		s.stopRecorder(state)
	}
}

func (s *LiveCameras) stopRecorder(state *cameraRecordState) {
	if state.recorderHD != nil {
		state.recorderHD.Stop()
		state.recorderHD = nil
	}
	if state.recorderLD != nil {
		state.recorderLD.Stop()
		state.recorderLD = nil
	}
}
