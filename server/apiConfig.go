package server

import (
	"net/http"
	"time"

	"github.com/bmharper/cyclops/server/camera"
	"github.com/bmharper/cyclops/server/configdb"
	"github.com/bmharper/cyclops/server/scanner"
	"github.com/bmharper/cyclops/server/www"
	"github.com/julienschmidt/httprouter"
)

func (s *Server) httpConfigAddCamera(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	cam := configdb.Camera{}
	www.ReadJSON(w, r, &cam, 1024*1024)
	cam.ID = 0

	camera, err := camera.NewCamera(s.Log, cam, s.RingBufferSize)
	www.Check(err)

	// Make sure we can talk to the camera
	err = camera.Start()
	if err != nil {
		camera.Close()
		www.Check(err)
	}

	// Add to DB
	res := s.configDB.DB.Create(&cam)
	//s.Log.Infof("cam.ID: %v", cam.ID)
	if res.Error != nil {
		camera.Close()
		www.Check(res.Error)
	}

	// Add to live system
	s.AddCamera(camera)

	www.SendID(w, cam.ID)
}

func (s *Server) httpConfigSetVariable(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	key := params.ByName("key")
	value := www.ReadString(w, r, 1024*1024)

	db, err := s.configDB.DB.DB()
	www.Check(err)
	_, err = db.Exec("INSERT INTO variable (key, value) VALUES ($1, $2) ON CONFLICT(key) DO UPDATE SET value = EXCLUDED.value", key, value)
	www.Check(err)

	// If you receive wantRestart:true, then you should call /api/system/restart when you're ready
	// You may want to batch a few setVariable calls before restarting.
	type Response struct {
		WantRestart bool `json:"wantRestart"`
	}

	www.SendJSON(w, &Response{
		WantRestart: configdb.VariableSetNeedsRestart(configdb.VariableKey(key)),
	})
}

func (s *Server) httpConfigScanNetworkForCameras(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
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
