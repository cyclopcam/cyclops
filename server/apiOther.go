package server

import (
	"fmt"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func (s *Server) httpIndex(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	fmt.Fprint(w, "Welcome!\n")
}
