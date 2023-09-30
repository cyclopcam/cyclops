package server

import (
	"net/http"
	"time"

	"github.com/cyclopcam/cyclops/pkg/www"
	"github.com/julienschmidt/httprouter"
)

func (s *Server) httpPing(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	type pingJSON struct {
		Time int64 `json:"time"`
	}
	ping := &pingJSON{
		Time: time.Now().Unix(),
	}
	www.SendJSON(w, ping)
}
