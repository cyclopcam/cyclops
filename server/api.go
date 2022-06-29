package server

import (
	"net/http"

	"github.com/bmharper/cyclops/server/www"
	"github.com/julienschmidt/httprouter"
)

// port example: ":8080"
func (s *Server) SetupHTTP(port string) {

	router := httprouter.New()
	www.Handle(s.Log, router, "GET", "/", s.httpIndex)
	www.Handle(s.Log, router, "GET", "/camera/latestImage/:index", s.httpCamGetLatestImage)
	www.Handle(s.Log, router, "GET", "/camera/recentVideo/:index", s.httpCamGetRecentVideo)

	s.Log.Infof("Listening on %v", port)
	http.ListenAndServe(port, router)
}
