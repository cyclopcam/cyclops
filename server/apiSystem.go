package server

import (
	"net/http"

	"github.com/bmharper/cyclops/server/www"
	"github.com/julienschmidt/httprouter"
)

type systemInfoJSON struct {
	ReadyError string         `json:"readyError,omitempty"` // If system is not yet ready to accept cameras, this will be populated
	Cameras    []*camInfoJSON `json:"cameras"`
}

func (s *Server) httpSystemGetInfo(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	j := systemInfoJSON{
		Cameras: make([]*camInfoJSON, 0),
	}
	if err := s.IsReady(); err != nil {
		j.ReadyError = err.Error()
	}
	for _, cam := range s.Cameras() {
		j.Cameras = append(j.Cameras, toCamInfoJSON(cam))
	}
	www.SendJSON(w, &j)
}

func (s *Server) httpSystemRestart(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	www.SendText(w, "Restarting...")
	// We run the shutdown from a new goroutine so that this HTTP handler can return,
	// which tells the HTTP framework that this request is finished.
	// If we instead run Shutdown from this thread, then the system never sees us return,
	// so it thinks that we're still busy sending a response.
	go func() {
		s.Shutdown(true)
	}()
}
