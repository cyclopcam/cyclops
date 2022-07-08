package server

import (
	"path/filepath"

	"github.com/bmharper/cyclops/server/staticfiles"
	"github.com/bmharper/cyclops/server/www"
	"github.com/julienschmidt/httprouter"
)

func (s *Server) SetupHTTP() {
	router := httprouter.New()

	//www.Handle(s.Log, router, "GET", "/", s.httpIndex)
	www.Handle(s.Log, router, "GET", "/api/system/info", s.httpSystemGetInfo)
	www.Handle(s.Log, router, "POST", "/api/system/restart", s.httpSystemRestart)
	www.Handle(s.Log, router, "GET", "/api/camera/info/:id", s.httpCamGetInfo)
	www.Handle(s.Log, router, "GET", "/api/camera/latestImage/:id", s.httpCamGetLatestImage)
	www.Handle(s.Log, router, "GET", "/api/camera/recentVideo/:id", s.httpCamGetRecentVideo)
	www.Handle(s.Log, router, "GET", "/api/ws/camera/stream/:resolution/:id", s.httpCamStreamVideo)
	www.Handle(s.Log, router, "POST", "/api/config/addCamera", s.httpConfigAddCamera)
	www.Handle(s.Log, router, "POST", "/api/config/setVariable/:key", s.httpConfigSetVariable)

	isImmutable := false
	root, err := filepath.Abs("debug/www")
	if err != nil {
		panic(err)
	}
	static := staticfiles.NewCachedStaticFileServer(root, []string{}, s.Log, isImmutable, nil)
	router.NotFound = static

	s.httpRouter = router
}
