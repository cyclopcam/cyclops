package server

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bmharper/cyclops/server/camera"
	"github.com/bmharper/cyclops/server/config"
	"github.com/bmharper/cyclops/server/log"
	"github.com/bmharper/cyclops/server/util"
)

type Server struct {
	Log         log.Log
	Cameras     []*camera.Camera
	StorageRoot string // Where we store our videos
	TempFiles   *util.TempFiles
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
	if cfg.StoragePath != "" {
		s.StorageRoot = cfg.StoragePath
	} else {
		if home, err := os.UserHomeDir(); err != nil {
			return fmt.Errorf("Failed to get home dir for storage path: %v", err)
		} else {
			s.StorageRoot = home + "/cyclops-videos"
		}
	}
	if err := os.MkdirAll(s.StorageRoot, 0777); err != nil {
		return fmt.Errorf("Failed to create video storage root '%v': %w", s.StorageRoot, err)
	}

	if tempFiles, err := util.NewTempFiles(filepath.Join(s.StorageRoot, "temp")); err != nil {
		return err
	} else {
		s.TempFiles = tempFiles
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
		cam, err := camera.NewCamera(cam.Name, s.Log, lowRes, highRes, cfg.CameraBufferMB*1024*1024)
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
