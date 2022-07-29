package server

import (
	"net/http"
	"time"

	"github.com/bmharper/cyclops/server/camera"
	"github.com/bmharper/cyclops/server/configdb"
	"github.com/bmharper/cyclops/server/www"
	"github.com/julienschmidt/httprouter"
)

// recorderFunc runs until recording stops
func (s *Server) recorderFunc(cam *camera.Camera, stopChan chan bool) {
	dumper := camera.NewVideoDumpReader(200 * 1024 * 1024)
	stream := cam.LowStream
	stream.ConnectSinkAndRun(dumper)
	defer stream.RemoveSink(dumper)

outer:
	for {
		select {
		case <-stopChan:
			s.Log.Infof("Recorder received stop message")
			break outer
		}
	}

	// signal that there is no recorder active
	s.recorderStartStopLock.Lock()
	if s.recorderStop == stopChan {
		s.recorderStop = nil
	}
	s.recorderStartStopLock.Unlock()

	if s.IsShutdown() {
		return
	}

	// save recording
	raw, err := dumper.ExtractRawBuffer(camera.ExtractMethodDrain, 365*24*time.Hour)
	if err != nil {
		// TODO: show error to user
		s.Log.Errorf("Failed to extract recording raw buffer: %v", err)
		return
	}

	s.permanentEvents.Save(raw)
	if err != nil {
		// TODO: show error to user
		s.Log.Errorf("Failed to save recording: %v", err)
		return
	}
}

func (s *Server) startRecorder(cam *camera.Camera) {
	s.Log.Infof("Recording started on camera %v", cam.Name)

	s.recorderStartStopLock.Lock()
	defer s.recorderStartStopLock.Unlock()

	// stop any existing recording
	if s.recorderStop != nil {
		s.recorderStop <- true
	}

	stopChan := make(chan bool, 1)
	s.recorderStop = stopChan
	go s.recorderFunc(cam, stopChan)
}

func (s *Server) stopRecorder() {
	s.Log.Infof("Recording stopped")

	s.recorderStartStopLock.Lock()
	defer s.recorderStartStopLock.Unlock()
	if s.recorderStop != nil {
		s.recorderStop <- true
	}
}

func (s *Server) httpRecordStart(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	cam := s.getCameraFromIDOrPanic(params.ByName("cameraID"))
	s.startRecorder(cam)
	www.SendOK(w)
}

func (s *Server) httpRecordStop(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	s.stopRecorder()
	www.SendOK(w)
}
