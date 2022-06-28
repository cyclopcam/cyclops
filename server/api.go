package server

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// port example: ":8080"
func (s *Server) SetupHTTP(port string) {

	router := httprouter.New()
	router.GET("/", s.httpIndex)
	router.GET("/camera/latest/:index", s.httpViewCam)

	s.Log.Infof("Listening on %v", port)
	http.ListenAndServe(port, router)
}
