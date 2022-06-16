package server

import (
	"github.com/bmharper/cyclops/server/camera"
	"github.com/bmharper/cyclops/server/log"
)

type Server struct {
	Log     log.Log
	cameras []*camera.Camera
}

func NewServer() *Server {
	log, err := log.NewLog()
	if err != nil {
		panic(err)
	}
	return &Server{
		Log: log,
	}
}

func (s *Server) AddCamera(cam *camera.Camera) {
	s.cameras = append(s.cameras, cam)
}

func (s *Server) StartAll() error {
	var firstErr error
	for _, cam := range s.cameras {
		if err := cam.Start(); err != nil {
			s.Log.Errorf("Error starting camera %v: %v", cam.Name, err)
			firstErr = err
		}
	}
	return firstErr
}
