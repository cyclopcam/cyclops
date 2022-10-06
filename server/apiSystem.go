package server

import (
	"encoding/base64"
	"net/http"
	"os"
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

type pingJSON struct {
	Greeting  string `json:"greeting"`
	Hostname  string `json:"hostname"`
	Time      int64  `json:"time"`
	PublicKey string `json:"publicKey"`
}

func (s *Server) httpSystemPing(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	hostname, _ := os.Hostname()
	ping := &pingJSON{
		Greeting:  "I am Cyclops", // This is used by the LAN scanner on our mobile app to find Cyclops servers, so it's part of our API.
		Hostname:  hostname,       // This is used by the LAN scanner on our mobile app to suggest a name
		Time:      time.Now().Unix(),
		PublicKey: base64.StdEncoding.EncodeToString(s.vpn.PublicKey[:]),
	}
	www.SendJSON(w, ping)
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

type checkVPNJSON struct {
	Error string `json:"error"`
}

// This API is intended to be used at setup time, if the user has somehow failed to start kernelwg.
func (s *Server) httpSystemStartVPN(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	if len(s.vpn.PublicKey) != 0 {
		www.SendJSON(w, &checkVPNJSON{})
	} else {
		if err := s.startVPN(); err != nil {
			www.SendJSON(w, &checkVPNJSON{Error: err.Error()})
		} else {
			www.SendJSON(w, &checkVPNJSON{})
		}
	}
}
