package server

import (
	"net/http"

	"github.com/bmharper/cyclops/server/www"
	"github.com/julienschmidt/httprouter"
)

type systemInfoJSON struct {
	Cameras []*camInfoJSON `json:"cameras"`
}

func (s *Server) httpSystemGetInfo(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	j := systemInfoJSON{}
	for _, cam := range s.Cameras {
		j.Cameras = append(j.Cameras, toCamInfoJSON(cam))
	}
	www.SendJSON(w, &j)
}
