package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/bmharper/cyclops/pkg/www"
	"github.com/bmharper/cyclops/server/camera"
	"github.com/bmharper/cyclops/server/configdb"
	"github.com/julienschmidt/httprouter"
)

// SYNC-SYSTEM-INFO-JSON
type systemInfoJSON struct {
	ReadyError string         `json:"readyError,omitempty"` // If system is not yet ready to accept cameras, this will be populated
	Cameras    []*camInfoJSON `json:"cameras"`
}

// If this gets too bloated, then we can split it up
type constantsJSON struct {
	CameraModels []string `json:"cameraModels"`
}

func (s *Server) httpSystemPing(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	www.SendText(w, fmt.Sprintf("%v", time.Now().Unix()))
}

func (s *Server) httpSystemGetInfo(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
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

func (s *Server) httpSystemRestart(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	www.SendText(w, "Restarting...")
	// We run the shutdown from a new goroutine so that this HTTP handler can return,
	// which tells the HTTP framework that this request is finished.
	// If we instead run Shutdown from this thread, then the system never sees us return,
	// so it thinks that we're still busy sending a response.
	go func() {
		s.Shutdown(true)
	}()
}

func (s *Server) httpSystemConstants(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	cams := []string{}
	for _, m := range camera.AllCameraModels {
		cams = append(cams, string(m))
	}
	c := &constantsJSON{
		CameraModels: cams,
	}
	www.SendJSON(w, c)
}
