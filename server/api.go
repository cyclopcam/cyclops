package server

import (
	"net/http"
	"path/filepath"

	"github.com/bmharper/cyclops/server/configdb"
	"github.com/bmharper/cyclops/server/staticfiles"
	"github.com/bmharper/cyclops/server/www"
	"github.com/julienschmidt/httprouter"
)

type ProtectedHandler func(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User)

func (s *Server) SetupHTTP() {
	router := httprouter.New()

	// protected creates an HTTP handler that only accepts an authenticated user with
	// the given set of permissions.
	// The set of permissions are from configdb.UserPermissions
	protected := func(requiredPerms string, method, route string, handle ProtectedHandler) {
		for _, perm := range requiredPerms {
			if !configdb.IsValidPermission(string(perm)) {
				panic("Invalid permission " + string(perm))
			}
		}
		www.Handle(s.Log, router, method, route, func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
			user := s.configDB.GetUser(r)
			if user == nil {
				www.PanicForbidden()
			}
			for _, perm := range requiredPerms {
				if !user.HasPermission(configdb.UserPermissions(perm)) {
					www.PanicForbidden()
				}
			}
			handle(w, r, params, user)
		})
	}

	// unprotected creates an HTTP handler that is accessible without authentication
	unprotected := func(method, route string, handle httprouter.Handle) {
		www.Handle(s.Log, router, method, route, func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
			handle(w, r, params)
		})
	}

	//www.Handle(s.Log, router, "GET", "/", s.httpIndex)
	protected("v", "GET", "/api/system/info", s.httpSystemGetInfo)
	protected("a", "POST", "/api/system/restart", s.httpSystemRestart)
	unprotected("GET", "/api/system/constants", s.httpSystemConstants)
	protected("v", "GET", "/api/camera/info/:cameraID", s.httpCamGetInfo)
	protected("v", "GET", "/api/camera/latestImage/:cameraID", s.httpCamGetLatestImage)
	protected("v", "GET", "/api/camera/recentVideo/:cameraID", s.httpCamGetRecentVideo)
	protected("v", "GET", "/api/ws/camera/stream/:resolution/:cameraID", s.httpCamStreamVideo)
	protected("a", "GET", "/api/config/cameras", s.httpConfigGetCameras)
	protected("a", "POST", "/api/config/addCamera", s.httpConfigAddCamera)
	protected("a", "GET", "/api/ws/config/testCamera", s.httpConfigTestCamera)
	protected("a", "GET", "/api/config/getVariableDefinitions", s.httpConfigGetVariableDefinitions)
	protected("a", "GET", "/api/config/getVariableValues", s.httpConfigGetVariableValues)
	protected("a", "POST", "/api/config/setVariable/:key", s.httpConfigSetVariable)
	unprotected("POST", "/api/config/scanNetworkForCameras", s.httpConfigScanNetworkForCameras)
	protected("a", "POST", "/api/record/start/:cameraID", s.httpRecordStart)
	protected("a", "POST", "/api/record/stop/:recorderID", s.httpRecordStop)
	protected("v", "GET", "/api/record/getRecordings", s.httpRecordGetRecordings)
	protected("v", "GET", "/api/record/getOntologies", s.httpRecordGetOntologies)
	protected("v", "GET", "/api/record/thumbnail/:id", s.httpRecordGetThumbnail)
	protected("v", "GET", "/api/record/video/:resolution/:id", s.httpRecordGetVideo)
	unprotected("GET", "/api/auth/hasAdmin", s.httpAuthHasAdmin)
	protected("v", "GET", "/api/auth/whoami", s.httpAuthWhoAmi)
	unprotected("POST", "/api/auth/createUser", s.httpAuthCreateUser)
	unprotected("POST", "/api/auth/login", s.httpAuthLogin)

	isImmutable := false
	root, err := filepath.Abs("debug/www")
	if err != nil {
		panic(err)
	}
	static := staticfiles.NewCachedStaticFileServer(root, []string{}, s.Log, isImmutable, nil)
	router.NotFound = static

	s.httpRouter = router
}
