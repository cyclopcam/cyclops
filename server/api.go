package server

import (
	"net/http"
	"path/filepath"

	"github.com/bmharper/cyclops/server/staticfiles"
	"github.com/bmharper/cyclops/server/www"
	"github.com/julienschmidt/httprouter"
)

// port example: ":8080"
func (s *Server) SetupHTTP(port string) {

	router := httprouter.New()
	//www.Handle(s.Log, router, "GET", "/", s.httpIndex)
	www.Handle(s.Log, router, "GET", "/camera/info/:index", s.httpCamGetInfo)
	www.Handle(s.Log, router, "GET", "/camera/latestImage/:index", s.httpCamGetLatestImage)
	www.Handle(s.Log, router, "GET", "/camera/recentVideo/:index", s.httpCamGetRecentVideo)
	www.Handle(s.Log, router, "GET", "/camera/stream/:resolution/:index", s.httpCamStreamVideo)

	isImmutable := false
	root, err := filepath.Abs("debug/www")
	if err != nil {
		panic(err)
	}
	static := staticfiles.NewCachedStaticFileServer(root, []string{}, s.Log, isImmutable, nil)
	router.NotFound = static

	s.Log.Infof("Listening on %v", port)
	http.ListenAndServe(port, router)
}
