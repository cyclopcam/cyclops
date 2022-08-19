package server

import (
	"net/http"
	"strconv"
	"time"

	"github.com/bmharper/cyclops/server/camera"
	"github.com/bmharper/cyclops/server/configdb"
	"github.com/bmharper/cyclops/server/defs"
	"github.com/bmharper/cyclops/server/www"
	"github.com/julienschmidt/httprouter"
)

func parseResolutionOrPanic(res string) defs.Resolution {
	r, err := defs.ParseResolution(res)
	if err != nil {
		www.PanicBadRequestf("%v", err)
	}
	return r
}

func (s *Server) getCameraFromIDOrPanic(idStr string) *camera.Camera {
	id, _ := strconv.ParseInt(idStr, 10, 64)
	cam := s.CameraFromID(id)
	if cam == nil {
		www.PanicBadRequestf("Invalid camera ID '%v'", idStr)
	}
	return cam
}

type streamInfoJSON struct {
	FPS    int `json:"fps"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// See CameraInfo in www
// camInfoJSON holds information about a running camera. This is distinct from
// it's configuration, which is stored in model.Camera
type camInfoJSON struct {
	ID   int64          `json:"id"`
	Name string         `json:"name"`
	LD   streamInfoJSON `json:"ld"`
	HD   streamInfoJSON `json:"hd"`
}

func toStreamInfoJSON(s *camera.Stream) streamInfoJSON {
	r := streamInfoJSON{
		FPS: s.FPS(),
	}
	inf := s.Info()
	if inf != nil {
		r.Width = inf.Width
		r.Height = inf.Height
	}
	return r
}

func toCamInfoJSON(c *camera.Camera) *camInfoJSON {
	r := &camInfoJSON{
		ID:   c.ID,
		Name: c.Name,
		LD:   toStreamInfoJSON(c.LowStream),
		HD:   toStreamInfoJSON(c.HighStream),
	}
	return r
}

func (s *Server) httpCamGetInfo(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	cam := s.getCameraFromIDOrPanic(params.ByName("cameraID"))
	www.SendJSON(w, toCamInfoJSON(cam))
}

// Fetch a low res JPG of the camera's last image.
// Example: curl -o img.jpg localhost:8080/camera/latestImage/0
func (s *Server) httpCamGetLatestImage(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	cam := s.getCameraFromIDOrPanic(params.ByName("cameraID"))

	www.CacheNever(w)

	contentType := "image/jpeg"
	img := cam.LatestImage(contentType)
	if img == nil {
		www.PanicBadRequestf("No image available yet")
	}

	w.Header().Set("Content-Type", contentType)
	w.Write(img)
}

// Fetch a high res MP4 of the camera's recent footage
// default duration is 5 seconds
// Example: curl -o recent.mp4 localhost:8080/camera/recentVideo/0?duration=15s
func (s *Server) httpCamGetRecentVideo(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	cam := s.getCameraFromIDOrPanic(params.ByName("cameraID"))
	duration, _ := time.ParseDuration(r.URL.Query().Get("duration"))
	if duration <= 0 {
		duration = 5 * time.Second
	}

	www.CacheNever(w)

	contentType := "video/mp4"
	fn := s.TempFiles.Get()
	raw, err := cam.ExtractHighRes(camera.ExtractMethodClone, duration)
	www.Check(err)
	www.Check(raw.SaveToMP4(fn))

	www.SendTempFile(w, fn, contentType)
}

func (s *Server) httpCamStreamVideo(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	cam := s.getCameraFromIDOrPanic(params.ByName("cameraID"))
	res := parseResolutionOrPanic(params.ByName("resolution"))
	stream := cam.GetStream(res)

	// send backlog for small stream, so user can play immediately.
	// could do the same for high stream too...
	var backlog *camera.VideoDumpReader
	if res == defs.ResLD {
		backlog = cam.LowDumper
	}

	s.Log.Infof("httpCamStreamVideo websocket upgrading")

	c, err := s.wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		s.Log.Errorf("httpCamStreamVideo websocket upgrade failed: %v", err)
		return
	}
	defer c.Close()

	s.Log.Infof("httpCamStreamVideo starting")

	streamer := camera.NewVideoWebSocketStreamer(cam.Name, s.Log)
	streamer.Run(c, stream, backlog)
}
