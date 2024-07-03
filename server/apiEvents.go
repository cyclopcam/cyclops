package server

import (
	"net/http"

	"github.com/cyclopcam/cyclops/pkg/www"
	"github.com/cyclopcam/cyclops/server/configdb"
	"github.com/julienschmidt/httprouter"
)

func (s *Server) httpEventsGetTiles(w http.ResponseWriter, r *http.Request, _ httprouter.Params, user *configdb.User) {
	cameraID := www.RequiredQueryValue(r, "camera")
	level := www.RequiredQueryInt(r, "level")
	startIdx := www.RequiredQueryInt(r, "startIdx")
	endIdx := www.RequiredQueryInt(r, "endIdx")
	cam := s.getCameraFromIDOrPanic(cameraID)
	tiles, err := s.videoDB.ReadEventTiles(cam.LongLivedName(), uint32(level), uint32(startIdx), uint32(endIdx))
	www.Check(err)
	www.SendJSONOpt(w, tiles, false)
}
