package server

import (
	"embed"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"

	"github.com/bmharper/cyclops/arc/server/auth"
	"github.com/bmharper/cyclops/pkg/staticfiles"
	"github.com/bmharper/cyclops/pkg/www"
	"github.com/julienschmidt/httprouter"
)

//go:embed www
var staticWWW embed.FS

type authenticatedHandler func(w http.ResponseWriter, r *http.Request, params httprouter.Params, cred *auth.Credentials)

func (s *Server) setupHttpRoutes() error {
	logEveryRequest := false
	router := httprouter.New()

	// protected creates an HTTP handler that is accessible only with authentication
	protected := func(method, route string, handle authenticatedHandler) {
		www.Handle(s.Log, router, method, route, func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
			if logEveryRequest {
				s.Log.Infof("HTTP (protected) %v %v", method, r.URL.Path)
			}
			cred := s.auth.AuthenticateRequest(w, r)
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
	protected("POST", "/api/auth/login", s.httpAuthLogin)
	protected("POST", "/api/auth/setPassword/:userid", s.httpAuthSetPassword)
	protected("POST", "/api/auth/check", s.httpAuthCheck)

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
