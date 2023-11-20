package server

import (
	"net/http"
	"strings"

	"github.com/cyclopcam/cyclops/arc/server/auth"
	"github.com/cyclopcam/cyclops/pkg/www"
	"github.com/julienschmidt/httprouter"
)

func (s *Server) httpAuthLogin(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	s.auth.Login(w, r)
}

func (s *Server) httpAuthLogout(w http.ResponseWriter, r *http.Request, params httprouter.Params, cred *auth.Credentials) {
	s.auth.Logout(w, r)
}

func (s *Server) httpAuthSetPassword(w http.ResponseWriter, r *http.Request, params httprouter.Params, cred *auth.Credentials) {
	userID := www.ParseID(params.ByName("userid"))
	if userID == 0 {
		www.PanicBadRequestf("Invalid user ID")
	}
	password := strings.TrimSpace(www.QueryValue(r, "password"))
	if len(password) < 8 {
		www.PanicBadRequestf("Password must be at least 8 characters")
	}
	if userID != cred.UserID {
		www.PanicBadRequestf("You can only set your own password")
	}
	www.Check(s.auth.SetPassword(userID, www.QueryValue(r, "password")))

	// Erase all login sessions except for the one that made this request
	s.auth.EraseAllSessionsExceptCallingSession(cred)

	www.SendOK(w)
}

func (s *Server) httpAuthCheck(w http.ResponseWriter, r *http.Request, params httprouter.Params, cred *auth.Credentials) {
	type response struct {
		UserID int64 `json:"userID"`
	}
	www.SendJSON(w, response{UserID: cred.UserID})
}

func (s *Server) httpAuthCreateApiKey(w http.ResponseWriter, r *http.Request, params httprouter.Params, cred *auth.Credentials) {
	s.auth.CreateKey(w, r, cred)
}
