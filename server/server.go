package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bmharper/cyclops/server/camera"
	"github.com/bmharper/cyclops/server/configdb"
	"github.com/bmharper/cyclops/server/eventdb"
	"github.com/bmharper/cyclops/server/gen"
	"github.com/bmharper/cyclops/server/log"
	"github.com/bmharper/cyclops/server/util"
	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
)

type Server struct {
	Log              log.Log
	TempFiles        *util.TempFiles
	RingBufferSize   int
	IsShutdownV      int32 // 1 if we were shutdown with an explicit call to Shutdown. Use IsShutdown() for a thread-safe check if IsShutdownV is true.
	MustRestart      bool  // Value of the 'restart' parameter to Shutdown()
	ShutdownComplete chan error

	camerasLock  sync.Mutex
	cameras      []*camera.Camera
	cameraFromID map[int64]*camera.Camera

	signalIn        chan os.Signal
	httpServer      *http.Server
	httpRouter      *httprouter.Router
	configDB        *configdb.ConfigDB
	permanentEvents *eventdb.EventDB // Where we store our permanent videos
	recentEvents    *eventdb.EventDB // Where we store our recent event videos
	wsUpgrader      websocket.Upgrader

	recorderStartStopLock sync.Mutex
	recorderStop          chan bool // Sent to recorder to tell it to stop

	lastScannedCamerasLock sync.Mutex
	lastScannedCameras     []*configdb.Camera
}

// Create a new server, load config, start cameras, and listen on HTTP
func NewServer(configDBFilename string) (*Server, error) {
	log, err := log.NewLog()
	if err != nil {
		return nil, err
	}
	s := &Server{
		Log:              log,
		RingBufferSize:   200 * 1024 * 1024,
		ShutdownComplete: make(chan error, 1),
		cameraFromID:     map[int64]*camera.Camera{},
	}
	if cfg, err := configdb.NewConfigDB(s.Log, configDBFilename); err != nil {
		return nil, err
	} else {
		s.configDB = cfg
	}
	if err := s.configDB.GuessDefaultVariables(); err != nil {
		log.Errorf("GuessDefaultVariables failed: %v", err)
	}
	// If config variables fail to load, then we must still continue to boot ourselves up to the point
	// where we can accept new config. Otherwise, the system is bricked if the user enters
	// invalid config.
	// Also, when the system first starts up, it won't be configured at all.
	configOK := true
	if err := s.LoadConfigVariables(); err != nil {
		log.Errorf("%v", err)
		configOK = false
	}
	if configOK {
		if err := s.LoadCamerasFromConfig(); err != nil {
			return nil, err
		}
	}
	s.SetupHTTP()
	return s, nil
}

// port example: ":8080"
func (s *Server) ListenHTTP(port string) error {
	s.Log.Infof("Listening on %v", port)
	s.httpServer = &http.Server{
		Addr:    port,
		Handler: s.httpRouter,
	}
	return s.httpServer.ListenAndServe()
}

func (s *Server) ListenForInterruptSignal() {
	s.signalIn = make(chan os.Signal, 1)
	signal.Notify(s.signalIn, os.Interrupt)
	go func() {
		for sig := range s.signalIn {
			s.Log.Infof("Received OS signal %v", sig.String())
			s.Shutdown(false)
		}
	}()
}

func (s *Server) IsShutdown() bool {
	return atomic.LoadInt32(&s.IsShutdownV) != 0
}

func (s *Server) Shutdown(restart bool) {
	if restart {
		s.Log.Infof("Restart")
	} else {
		s.Log.Infof("Shutdown")
	}
	atomic.StoreInt32(&s.IsShutdownV, 1)
	s.MustRestart = restart

	// cancel signal handler (we'll re-enable it again if we restart)
	signal.Stop(s.signalIn)

	s.recorderStartStopLock.Lock()
	if s.recorderStop != nil {
		s.recorderStop <- true
	}
	s.recorderStartStopLock.Unlock()

	// CloseAllCameras() should close all WebSockets, by virtue of the Streams closing, which sends
	// a message to the websocket thread.
	// This is relevant because calling Shutdown() on our http server will not do anything to upgraded
	// connections (this is explicit in the http server docs).
	// NOTE: there is also RegisterOnShutdown.. which might be useful
	s.CloseAllCameras()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	err := s.httpServer.Shutdown(ctx)
	defer cancel()

	s.Log.Infof("Shutdown complete (err: %v)", err)
	s.Log.Close()
	s.ShutdownComplete <- err
}

// Returns nil if the system is ready to start listening to cameras
// Returns an error if some part of the system needs configuring
// The idea is that the web client will continue to show the configuration page
// until IsReady() returns true.
func (s *Server) IsReady() error {
	if s.TempFiles == nil {
		return fmt.Errorf("Variable %v is not set (for temporary files location)", configdb.VarTempFilePath)
	}
	if s.permanentEvents == nil {
		return fmt.Errorf("Variable %v is not set (for permanent event storage)", configdb.VarPermanentStoragePath)
	}
	if s.recentEvents == nil {
		return fmt.Errorf("Variable %v is not set (for recent event storage)", configdb.VarRecentEventStoragePath)
	}
	return nil
}

func (s *Server) CameraFromID(id int64) *camera.Camera {
	s.camerasLock.Lock()
	defer s.camerasLock.Unlock()
	return s.cameraFromID[id]
}

func (s *Server) Cameras() []*camera.Camera {
	s.camerasLock.Lock()
	defer s.camerasLock.Unlock()
	return gen.CopySlice(s.cameras)
}

// Add a newly configured, running camera
func (s *Server) AddCamera(cam *camera.Camera) {
	s.camerasLock.Lock()
	defer s.camerasLock.Unlock()
	s.cameras = append(s.cameras, cam)
	s.cameraFromID[cam.ID] = cam
}

func (s *Server) CloseAllCameras() {
	s.camerasLock.Lock()
	defer s.camerasLock.Unlock()
	for _, cam := range s.cameras {
		cam.Close()
	}
	s.cameras = []*camera.Camera{}
	s.cameraFromID = map[int64]*camera.Camera{}
}

// Load state from 'variables'
func (s *Server) LoadConfigVariables() error {
	vars := []configdb.Variable{}
	if err := s.configDB.DB.Find(&vars).Error; err != nil {
		return err
	}
	for _, v := range vars {
		trimmed := strings.TrimSpace(v.Value)
		if trimmed == "" {
			// I added this after building the UI, where it's just so hard to avoid empty strings
			continue
		}
		var err error
		switch configdb.VariableKey(v.Key) {
		case configdb.VarPermanentStoragePath:
			err = s.SetPermanentStoragePath(trimmed)
		case configdb.VarRecentEventStoragePath:
			err = s.SetRecentEventStoragePath(trimmed)
		case configdb.VarTempFilePath:
			err = s.SetTempFilePath(trimmed)
		default:
			s.Log.Errorf("Config variable '%v' not recognized", v.Key)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// Loads cameras from config, but does not start them yet
func (s *Server) LoadCamerasFromConfig() error {
	s.CloseAllCameras()
	cams := []configdb.Camera{}
	if err := s.configDB.DB.Find(&cams).Error; err != nil {
		return err
	}
	for _, cam := range cams {
		if camera, err := camera.NewCamera(s.Log, cam, s.RingBufferSize); err != nil {
			return err
		} else {
			camera.ID = cam.ID
			s.AddCamera(camera)
		}
	}
	return nil
}

// Start all cameras
// If a camera fails to start, it is skipped, and other cameras are tried
// Returns the first error
func (s *Server) StartAllCameras() error {
	var firstErr error
	for _, cam := range s.cameras {
		if err := cam.Start(); err != nil {
			s.Log.Errorf("Error starting camera %v: %v", cam.Name, err)
			firstErr = err
		}
	}
	return firstErr
}
