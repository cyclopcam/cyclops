package server

import (
	"net/http"
	"time"

	"github.com/bmharper/cimg/v2"
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
	cam := configdb.Camera{}
	www.ReadJSON(w, r, &cam, 1024*1024)
	cam.ID = 0

	camera, err := camera.NewCamera(s.Log, cam, s.RingBufferSize)
	www.Check(err)

	// Make sure we can talk to the camera
	err = camera.Start()
	if err != nil {
		camera.Close(nil)
		www.Check(err)
	}

	// Add to DB
	res := s.configDB.DB.Create(&cam)
	if res.Error != nil {
		camera.Close(nil)
		www.Check(res.Error)
	}
	s.Log.Infof("Added new camera to DB. Camera ID: %v", cam.ID)

	// Add to live system
	camera.ID = cam.ID
	s.LiveCameras.AddCamera(camera)

	www.SendID(w, cam.ID)
}

func (s *Server) httpConfigChangeCamera(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	cam := configdb.Camera{}
	www.ReadJSON(w, r, &cam, 1024*1024)

	if s.LiveCameras.CameraFromID(cam.ID) == nil {
		www.PanicBadRequestf("Camera ID %v not found", cam.ID)
	}

	// Create a new Camera object and open a connection
	camera, err := camera.NewCamera(s.Log, cam, s.RingBufferSize)
	www.Check(err)
	err = camera.Start()
	if err != nil {
		camera.Close(nil)
		www.Check(err)
	}

	// Update DB
	if err := s.configDB.DB.Save(&cam).Error; err != nil {
		camera.Close(nil)
		www.PanicServerErrorf("Error saving camera config to DB: %v", err)
	}

	// Update live system
	if err := s.LiveCameras.ReplaceCamera(camera); err != nil {
		camera.Close(nil)
		www.PanicServerErrorf("Error replacing camera: %v", err)
	}

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
	cache := www.QueryValue(r, "cache")
	timeoutMS := www.QueryInt(r, "timeout") // timeout in milliseconds

	s.lastScannedCamerasLock.Lock()
	cacheSize := len(s.lastScannedCameras)
	s.lastScannedCamerasLock.Unlock()

	if cache == "nocache" || (cache == "" && cacheSize == 0) {
		options := &scanner.ScanOptions{}
		if timeoutMS != 0 {
			options.Timeout = time.Millisecond * time.Duration(timeoutMS)
		}
		if s.OwnIP != nil {
			options.OwnIP = s.OwnIP
		}
		cameras, err := scanner.ScanForLocalCameras(options)
		if err != nil {
			www.PanicServerError(err.Error())
		}
		s.Log.Infof("Network scanner found %v cameras", len(cameras))
		s.lastScannedCamerasLock.Lock()
		s.lastScannedCameras = cameras
		s.lastScannedCamerasLock.Unlock()
	}

	s.lastScannedCamerasLock.Lock()
	defer s.lastScannedCamerasLock.Unlock()
	www.SendJSON(w, s.lastScannedCameras)
}

// ConfigTestCamera is used by the front-end when adding a new camera
// We use a websocket so that we can show progress while waiting for a keyframe.
// The difference between doing this with a websocket and regular HTTP call
// is maybe 1 or 2 seconds latency (depending on camera's keyframe interval),
// but I want to spark joy.
func (s *Server) httpConfigTestCamera(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	//www.ReadJSON(w, r, &cfg, 1024*1024)
	s.Log.Infof("httpConfigTestCamera starting")

	timeoutSeconds := www.QueryInt(r, "timeout")
	if timeoutSeconds >= 0 {
		timeoutSeconds = 7
	} else if timeoutSeconds > 30 {
		timeoutSeconds = 30
	}

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

	cam, err := camera.NewCamera(s.Log, cfg, 8*1024*1024)
	if err != nil {
		c.WriteJSON(message{Error: err.Error()})
		return
	}
	defer cam.Close(nil)
	if err := cam.Start(); err != nil {
		c.WriteJSON(message{Error: err.Error()})
		return
	}
	if err := c.WriteJSON(message{Status: "Connected. Waiting for keyframe..."}); err != nil {
		s.Log.Warnf("Tester failed to send Connected.. message to websocket: %v", err)
	}

	success := false
	start := time.Now()
	for {
		img, _ := cam.LowDecoder.LastImageCopy()
		if img != nil {
			// Yes, this is stupid going from YUV to RGB, to YUV, to JPEG.
			jpg, err := cimg.Compress(img.ToCImageRGB(), cimg.MakeCompressParams(cimg.Sampling420, 85, 0))
			if err != nil {
				c.WriteJSON(message{Error: "Failed to compress image to JPEG: " + err.Error()})
			}
			c.WriteMessage(websocket.BinaryMessage, jpg)
			success = true
			break
		} else if time.Now().Sub(start) > time.Duration(timeoutSeconds)*time.Second {
			c.WriteJSON(message{Error: "Timeout waiting for keyframe"})
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	s.Log.Infof("httpConfigTestCamera finished (success: %v)", success)
}
