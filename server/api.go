package server

import (
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"embed"

	"github.com/cyclopcam/cyclops/pkg/staticfiles"
	"github.com/cyclopcam/cyclops/server/configdb"
	"github.com/cyclopcam/www"
	"github.com/go-chi/httprate"
	"github.com/julienschmidt/httprouter"
)

//go:embed www
var staticWWW embed.FS

type ProtectedHandler func(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User)

func (s *Server) SetupHTTP() error {
	router := httprouter.New()

	logEveryRequest := false

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
			if logEveryRequest {
				s.Log.Infof("HTTP (protected) %v", r.URL.Path)
			}
			//w.Header().Set("Access-Control-Allow-Origin", "https://appassets.androidplatform.net")
			//w.Header().Set("Access-Control-Allow-Origin", "*")
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
			if logEveryRequest {
				s.Log.Infof("HTTP (unprotected) %v %v", method, r.URL.Path)
			}
			//w.Header().Set("Access-Control-Allow-Origin", "https://appassets.androidplatform.net")
			//w.Header().Set("Access-Control-Allow-Origin", "*")
			handle(w, r, params)
		})
	}

	// unprotectedLimited creates an unprotected HTTP handler that is rate limited
	unprotectedLimited := func(method, route string, handle func(w http.ResponseWriter, r *http.Request), requestLimit int, windowLength time.Duration) {
		// We don't need httprate.KeyByEndpoint, because we create a unique rate limiter for each endpoint.
		// That's not really the intended use case, but I only need it for a few endpoints, so I think this is OK.
		// The data structures in httprate looks decent, so we're hopefully not bloating up too much here.
		limited := httprate.Limit(requestLimit, windowLength, httprate.WithKeyFuncs(httprate.KeyByIP))

		www.Handle(s.Log, router, method, route, func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
			if logEveryRequest {
				s.Log.Infof("HTTP (rate limited) %v %v", method, r.URL.Path)
			}
			limited(http.HandlerFunc(handle)).ServeHTTP(w, r)
		})
	}

	unprotected("GET", "/api/ping", s.httpSystemPing)
	unprotected("GET", "/api/keys", s.httpSystemKeys)
	protected("v", "GET", "/api/system/info", s.httpSystemGetInfo)
	protected("a", "POST", "/api/system/restart", s.httpSystemRestart)
	//unprotected("POST", "/api/system/startVPN", s.httpSystemStartVPN) // disabling this because I no longer think it's a good part of user flow
	unprotected("GET", "/api/system/constants", s.httpSystemConstants)
	protected("v", "GET", "/api/camera/info/:cameraID", s.httpCamGetInfo)
	protected("v", "GET", "/api/camera/latestImage/:cameraID", s.httpCamGetLatestImage)
	protected("v", "GET", "/api/camera/recentVideo/:cameraID", s.httpCamGetRecentVideo)
	protected("v", "GET", "/api/camera/image/:cameraID/:resolution/:time", s.httpCamGetImage)
	protected("v", "GET", "/api/camera/frames/:cameraID/:resolution/:startTime/:endTime", s.httpCamGetFrames)
	protected("a", "POST", "/api/camera/debug/saveClip/:cameraID/:startTime/:endTime", s.httpCamDebugSaveClip)
	protected("v", "GET", "/api/camera/debug/stats", s.httpCamDebugStats)
	protected("v", "GET", "/api/camera/debug/frameTimes/:cameraID/:resolution", s.httpCamDebugFrameTimes)
	protected("v", "GET", "/api/ws/camera/stream/:cameraID/:resolution", s.httpCamStreamVideo)
	protected("a", "GET", "/api/config/camera/:cameraID", s.httpConfigGetCamera)
	protected("a", "GET", "/api/config/cameras", s.httpConfigGetCameras)
	protected("a", "POST", "/api/config/addCamera", s.httpConfigAddCamera)
	protected("a", "POST", "/api/config/changeCamera", s.httpConfigChangeCamera)
	protected("a", "POST", "/api/config/removeCamera/:cameraID", s.httpConfigRemoveCamera)
	protected("a", "GET", "/api/ws/config/testCamera", s.httpConfigTestCamera)
	protected("a", "GET", "/api/config/settings", s.httpConfigGetSettings)
	protected("a", "POST", "/api/config/settings", s.httpConfigSetSettings)
	protected("a", "POST", "/api/config/scanNetworkForCameras", s.httpConfigScanNetworkForCameras)
	protected("a", "GET", "/api/config/measureStorageSpace", s.httpConfigMeasureStorageSpace)
	protected("v", "GET", "/api/events/tiles", s.httpEventsGetTiles)
	protected("v", "GET", "/api/events/details", s.httpEventsGetDetails)
	unprotected("GET", "/api/auth/hasAdmin", s.httpAuthHasAdmin)
	protected("v", "GET", "/api/auth/whoami", s.httpAuthWhoAmi)
	unprotected("POST", "/api/auth/createUser", s.httpAuthCreateUser)
	unprotectedLimited("POST", "/api/auth/login", s.httpAuthLogin, 10, 10*time.Second)

	//static, err := staticfiles.NewCachedStaticFileServer(absRoot, []string{"/api/"}, s.Log, isImmutable, nil)
	isImmutable := true
	var fsys fs.FS
	fsysRoot := "www"
	fsys = staticWWW
	if s.HotReloadWWW {
		relRoot := "server/www"
		absRoot, err := filepath.Abs(relRoot)
		if err != nil {
			s.Log.Errorf("Failed to resolve static file directory %v: %v. Run 'npm run build' in 'www' to build static files.", relRoot, err)
			return errors.New("Failed to resolve static file directory for hot reload")
		}
		s.Log.Infof("Serving static files from %v, with hot reload", absRoot)
		fsys = os.DirFS(absRoot)
		fsysRoot = ""
		isImmutable = false
	}

	static, err := staticfiles.NewCachedStaticFileServer(fsys, fsysRoot, []string{"/api/"}, s.Log, isImmutable, nil)
	if err != nil {
		s.Log.Warnf("Error in static files: %v. Run 'npm run build' in 'www' to build static files. If you're using 'npm run dev', then you can ignore this warning.", err)
		//s.Log.Warnf("Error in static files ('%v' resolved to '%v'), error %v. Run 'npm run build' in 'www' to build static files. If you're using 'npm run dev', then you can ignore this warning.", relRoot, absRoot, err)
	}
	router.NotFound = static

	s.httpRouter = router
	return nil
}
