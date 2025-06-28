package server

import (
	"net"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bmharper/cimg/v2"
	"github.com/cyclopcam/cyclops/pkg/shell"
	"github.com/cyclopcam/cyclops/server/camera"
	"github.com/cyclopcam/cyclops/server/configdb"
	"github.com/cyclopcam/cyclops/server/scanner"
	"github.com/cyclopcam/www"
	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
)

var digitRegex = regexp.MustCompile(`\d+`)

func (s *Server) httpConfigGetCamera(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	id := www.ParseID(params.ByName("cameraID"))
	cam := configdb.Camera{}
	www.Check(s.configDB.DB.First(&cam, id).Error)
	www.SendJSON(w, &cam)
}

func (s *Server) httpConfigGetCameras(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	cams := []*configdb.Camera{}
	www.Check(s.configDB.DB.Find(&cams).Error)
	www.SendJSON(w, cams)
}

func (s *Server) httpConfigAddCamera(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	cfg := configdb.Camera{}
	www.ReadJSON(w, r, &cfg, 1024*1024)

	cfg.ID = 0

	tx := s.configDB.DB.Begin()
	www.Check(tx.Error)
	defer tx.Rollback()

	if cfg.LongLivedName == "" {
		llid, err := s.configDB.GenerateNewID(tx, "cameraLongLivedName")
		www.Check(err)
		cfg.LongLivedName = "cam-" + strconv.FormatInt(llid, 10)
	}

	// Add to DB
	www.Check(tx.Create(&cfg).Error)
	www.Check(tx.Commit().Error)

	s.Log.Infof("Added new camera to DB. Camera ID: %v", cfg.ID)
	s.LiveCameras.CameraAdded(cfg.ID)

	www.SendID(w, cfg.ID)
}

func (s *Server) httpConfigChangeCamera(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	cfgNew := configdb.Camera{}
	www.ReadJSON(w, r, &cfgNew, 1024*1024)

	cfgOld := configdb.Camera{}
	www.Check(s.configDB.DB.First(&cfgOld, cfgNew.ID).Error)

	cfgNew.LongLivedName = cfgOld.LongLivedName
	cfgNew.CreatedAt = cfgOld.CreatedAt

	// Update DB
	if err := s.configDB.DB.Save(&cfgNew).Error; err != nil {
		www.PanicServerErrorf("Error saving camera config to DB: %v", err)
	}

	s.LiveCameras.CameraChanged(cfgNew.ID)

	www.SendOK(w)
}

func (s *Server) httpConfigRemoveCamera(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	camID := www.ParseID(params.ByName("cameraID"))
	cam := configdb.Camera{}
	www.Check(s.configDB.DB.First(&cam, camID).Error)
	www.Check(s.configDB.DB.Delete(&cam).Error)
	s.Log.Infof("Removed camera %v (%v) from DB", camID, cam.Name)
	s.LiveCameras.CameraRemoved(camID)
	www.SendOK(w)
}

func (s *Server) httpConfigScanNetworkForCameras(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	timeoutMS := www.QueryInt(r, "timeout") // timeout in milliseconds
	includeExisting := www.QueryInt(r, "includeExisting") == 1

	options := &scanner.ScanOptions{
		Log: s.Log,
	}
	if timeoutMS != 0 {
		options.Timeout = time.Millisecond * time.Duration(timeoutMS)
	}
	if s.OwnIP != nil {
		options.OwnIP = s.OwnIP
	}
	if !includeExisting {
		existing := []configdb.Camera{}
		s.configDB.DB.Find(&existing) // ignore errors
		for _, cam := range existing {
			// We could resolve Host -> IP here, but that would add latency, and I'm not sure this fringe feature is worth much attention
			if ip := net.ParseIP(cam.Host); ip != nil {
				options.ExcludeIPs = append(options.ExcludeIPs, ip)
			}
		}
	}
	cameras, err := scanner.ScanForLocalCameras(options)
	if err != nil {
		www.PanicServerError(err.Error())
	}
	s.Log.Infof("Network scanner found %v cameras", len(cameras))

	www.SendJSON(w, cameras)
}

// ConfigTestCamera is used by the front-end when adding a new camera
// We use a websocket so that we can show progress while waiting for a keyframe.
// The difference between doing this with a websocket and regular HTTP call
// is maybe 1 or 2 seconds latency (depending on camera's keyframe interval),
// but I want to spark joy.
func (s *Server) httpConfigTestCamera(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	s.Log.Infof("httpConfigTestCamera starting")

	// My cameras are set to 3 seconds between keyframes, but sometimes it takes over 7 seconds before I receive the first
	// keyframe. This is weird... it didn't used to be like this. Can't figure out what changed.
	keyframeTimeout := 11 * time.Second

	c, err := s.wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		s.Log.Errorf("httpConfigTestCamera websocket upgrade failed: %v", err)
		return
	}
	defer c.Close()

	cfg := configdb.Camera{}
	if err := c.ReadJSON(&cfg); err != nil {
		s.Log.Errorf("Client sent invalid Camera to tester: %v", err)
		return
	}

	type message struct {
		Error  string `json:"error"`
		Status string `json:"status"`
		Image  string `json:"image"`
	}

	// In case the previous test camera is the same as this one, close it.
	// Cameras (i.e. the hardware devices in the real world) have a limited number of listeners,
	// so it's not a good idea to open more connections to a camera than strictly necessary.
	s.LiveCameras.CloseTestCamera()

	cam, err := camera.NewCamera(s.Log, cfg, s.RingBufferSize)
	if err != nil {
		c.WriteJSON(message{Error: err.Error()})
		return
	}
	// From here on out, we need to make sure to Close() the camera at any failure point.
	if err := cam.Start(); err != nil {
		cam.Close(nil)
		c.WriteJSON(message{Error: err.Error()})
		return
	}
	if err := c.WriteJSON(message{Status: "Connected. Waiting for keyframe..."}); err != nil {
		s.Log.Errorf("Tester failed to send Connected.. message to websocket: %v", err)
	}

	success := false
	start := time.Now()
	for {
		img, _ := cam.LowDecoder.LastImageCopy()
		if img != nil {
			s.Log.Infof("Success connecting to camera %v after %v", cfg.Host, time.Since(start))
			// Stash this camera, because the next call is extremely likely to be AddCamera(), which will re-use this
			// already-connected camera, thereby shorterning the time it takes to add a camera to the system.
			s.LiveCameras.SaveTestCamera(cfg, cam)

			// Yes, this is stupid going from YUV to RGB, to YUV, to JPEG.
			jpg, err := cimg.Compress(img.ToCImageRGB(), cimg.MakeCompressParams(cimg.Sampling420, 85, 0))
			if err != nil {
				c.WriteJSON(message{Error: "Failed to compress image to JPEG: " + err.Error()})
			} else {
				c.WriteMessage(websocket.BinaryMessage, jpg)
			}
			success = true
			break
		} else if time.Since(start) > keyframeTimeout {
			cam.Close(nil)
			c.WriteJSON(message{Error: "Timeout waiting for keyframe"})
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	s.Log.Infof("httpConfigTestCamera finished (success: %v)", success)
}

// The user wants to know how much space is available for storing videos
// at the given location. We return how much space is available on that
// volume, and also how much space is used by that location.
func (s *Server) httpConfigMeasureStorageSpace(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	inPath := strings.TrimSpace(www.RequiredQueryValue(r, "path"))
	path := filepath.Clean(inPath)
	// We refuse to run this operation on "/" because the "du" would take very long
	if strings.HasSuffix(path, string(filepath.Separator)) {
		www.PanicBadRequestf("Invalid measurement path")
	}

	s.Log.Infof("Measure space available at %v (raw %v)", path, inPath)
	availB, err := configdb.MeasureDiskSpaceAvailable(path)
	if err != nil {
		www.PanicBadRequestf("%v", err)
	}

	// For the disk used portion, we don't consider it a failed API call
	// if we can't get the amount of disk space used. This is because
	// the path might not exist yet.
	usedB := int64(0)

	// Measure the amount of space used by /path
	// du -s -b /path
	// Example output:
	// 12345678 /path
	res, err := shell.Run("du", "-sb", path)
	if err != nil {
		s.Log.Warnf("Failed to read space used: %v", err)
		//www.PanicBadRequestf("Failed to read space used: %v", err)
	} else {
		usedStr := digitRegex.FindString(string(res))
		usedB, err = strconv.ParseInt(usedStr, 10, 64)
	}

	output := struct {
		Available int64 `json:"available"`
		Used      int64 `json:"used"`
	}{
		Available: availB,
		Used:      usedB,
	}

	www.SendJSON(w, &output)
}

func (s *Server) httpConfigGetSettings(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	www.SendJSON(w, s.configDB.GetConfig())
}

func (s *Server) httpConfigSetSettings(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	config := configdb.ConfigJSON{}
	www.ReadJSON(w, r, &config, 1024*1024)
	needsRestart, err := s.configDB.SetConfig(config)
	www.Check(err)
	resp := struct {
		NeedsRestart bool `json:"needsRestart"`
	}{
		NeedsRestart: needsRestart,
	}
	www.SendJSON(w, &resp)
}
