package server

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bmharper/cyclops/server/camera"
	"github.com/bmharper/cyclops/server/config"
	"github.com/bmharper/cyclops/server/log"
	"github.com/bmharper/cyclops/server/util"
	"github.com/gorilla/websocket"
)

type Server struct {
	Log         log.Log
	Cameras     []*camera.Camera
	StorageRoot string // Where we store our videos
	TempFiles   *util.TempFiles

	wsUpgrader websocket.Upgrader
}

// After calling NewServer, you must call LoadConfig() to setup additional things like
// the TempFiles object.
func NewServer() *Server {
	log, err := log.NewLog()
	if err != nil {
		panic(err)
	}
	return &Server{
		Log: log,
	}
}

func (s *Server) LoadConfig(cfg config.Config) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("Failed to get home dir: %v", err)
	}

	if cfg.StoragePath != "" {
		s.StorageRoot = cfg.StoragePath
	} else {
		s.StorageRoot = filepath.Join(home, "cyclops", "videos")
	}
	if err := os.MkdirAll(s.StorageRoot, 0777); err != nil {
		return fmt.Errorf("Failed to create video storage root '%v': %w", s.StorageRoot, err)
	}

	// We don't want temp files to be on the videos dir, because the videos are likely to be
	// stored on a USB flash drive, and this could cause the temp file to get written to disk,
	// when we don't actually want that. We just want it as swap space... i.e. only written to disk
	// if we run out of RAM.
	tempPath := filepath.Join(home, "cyclops", "temp")
	if cfg.TempPath != "" {
		tempPath = cfg.TempPath
	}

	if tempFiles, err := util.NewTempFiles(tempPath); err != nil {
		return err
	} else {
		s.TempFiles = tempFiles
	}

	cameraBufferMB := cfg.CameraBufferMB
	if cameraBufferMB == 0 {
		cameraBufferMB = 200
	} else if cameraBufferMB <= 1 {
		return fmt.Errorf("cameraBufferMB (%v) is too small. Recommended at least 100. Minimum 1", cameraBufferMB)
	}

	for _, cam := range cfg.Cameras {
		lowRes, err := camera.URLForCamera(cam.Model, cam.URL, cam.LowResURLSuffix, cam.HighResURLSuffix, false)
		if err != nil {
			return err
		}
		highRes, err := camera.URLForCamera(cam.Model, cam.URL, cam.LowResURLSuffix, cam.HighResURLSuffix, true)
		if err != nil {
			return err
		}
		cam, err := camera.NewCamera(cam.Name, s.Log, lowRes, highRes, cameraBufferMB*1024*1024)
		if err != nil {
			return err
		}
		s.AddCamera(cam)
	}
	return nil
}

func (s *Server) AddCamera(cam *camera.Camera) {
	s.Cameras = append(s.Cameras, cam)
}

func (s *Server) StartAll() error {
	var firstErr error
	for _, cam := range s.Cameras {
		if err := cam.Start(); err != nil {
			s.Log.Errorf("Error starting camera %v: %v", cam.Name, err)
			firstErr = err
		}
	}
	return firstErr
}
