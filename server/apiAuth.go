package server

import (
	"net/http"

	"github.com/bmharper/cyclops/server/configdb"
	"github.com/bmharper/cyclops/server/www"
	"github.com/julienschmidt/httprouter"
)

func (s *Server) httpAuthWhoAmi(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	userID := s.configDB.GetUserID(r)
	if userID == 0 {
		www.PanicForbidden()
	}
	user := configdb.User{}
	www.Check(s.configDB.DB.First(&user, userID).Error)
	www.SendJSON(w, &user)
}

func (s *Server) httpAuthHasAdmin(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	n, err := s.configDB.NumAdminUsers()
	www.Check(err)
	www.SendJSONBool(w, n != 0)
}

func (s *Server) httpAuthCreateUser(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	password := www.RequiredQueryValue(r, "password")
	newUser := configdb.User{}
	www.ReadJSON(w, r, &newUser, 1024*1024)
	if newUser.Username == "" {
		www.PanicBadRequestf("Username may not be empty")
	}
	newUser.Password = configdb.HashPassword(password)

	creds := s.configDB.GetUser(r)
	if creds == nil || !creds.HasPermission(configdb.UserPermissionAdmin) {
		n, err := s.configDB.NumAdminUsers()
		www.Check(err)
		if n != 0 {
			// There is already an admin user, so you can't create the initial user now
			www.PanicForbidden()
		}
	}

	www.Check(s.configDB.DB.Create(&newUser).Error)
	www.SendOK(w)
}

func (s *Server) httpAuthLogin(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	s.configDB.Login(w, r)
}
