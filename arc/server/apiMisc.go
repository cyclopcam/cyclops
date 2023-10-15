package server

import (
	"net/http"
	"strings"
	"time"

	"github.com/cyclopcam/cyclops/arc/server/auth"
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

// This is the first call that the client makes to us.
// If authentication fails, then it returns an appropriate error code.
// If authentication succeeds, then it returns a set of server-specific constants.
func (s *Server) httpConstants(w http.ResponseWriter, r *http.Request, params httprouter.Params, cred *auth.Credentials) {
	type response struct {
		UserID             int64  `json:"userID"`
		PublicVideoBaseUrl string `json:"publicVideoBaseUrl"`
	}

	// publicVideoUrl can be empty, if this server is not serving up publicly accessible training data
	publicVideoUrl, _ := s.storage.URL("")
	if strings.HasSuffix(publicVideoUrl, "/") {
		// Strip the trailing slash
		publicVideoUrl = publicVideoUrl[:len(publicVideoUrl)-1]
	}

	resp := response{
		UserID:             cred.UserID,
		PublicVideoBaseUrl: publicVideoUrl,
	}

	www.SendJSON(w, &resp)
}
