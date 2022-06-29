package server

import (
	"net/http"
	"strconv"
	"time"

	"github.com/bmharper/cyclops/server/camera"
	"github.com/bmharper/cyclops/server/www"
	"github.com/julienschmidt/httprouter"
)

func (s *Server) getCameraIndexOrPanic(idxStr string) int {
	idx, _ := strconv.Atoi(idxStr)
	if idx < 0 || idx >= len(s.Cameras) {
		www.PanicBadRequestf("Invalid camera index %v. Valid values are %v .. %v", idx, 0, len(s.Cameras)-1)
	}
	return idx
}

// Fetch a low res JPG of the camera's last image.
// Example: curl -o img.jpg localhost:8080/camera/latestImage/0
func (s *Server) httpCamGetLatestImage(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	idx := s.getCameraIndexOrPanic(params.ByName("index"))

	www.CacheNever(w)

	contentType := "image/jpeg"
	img := s.Cameras[idx].LatestImage(contentType)
	if img == nil {
		www.PanicBadRequestf("No image available yet")
	}

	w.Header().Set("Content-Type", contentType)
	w.Write(img)
}

// Fetch a high res MP4 of the camera's recent footage
// default duration is 5 seconds
// Example: curl -o recent.mp4 localhost:8080/camera/recentVideo/0?duration=15s
func (s *Server) httpCamGetRecentVideo(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	idx := s.getCameraIndexOrPanic(params.ByName("index"))
	duration, _ := time.ParseDuration(r.URL.Query().Get("duration"))
	if duration <= 0 {
		duration = 5 * time.Second
	}

	www.CacheNever(w)

	contentType := "video/mp4"
	fn := s.TempFiles.Get()
	raw, err := s.Cameras[idx].ExtractHighRes(camera.ExtractMethodClone, duration)
	www.Check(err)
	www.Check(raw.SaveToMP4(fn))

	www.SendTempFile(w, fn, contentType)
}
