package server

import (
	"net/http"
	"strings"
	"time"

	"github.com/cyclopcam/cyclops/pkg/pwdhash"
	"github.com/cyclopcam/cyclops/server/configdb"
	"github.com/cyclopcam/www"
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
	identityToken := www.QueryValue(r, "identityToken")
	password := www.QueryValue(r, "password")

	isInitialUser := false
	needRestart := false
	newUser := configdb.User{}

	if identityToken != "" {
		// This path (and the query parameter "identityToken") was created for the initial admin user creation.
		// Note that for the sake of authorization, we don't actually care who's calling us, or how they validate
		// themselves here, because this is the creation of the initial admin user, so we allow absolutely
		// anything in. So the reason we accept an identity token is simply to reduce the back and forth
		// comms between the native app and the webview.
		verified, err := s.configDB.VerifyIdentityAndBindToServer(identityToken)
		if err != nil {
			www.PanicBadRequestf("Failed to verify identity token: %v", err)
		}
		newUser.Email = verified.Email
		newUser.Name = verified.DisplayName
		newUser.ExternalID = verified.ID
		needRestart = true
	} else {
		// This code path could be used to create the initial admin user, or any other user thereafter
		www.ReadJSON(w, r, &newUser, 1024*1024)
		newUser.ID = 0
		newUser.Username = strings.TrimSpace(newUser.Username)
		newUser.UsernameNormalized = configdb.NormalizeUsername(newUser.Username)
		if password != "" {
			newUser.Password = pwdhash.HashPasswordBase64(password)
		}
	}

	creds := s.configDB.GetUser(r, 0)
	if creds == nil || !creds.HasPermission(configdb.UserPermissionAdmin) {
		// Create the initial admin user.
		// This requires no authentication.
		n, err := s.configDB.NumAdminUsers()
		www.Check(err)
		if n != 0 {
			// There is already an admin user, so you can't create the initial user now
			www.PanicForbidden()
		}
		if !s.configDB.IsCallerOnLAN(r) {
			www.PanicForbiddenf("You must be on the LAN to create the initial user")
		}
		s.Log.Infof("Creating initial user %v, %v, %v", newUser.Username, newUser.Email, newUser.ExternalID)
		isInitialUser = true
		if !newUser.HasPermission(configdb.UserPermissionAdmin) {
			// We must force initial creation to be an admin user, otherwise you could somehow
			// screw this up and create a bunch of non-admin users before creating your first
			// admin user... which just doesn't make any sense. This code path is always hit for the
			// case where you're using external ID to create the initial user.
			newUser.Permissions += string(configdb.UserPermissionAdmin)
		}
	}

	if newUser.ExternalID == "" {
		if newUser.Username == "" {
			www.PanicBadRequestf("Either username or external ID must be set")
		}
		if password == "" {
			www.PanicBadRequestf("If not using an external ID, then password must be set")
		}
	}

	www.Check(s.configDB.DB.Create(&newUser).Error)
	s.Log.Infof("Created new user %v, %v, %v, perms:%v", newUser.Username, newUser.Email, newUser.ExternalID, newUser.Permissions)

	if isInitialUser {
		s.configDB.LoginInternal(w, newUser.ID, time.Time{}, configdb.LoginModeCookieAndBearerToken, needRestart)
	} else {
		www.SendOK(w)
	}
}

func (s *Server) httpAuthLogin(w http.ResponseWriter, r *http.Request) {
	s.configDB.Login(w, r)
}
