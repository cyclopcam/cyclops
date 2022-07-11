package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bmharper/cyclops/server/camera"
	"github.com/bmharper/cyclops/server/configdb"
	"github.com/bmharper/cyclops/server/dbh"
	"github.com/bmharper/cyclops/server/eventdb"
	"github.com/bmharper/cyclops/server/gen"
	"github.com/bmharper/cyclops/server/log"
	"github.com/bmharper/cyclops/server/util"
	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
	"gorm.io/gorm"
)

type Server struct {
	Log              log.Log
	ConfigDBFilename string // Path to sqlite config DB
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
	configDB        *gorm.DB
	permanentEvents *eventdb.EventDB // Where we store our permanent videos
	recentEvents    *eventdb.EventDB // Where we store our recent event videos
	wsUpgrader      websocket.Upgrader

	recorderStartStopLock sync.Mutex
	recorderStop          chan bool // Sent to recorder to tell it to step
}

// After calling NewServer, you must call LoadConfig() to setup additional things like
// the TempFiles object.
func NewServer(configDBFilename string) (*Server, error) {
	log, err := log.NewLog()
	if err != nil {
		return nil, err
	}
	s := &Server{
		Log:              log,
		ConfigDBFilename: configDBFilename,
		RingBufferSize:   200 * 1024 * 1024,
		ShutdownComplete: make(chan error, 1),
		cameraFromID:     map[int64]*camera.Camera{},
	}
	if err := s.openConfigDB(); err != nil {
		return nil, err
	}
	if err := s.LoadConfigVariables(); err != nil {
		return nil, err
	}
	if err := s.LoadCamerasFromConfig(); err != nil {
		return nil, err
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
	s.Log.Infof("Shutdown (restart = %v)", restart)
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

func (s *Server) openConfigDB() error {
	os.MkdirAll(filepath.Dir(s.ConfigDBFilename), 0777)
	configDB, err := dbh.OpenDB(s.Log, dbh.DriverSqlite, s.ConfigDBFilename, configdb.Migrations(s.Log), 0)
	if err != nil {
		return fmt.Errorf("Failed to open database %v: %w", s.ConfigDBFilename, err)
	}
	s.configDB = configDB
	return nil
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
	if err := s.configDB.Find(&vars).Error; err != nil {
		return err
	}
	for _, v := range vars {
		var err error
		switch configdb.VariableKey(v.Key) {
		case configdb.VarPermanentStoragePath:
			err = s.SetPermanentStoragePath(v.Value)
		case configdb.VarRecentEventStoragePath:
			err = s.SetRecentEventStoragePath(v.Value)
		case configdb.VarTempFilePath:
			err = s.SetTempFilePath(v.Value)
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
	if err := s.configDB.Find(&cams).Error; err != nil {
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
