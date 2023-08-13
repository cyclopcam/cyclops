package server

import (
	"net"
	"net/http"
	"time"

	"github.com/bmharper/cimg/v2"
	"github.com/bmharper/cyclops/pkg/dbh"
	"github.com/bmharper/cyclops/pkg/www"
	"github.com/bmharper/cyclops/server/camera"
	"github.com/bmharper/cyclops/server/configdb"
	"github.com/bmharper/cyclops/server/scanner"
	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
)

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

	// Add to DB
	now := dbh.MakeIntTime(time.Now())
	cfg.CreatedAt = now
	cfg.UpdatedAt = now
	res := s.configDB.DB.Create(&cfg)
	if res.Error != nil {
		www.Check(res.Error)
	}
	s.Log.Infof("Added new camera to DB. Camera ID: %v", cfg.ID)
	s.LiveCameras.CameraAdded(cfg.ID)

	www.SendID(w, cfg.ID)
}

func (s *Server) httpConfigChangeCamera(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	cfgNew := configdb.Camera{}
	www.ReadJSON(w, r, &cfgNew, 1024*1024)

	cfgOld := configdb.Camera{}
	www.Check(s.configDB.DB.First(&cfgOld, cfgNew.ID).Error)

	cfgNew.CreatedAt = cfgOld.CreatedAt
	cfgNew.UpdatedAt = dbh.MakeIntTime(time.Now())

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
	s.LiveCameras.CameraRemoved(camID)
	www.SendOK(w)
}

func (s *Server) httpConfigGetVariableDefinitions(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	www.SendJSON(w, configdb.AllVariables)
}

func (s *Server) httpConfigGetVariableValues(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	values := []configdb.Variable{}
	www.Check(s.configDB.DB.Find(&values).Error)
	www.SendJSON(w, values)
}

func (s *Server) httpConfigSetVariable(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	keyStr := params.ByName("key")
	value := ""
	if r.URL.Query().Has("value") {
		value = r.URL.Query().Get("value")
	} else {
		value = www.ReadString(w, r, 1024*1024)
	}

	key := configdb.VariableKey(keyStr)

	www.CheckClient(configdb.ValidateVariable(key, value))

	db, err := s.configDB.DB.DB()
	www.Check(err)
	_, err = db.Exec("INSERT INTO variable (key, value) VALUES ($1, $2) ON CONFLICT(key) DO UPDATE SET value = EXCLUDED.value", key, value)
	www.Check(err)

	s.Log.Infof("Set config variable %v: %v", key, value)

	// If you receive wantRestart:true, then you should call /api/system/restart when you're ready to restart.
	// You may want to batch a few setVariable calls before restarting.
	// SYNC-SET-VARIABLE-RESPONSE
	type Response struct {
		WantRestart bool `json:"wantRestart"`
	}

	www.SendJSON(w, &Response{
		WantRestart: configdb.VariableSetNeedsRestart(key),
	})
}

func (s *Server) httpConfigScanNetworkForCameras(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	timeoutMS := www.QueryInt(r, "timeout") // timeout in milliseconds
	includeExisting := www.QueryInt(r, "includeExisting") == 1

	options := &scanner.ScanOptions{}
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
	// Cameras have a limited number of listeners, so it's not a good idea to open
	// more connections to a camera than strictly necessary.
	s.LiveCameras.CloseTestCamera()

	cam, err := camera.NewCamera(s.Log, cfg, s.RingBufferSize)
	if err != nil {
		c.WriteJSON(message{Error: err.Error()})
		return
	}
	// From here on out, we need to pay attention to Close() the camera at any failure point.
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
		} else if time.Now().Sub(start) > keyframeTimeout {
			cam.Close(nil)
			c.WriteJSON(message{Error: "Timeout waiting for keyframe"})
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	s.Log.Infof("httpConfigTestCamera finished (success: %v)", success)
}
