package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/bmharper/cimg/v2"
	"github.com/cyclopcam/cyclops/pkg/www"
	"github.com/cyclopcam/cyclops/server/camera"
	"github.com/cyclopcam/cyclops/server/configdb"
	"github.com/cyclopcam/cyclops/server/defs"
	"github.com/cyclopcam/cyclops/server/streamer"
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
	cam := s.LiveCameras.CameraFromID(id)
	if cam == nil {
		www.PanicBadRequestf("Invalid camera ID '%v'", idStr)
	}
	return cam
}

type streamInfoJSON struct {
	FPS            int     `json:"fps"`
	Width          int     `json:"width"`
	Height         int     `json:"height"`
	FrameSize      float64 `json:"frameSize"`
	KeyFrameSize   float64 `json:"keyFrameSize"`
	InterFrameSize float64 `json:"interFrameSize"`
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
	stats := s.RecentFrameStats()
	r := streamInfoJSON{
		FPS:            stats.FPSRounded(),
		FrameSize:      stats.FrameSize,
		KeyFrameSize:   stats.KeyFrameSize,
		InterFrameSize: stats.InterFrameSize,
	}
	inf := s.Info()
	if inf != nil {
		r.Width = inf.Width
		r.Height = inf.Height
	}
	return r
}

func liveToCamInfoJSON(c *camera.Camera) *camInfoJSON {
	r := &camInfoJSON{
		ID:   c.ID(),
		Name: c.Name(),
		LD:   toStreamInfoJSON(c.LowStream),
		HD:   toStreamInfoJSON(c.HighStream),
	}
	return r
}

func cfgToCamInfoJSON(c *configdb.Camera) *camInfoJSON {
	r := &camInfoJSON{
		ID:   c.ID,
		Name: c.Name,
	}
	return r
}

func (s *Server) httpCamGetInfo(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	cam := s.getCameraFromIDOrPanic(params.ByName("cameraID"))
	www.SendJSON(w, liveToCamInfoJSON(cam))
}

// Fetch a low res JPG of the camera's last image.
// Example: curl -o img.jpg localhost:8080/camera/latestImage/0
func (s *Server) httpCamGetLatestImage(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	cam := s.getCameraFromIDOrPanic(params.ByName("cameraID"))

	www.CacheNever(w)

	contentType := "image/jpeg"
	var encodedImg []byte

	// First try to get latest frame that has had NN detections run on it
	img, detections, analysis, err := s.monitor.LatestFrame(cam.ID())
	if err == nil {
		encodedImg, err = cimg.Compress(img, cimg.MakeCompressParams(cimg.Sampling420, 85, 0))
		www.Check(err)
		jsDet, err := json.Marshal(detections)
		www.Check(err)
		jsAna, err := json.Marshal(analysis)
		www.Check(err)
		// We must send Content-Type before X-Detections or X-Analysis... not sure if that's browser or Go HTTP infra, but it's a thing.
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("X-Detections", string(jsDet))
		w.Header().Set("X-Analysis", string(jsAna))
	} else {
		// Fall back to latest frame without NN detections
		s.Log.Infof("httpCamGetLatestImage fallback on camera %v (%v)", cam.ID(), err)
		encodedImg = cam.LatestImage(contentType)
		if encodedImg == nil {
			www.PanicBadRequestf("No image available yet")
		}
		w.Header().Set("Content-Type", contentType)
	}

	w.Write(encodedImg)
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
	fn := s.TempFiles.GetOnceOff()
	raw, err := cam.ExtractHighRes(camera.ExtractMethodShallowClone, duration)
	www.Check(err)
	www.Check(raw.SaveToMP4(fn))

	www.SendTempFile(w, r, fn, contentType)
}

func (s *Server) httpCamStreamVideo(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	cam := s.getCameraFromIDOrPanic(params.ByName("cameraID"))
	res := parseResolutionOrPanic(params.ByName("resolution"))
	stream := cam.GetStream(res)

	// send backlog for small stream, so user can play immediately.
	// could do the same for high stream too...
	var backlog *camera.VideoRingBuffer
	if res == defs.ResLD {
		backlog = cam.LowDumper
	}

	s.Log.Infof("httpCamStreamVideo websocket upgrading")

	conn, err := s.wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		s.Log.Errorf("httpCamStreamVideo websocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	s.Log.Infof("httpCamStreamVideo starting")

	newDetections := s.monitor.AddWatcher(cam.ID())

	streamer.RunVideoWebSocketStreamer(cam.Name(), s.Log, conn, stream, backlog, newDetections)

	s.monitor.RemoveWatcher(newDetections)

	s.Log.Infof("httpCamStreamVideo done")
}
