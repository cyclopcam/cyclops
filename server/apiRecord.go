package server

import (
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"github.com/bmharper/cyclops/server/camera"
	"github.com/bmharper/cyclops/server/configdb"
	"github.com/bmharper/cyclops/server/defs"
	"github.com/bmharper/cyclops/server/eventdb"
	"github.com/bmharper/cyclops/server/log"
	"github.com/bmharper/cyclops/server/www"
	"github.com/julienschmidt/httprouter"
)

// recorderOutMsg is sent by a recorder after receiving the stop message
type recorderOutMsg struct {
	err       error              // If not nil, there was an error
	recording *eventdb.Recording // If successful, this is the recording in the permanent DB
}

// recorder is for facilitating communication with an active recorder
type recorder struct {
	id       int64          // ID of recorder. This is the key in server's map[int64]*recorder
	stop     chan bool      // sent by client to signal that recording must stop
	finished chan bool      // closed by the recorder once it is done
	result   recorderOutMsg // Once 'finished' is closed, result is guaranteed to be populated
}

// recorderFunc runs until recording stops
func (s *Server) recorderFunc(cam *camera.Camera, self *recorder) {
	startAt := time.Now()

	// SYNC-MAX-TRAIN-RECORD-TIME
	// We add 15 seconds grace, on top of the UI limit of 45 seconds.
	maxTime := (45 + 15) * time.Second

	logger := log.NewPrefixLogger(s.Log, fmt.Sprintf("Recorder %v", self.id))

	defer close(self.finished)

outer:
	for {
		select {
		case <-self.stop:
			logger.Infof("Received stop message")
			break outer
		case <-time.After(time.Second):
			if time.Now().Sub(startAt) > maxTime {
				logger.Infof("Timeout")
				break outer
			}
		}
	}

	if s.IsShutdown() {
		logger.Infof("Aborting due to system shutdown")
		self.result.err = errors.New("System shutdown")
		return
	}

	// save recording
	raw, err := cam.LowDumper.ExtractRawBuffer(camera.ExtractMethodDrain, time.Now().Sub(startAt))
	if err != nil {
		msg := fmt.Errorf("Failed to extract raw buffer: %v", err)
		logger.Errorf("%v", msg)
		self.result.err = msg
		return
	}

	recording, err := s.permanentEvents.Save(defs.ResLD, raw)
	if err != nil {
		msg := fmt.Errorf("Failed to save recording: %v", err)
		logger.Errorf("%v", msg)
		self.result.err = msg
		return
	}

	// Make sure we get removed *eventually*
	// In the normal case, we'll be removed sooner, when somebody calls stopRecorder()
	s.deleteRecorderAfterDelay(self.id, 15*time.Minute)

	self.result.recording = recording
}

func (s *Server) startRecorder(cam *camera.Camera) int64 {
	s.recordersLock.Lock()
	id := s.nextRecorderID
	s.nextRecorderID++
	rec := &recorder{
		id: id,

		// buffer size 10 in case we receive multiple requests to stop the recording (eg client needs to reconnect).
		// If the buffer size was only 1, then the 2nd client who tried to send to stop would block forever, because
		// the recorder has already stopped.
		stop: make(chan bool, 10),

		// We never send on finished. We simply close the channel when done.
		finished: make(chan bool),
	}
	s.recorders[id] = rec
	s.recordersLock.Unlock()

	s.Log.Infof("Recording %v started on camera %v", id, cam.Name)

	go s.recorderFunc(cam, rec)

	return id
}

func (s *Server) stopRecorder(recorderID int64) *recorderOutMsg {
	s.recordersLock.Lock()
	recorder := s.recorders[recorderID]
	s.recordersLock.Unlock()

	if recorder == nil {
		return &recorderOutMsg{
			err: fmt.Errorf("Recorder %v not found", recorderID),
		}
	}
	recorder.stop <- true
	<-recorder.finished // wait for channel to close

	// Give clients 1 minute to read the results of the record operation.
	s.deleteRecorderAfterDelay(recorderID, time.Minute)

	return &recorder.result
}

func (s *Server) deleteRecorderAfterDelay(recorderID int64, delay time.Duration) {
	go func() {
		time.Sleep(delay)
		if s.IsShutdown() {
			return
		}
		s.recordersLock.Lock()
		delete(s.recorders, recorderID)
		s.recordersLock.Unlock()
	}()
}

func (s *Server) httpRecordStart(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	cam := s.getCameraFromIDOrPanic(params.ByName("cameraID"))
	id := s.startRecorder(cam)
	www.SendID(w, id)
}

func (s *Server) httpRecordStop(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	recorderID := www.ParseID(params.ByName("recorderID"))
	result := s.stopRecorder(recorderID)
	www.Check(result.err)
	www.SendJSON(w, result.recording)
}

func (s *Server) httpRecordGetRecordings(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	err, recordings := s.permanentEvents.GetRecordings()
	www.Check(err)
	www.SendJSON(w, recordings)
}

func (s *Server) httpRecordGetOntologies(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	err, ontologies := s.permanentEvents.GetOntologies()
	www.Check(err)
	www.SendJSON(w, ontologies)
}

func (s *Server) httpRecordGetThumbnail(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	err, recording := s.permanentEvents.GetRecording(www.ParseID(params.ByName("id")))
	www.Check(err)
	fullpath := filepath.Join(s.permanentEvents.Root, recording.ThumbnailFilename())
	www.SendFile(w, fullpath, "")
}

func (s *Server) httpRecordGetVideo(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	res := parseResolutionOrPanic(params.ByName("resolution"))
	err, recording := s.permanentEvents.GetRecording(www.ParseID(params.ByName("id")))
	www.Check(err)
	fullpath := filepath.Join(s.permanentEvents.Root, recording.VideoFilename(res))
	www.SendFile(w, fullpath, recording.VideoContentType(res))
}
