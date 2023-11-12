package server

import (
	"embed"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"

	"github.com/cyclopcam/cyclops/arc/server/auth"
	"github.com/cyclopcam/cyclops/pkg/staticfiles"
	"github.com/cyclopcam/cyclops/pkg/www"
	"github.com/julienschmidt/httprouter"
)

//go:embed www
var staticWWW embed.FS

type authenticatedHandler func(w http.ResponseWriter, r *http.Request, params httprouter.Params, cred *auth.Credentials)

func (s *Server) setupHttpRoutes() error {
	logEveryRequest := false
	router := httprouter.New()

	// This is useful when debugging, for "curl -u admin:123 ..."
	// The bash script in scripts/arcserver sets this env var, but it's not expected to be set in production
	alwaysAllowBASICAuth := false
	if os.Getenv("ARC_ALWAYS_ALLOW_BASIC_AUTH") == "1" {
		s.Log.Infof("Allowing BASIC authentication for all requests (not just logins)")
		alwaysAllowBASICAuth = true
	}

	// protected creates an HTTP handler that is accessible only with authentication
	protected := func(method, route string, handle authenticatedHandler) {
		www.Handle(s.Log, router, method, route, func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
			if logEveryRequest {
				s.Log.Infof("HTTP (protected) %v %v", method, r.URL.Path)
			}
			allowModes := auth.AuthTypeSessionCookie
			if alwaysAllowBASICAuth {
				allowModes |= auth.AuthTypeUsernamePassword
			}
			cred := s.auth.AuthenticateRequest(w, r, allowModes)
			if cred == nil {
				return
			}
			handle(w, r, params, cred)
		})
	}

	// unprotected creates an HTTP handler that is accessible without authentication
	unprotected := func(method, route string, handle httprouter.Handle) {
		www.Handle(s.Log, router, method, route, func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
			if logEveryRequest {
				s.Log.Infof("HTTP (unprotected) %v %v", method, r.URL.Path)
			}
			handle(w, r, params)
		})
	}

	unprotected("GET", "/api/ping", s.httpPing)
	protected("GET", "/api/constants", s.httpConstants)

	unprotected("POST", "/api/auth/login", s.httpAuthLogin)
	protected("POST", "/api/auth/logout", s.httpAuthLogout)
	protected("POST", "/api/auth/setPassword/:userid", s.httpAuthSetPassword)
	protected("GET", "/api/auth/check", s.httpAuthCheck)

	protected("PUT", "/api/video", s.video.HttpPutVideo)
	protected("GET", "/api/video/:id/thumbnail", s.video.HttpVideoThumbnail)
	protected("GET", "/api/video/:id/video/:res", s.video.HttpGetVideo)
	protected("GET", "/api/videos/list", s.video.HttpListVideos)

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
	}
	router.NotFound = static

	s.httpRouter = router
	return nil
}