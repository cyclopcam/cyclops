package server

import (
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
)

// curl -o img.jpg localhost:8080/camera/latest/0
func (s *Server) httpViewCam(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	idx, _ := strconv.Atoi(params.ByName("index"))
	if idx < 0 || idx >= len(s.Cameras) {
		http.Error(w, "Invalid camera index", 400)
		return
	}
	w.Header().Set("Cache-Control", "max-age: 0")

	contentType := "image/jpeg"
	img := s.Cameras[idx].LatestImage(contentType)
	if img == nil {
		http.Error(w, "Not yet available", 500)
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Write(img)
}
