package server

import (
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"github.com/cyclopcam/cyclops/pkg/dbh"
	"github.com/cyclopcam/cyclops/pkg/log"
	"github.com/cyclopcam/cyclops/pkg/www"
	"github.com/cyclopcam/cyclops/server/camera"
	"github.com/cyclopcam/cyclops/server/configdb"
	"github.com/cyclopcam/cyclops/server/defs"
	"github.com/cyclopcam/cyclops/server/eventdb"
	"github.com/cyclopcam/cyclops/server/videox"
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
		case <-s.ShutdownStarted:
			logger.Infof("Aborting due to system shutdown")
			self.result.err = errors.New("System shutdown")
			return
		}
	}

	// save recording
	raw, err := cam.LowDumper.ExtractRawBuffer(camera.ExtractMethodDrain, time.Now().Sub(startAt))
	if err != nil {
		msg := fmt.Errorf("Failed to extract raw buffer: %v", err)
		logger.Errorf("%v", msg)
		self.result.err = msg
		return
	}

	recording, err := s.permanentEvents.Save(defs.ResLD, eventdb.RecordingOriginExplicit, cam.ID(), startAt, raw)
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

	s.Log.Infof("Recording %v started on camera %v", id, cam.Name())

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
		select {
		case <-time.After(delay):
		case <-s.ShutdownStarted:
			return
		}
		s.recordersLock.Lock()
		delete(s.recorders, recorderID)
		s.recordersLock.Unlock()
	}()
}

// This is just a hack to start the background recorder.
// Built to send some footage to a friend.
func (s *Server) httpRecordQuick(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	seconds := www.RequiredQueryInt(r, "seconds")
	res, err := defs.ParseResolution(www.RequiredQueryValue(r, "resolution"))
	www.Check(err)
	start := time.Now()
	ins := configdb.RecordInstruction{
		StartAt:    dbh.MakeIntTime(start),
		FinishAt:   dbh.MakeIntTime(start.Add(time.Duration(seconds) * time.Second)),
		Resolution: string(res),
	}
	www.Check(s.configDB.DB.Create(&ins).Error)
	www.SendOK(w)
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
	if id := www.QueryInt64(r, "id"); id != 0 {
		recording, err := s.permanentEvents.GetRecording(id)
		www.Check(err)
		www.SendJSON(w, []*eventdb.Recording{recording})
	} else {
		recordings, err := s.permanentEvents.GetRecordings()
		www.Check(err)
		www.SendJSON(w, recordings)
	}
}

func (s *Server) httpRecordSetLabels(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	rec := eventdb.Recording{}
	www.ReadJSON(w, r, &rec, 1024*1024)
	www.Check(s.permanentEvents.SetRecordingLabels(&rec))
	www.SendOK(w)
}

func (s *Server) httpRecordCount(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	count, err := s.permanentEvents.Count()
	www.Check(err)
	www.SendInt64(w, count)
}

func (s *Server) httpRecordDeleteRecording(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	id := www.ParseID(params.ByName("id"))
	www.Check(s.permanentEvents.DeleteRecordingComplete(id))
	www.SendOK(w)
}

func (s *Server) httpRecordGetOntologies(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	ontologies, err := s.permanentEvents.GetOntologies()
	www.Check(err)
	www.SendJSON(w, ontologies)
}

func (s *Server) httpRecordGetLatestOntology(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	latest, err := s.permanentEvents.GetLatestOntology()
	www.Check(err)
	www.SendJSON(w, latest)
}

func (s *Server) httpRecordSetOntology(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	spec := eventdb.OntologyDefinition{}
	www.ReadJSON(w, r, &spec, 1024*1024)
	// CreateOntology will create a new ontology, because we can't alter ontologies that are
	// already referenced by recordings.
	// One thing we might want to do is to modify an ontology in-place, if the following conditions are met:
	// 1. The most recent ontology in the DB is a subset of the new ontology
	id, err := s.permanentEvents.CreateOntology(&spec)
	www.Check(err)
	// Prune unused ontologies, so that we don't end up with unnecessary records
	www.Check(s.permanentEvents.PruneUnusedOntologies([]int64{id}))
	www.SendOK(w)
}

func (s *Server) httpRecordGetThumbnail(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	recording, err := s.permanentEvents.GetRecording(www.ParseID(params.ByName("id")))
	www.Check(err)
	fullpath := filepath.Join(s.permanentEvents.Root, recording.ThumbnailFilename())
	www.SendFile(w, r, fullpath, "")
}

func (s *Server) httpRecordGetVideo(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	res := parseResolutionOrPanic(params.ByName("resolution"))
	seekable := www.QueryValue(r, "seekable") == "1"
	recording, err := s.permanentEvents.GetRecording(www.ParseID(params.ByName("id")))
	www.Check(err)
	fullpath := filepath.Join(s.permanentEvents.Root, recording.VideoFilename(res))
	if seekable {
		seekable, exists := s.TempFiles.GetNamed(fmt.Sprintf("seekable-perm-%v-%v.mp4", recording.ID, res))
		if !exists {
			s.Log.Infof("Transcoding %v into a seekable format", fullpath)
			www.Check(videox.TranscodeSeekable(fullpath, seekable))
			s.Log.Infof("Transcoding %v done", fullpath)
		}
		fullpath = seekable
	}
	www.SendFile(w, r, fullpath, recording.VideoContentType(res))
}

func (s *Server) httpRecordBackgroundCreate(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	ins := configdb.RecordInstruction{}
	www.ReadJSON(w, r, &ins, 1024*1024)
	if ins.StartAt.IsZero() || ins.FinishAt.IsZero() {
		www.PanicBadRequestf("Both StartAt and FinishAt must be defined")
	}
	if ins.StartAt.Get().After(ins.FinishAt.Get()) {
		www.PanicBadRequestf("Start time %v is after Finish time %v", ins.StartAt.Get(), ins.FinishAt.Get())
	}
	if ins.FinishAt.Get().Before(time.Now()) {
		www.PanicBadRequestf("Finish time %v is in the past", ins.FinishAt.Get())
	}
	www.Check(s.configDB.DB.Create(&ins).Error)
	www.SendOK(w)
}
