package server

import (
	"fmt"
	"sync"

	"github.com/bmharper/cyclops/server/camera"
	"github.com/bmharper/cyclops/server/configdb"
)

// LiveCameras stores the list of currently running cameras
type LiveCameras struct {
	parent *Server

	camerasLock  sync.Mutex
	cameraFromID map[int64]*camera.Camera
}

func NewLiveCameras(parent *Server) *LiveCameras {
	return &LiveCameras{
		parent:       parent,
		cameraFromID: map[int64]*camera.Camera{},
	}
}

// Return a slice of all cameras.
// Assumes you've already taken camerasLock.
func (s *LiveCameras) cameraListNoLock() []*camera.Camera {
	list := make([]*camera.Camera, 0, len(s.cameraFromID))
	for _, c := range s.cameraFromID {
		list = append(list, c)
	}
	return list
}

func (s *LiveCameras) CameraFromID(id int64) *camera.Camera {
	s.camerasLock.Lock()
	defer s.camerasLock.Unlock()
	return s.cameraFromID[id]
}

// Return a list of running cameras
func (s *LiveCameras) Cameras() []*camera.Camera {
	s.camerasLock.Lock()
	defer s.camerasLock.Unlock()
	return s.cameraListNoLock()
}

// Add a running camera
func (s *LiveCameras) AddCamera(cam *camera.Camera) {
	s.camerasLock.Lock()
	defer s.camerasLock.Unlock()
	s.cameraFromID[cam.ID] = cam

	if s.parent.monitor != nil {
		// Monitor is nil during startup, but not nil if a Camera is newly configured and added to the system.
		s.parent.monitor.SetCameras(s.cameraListNoLock())
	}
}

// Remove a running camera
// If the camera does not exist, then the function returns immediately
func (s *LiveCameras) RemoveCamera(camID int64) {
	s.camerasLock.Lock()
	defer s.camerasLock.Unlock()
	cam := s.cameraFromID[camID]
	if cam == nil {
		return
	}
	delete(s.cameraFromID, camID)
	if s.parent.monitor != nil {
		s.parent.monitor.SetCameras(s.cameraListNoLock())
	}
}

// Replace a running camera
// This is used when a camera is reconfigured
func (s *LiveCameras) ReplaceCamera(cam *camera.Camera) error {
	s.camerasLock.Lock()
	defer s.camerasLock.Unlock()
	existing := s.cameraFromID[cam.ID]
	if existing == nil {
		return fmt.Errorf("Camera %v does not exist", cam.ID)
	}
	s.cameraFromID[cam.ID] = cam
	if s.parent.monitor != nil {
		s.parent.monitor.SetCameras(s.cameraListNoLock())
	}
	existing.Close(nil)
	return nil
}

// Start all cameras
// If a camera fails to start, it is skipped, and other cameras are tried
// Returns the first error
func (s *LiveCameras) StartAllCameras() error {
	var firstErr error
	s.camerasLock.Lock()
	defer s.camerasLock.Unlock()
	for _, cam := range s.cameraFromID {
		if err := cam.Start(); err != nil {
			s.parent.Log.Errorf("Error starting camera %v: %v", cam.Name, err)
			firstErr = err
		}
	}
	return firstErr
}

func (s *LiveCameras) closeAllCameras(wg *sync.WaitGroup) {
	s.camerasLock.Lock()
	defer s.camerasLock.Unlock()
	for _, cam := range s.cameraFromID {
		cam.Close(wg)
	}
	s.cameraFromID = map[int64]*camera.Camera{}
}

// Loads cameras from config, but does not start them yet
func (s *LiveCameras) loadCamerasFromConfig() error {
	cams := []configdb.Camera{}
	if err := s.parent.configDB.DB.Find(&cams).Error; err != nil {
		return err
	}

	s.camerasLock.Lock()
	defer s.camerasLock.Unlock()
	for _, cam := range cams {
		if camera, err := camera.NewCamera(s.parent.Log, cam, s.parent.RingBufferSize); err != nil {
			return err
		} else {
			s.cameraFromID[camera.ID] = camera
		}
	}
	return nil
}
