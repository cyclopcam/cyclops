package server

import (
	"errors"
	"net/http"

	"github.com/cyclopcam/cyclops/server/configdb"
	"github.com/cyclopcam/cyclops/server/defs"
	"github.com/cyclopcam/cyclops/server/eventdb"
	"github.com/cyclopcam/www"
	"github.com/julienschmidt/httprouter"
	"gorm.io/gorm"
)

func (s *Server) getEventOrPanic(id int64) *eventdb.Event {
	n := eventdb.Event{}
	if err := s.eventDB.DB.First(&n, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			www.PanicNotFound()
		}
		www.Check(err)
	}
	return &n
}

func (s *Server) httpEventsGet(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	id := www.ParseID(params.ByName("id"))
	n := s.getEventOrPanic(id)
	www.SendJSON(w, &n)
}

func (s *Server) httpEventsGetImage(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	id := www.ParseID(params.ByName("id"))
	n := s.getEventOrPanic(id)
	if n.EventType != eventdb.EventTypeAlarm {
		www.PanicBadRequest()
	}
	cam := s.LiveCameras.CameraFromID(n.Detail.Data.Alarm.CameraID)
	if cam == nil {
		www.PanicBadRequestf("Invalid camera ID '%v'", n.Detail.Data.Alarm.CameraID)
	}
	s.httpGetCameraImage(w, cam, defs.ResLD, n.Time.Get(), "", 95, true)
}
