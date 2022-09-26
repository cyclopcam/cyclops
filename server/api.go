package server

import (
	"net/http"
	"path/filepath"
	"strings"

	"github.com/bmharper/cyclops/pkg/staticfiles"
	"github.com/bmharper/cyclops/pkg/www"
	"github.com/bmharper/cyclops/server/configdb"
	"github.com/julienschmidt/httprouter"
)

type ProtectedHandler func(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User)

func (s *Server) SetupHTTP() error {
	router := httprouter.New()

	// protected creates an HTTP handler that only accepts an authenticated user with
	// the given set of permissions.
	// The set of permissions are from configdb.UserPermissions
	protected := func(requiredPerms string, methods, route string, handle ProtectedHandler) {
		for _, perm := range requiredPerms {
			if !configdb.IsValidPermission(string(perm)) {
				panic("Invalid permission " + string(perm))
			}
		}
		handleWrapper := func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
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
		}
		for _, method := range strings.Split(methods, "|") {
			www.Handle(s.Log, router, method, route, handleWrapper)
		}
	}

	// unprotected creates an HTTP handler that is accessible without authentication
	unprotected := func(method, route string, handle httprouter.Handle) {
		www.Handle(s.Log, router, method, route, func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
			handle(w, r, params)
		})
	}

	unprotected("GET", "/api/ping", s.httpSystemPing)
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
	protected("v", "GET", "/api/record/count", s.httpRecordCount)
	protected("v", "POST", "/api/record/delete/:id", s.httpRecordDeleteRecording)
	protected("v", "GET", "/api/record/getOntologies", s.httpRecordGetOntologies)
	protected("v", "POST", "/api/record/setOntology", s.httpRecordSetOntology)
	protected("v", "GET", "/api/record/thumbnail/:id", s.httpRecordGetThumbnail)
	protected("v", "GET|HEAD", "/api/record/video/:resolution/:id", s.httpRecordGetVideo)
	protected("v", "POST", "/api/record/video/:resolution/:id", s.httpRecordGetVideo)
	protected("v", "POST", "/api/record/background/create", s.httpRecordBackgroundCreate)
	unprotected("GET", "/api/auth/hasAdmin", s.httpAuthHasAdmin)
	protected("v", "GET", "/api/auth/whoami", s.httpAuthWhoAmi)
	unprotected("POST", "/api/auth/createUser", s.httpAuthCreateUser)
	unprotected("POST", "/api/auth/login", s.httpAuthLogin)

	isImmutable := true
	relRoot := "www/dist"
	absRoot, err := filepath.Abs(relRoot)
	if err != nil {
		s.Log.Warnf("Failed to resolve static file directory %v: %v. Run 'npm run build' in 'www' to build static files. If you're using 'npm run dev', then you can ignore this warning.", relRoot, err)
	}
	s.Log.Infof("Serving static files from %v", absRoot)
	static, err := staticfiles.NewCachedStaticFileServer(absRoot, []string{"/api/"}, s.Log, isImmutable, nil)
	if err != nil {
		s.Log.Warnf("Error in static files ('%v' resolved to '%v'), error %v. Run 'npm run build' in 'www' to build static files. If you're using 'npm run dev', then you can ignore this warning.", relRoot, absRoot, err)
	}
	router.NotFound = static

	s.httpRouter = router
	return nil
}
