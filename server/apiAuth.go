package server

import (
	"net/http"
	"strings"

	"github.com/cyclopcam/cyclops/pkg/pwdhash"
	"github.com/cyclopcam/cyclops/pkg/www"
	"github.com/cyclopcam/cyclops/server/configdb"
	"github.com/julienschmidt/httprouter"
)

func (s *Server) httpAuthWhoAmi(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
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
	newUser.Username = strings.TrimSpace(newUser.Username)
	newUser.UsernameNormalized = configdb.NormalizeUsername(newUser.Username)
	if newUser.Username == "" {
		www.PanicBadRequestf("Username may not be empty")
	}
	newUser.Password = pwdhash.HashPassword(password)

	//isInitialUser := false
	creds := s.configDB.GetUser(r)
	if creds == nil || !creds.HasPermission(configdb.UserPermissionAdmin) {
		n, err := s.configDB.NumAdminUsers()
		www.Check(err)
		if n != 0 {
			// There is already an admin user, so you can't create the initial user now
			www.PanicForbidden()
		}
		//isInitialUser = true
		s.Log.Infof("Creating initial user %v", newUser.Username)
		if !newUser.HasPermission(configdb.UserPermissionAdmin) {
			// We must force initial creation to be an admin user, otherwise you could somehow
			// screw this up and create a bunch of non-admin users before creating your first
			// admin user... which just doesn't make any sense.
			newUser.Permissions += string(configdb.UserPermissionAdmin)
		}
	}

	www.Check(s.configDB.DB.Create(&newUser).Error)
	s.Log.Infof("Created new user %v, perms:%v", newUser.Username, newUser.Permissions)
	www.SendOK(w)

	//if isInitialUser {
	//	// For initial login, send Cookie and BearerToken, so that caller has flexibility
	//	s.configDB.LoginInternal(w, newUser.ID, time.Time{}, configdb.LoginModeCookieAndBearerToken)
	//} else {
	//	www.SendOK(w)
	//}
}

func (s *Server) httpAuthLogin(w http.ResponseWriter, r *http.Request) {
	s.configDB.Login(w, r)
}
