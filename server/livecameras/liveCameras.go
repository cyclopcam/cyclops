package livecameras

import (
	"sort"
	"sync"
	"time"

	"github.com/cyclopcam/cyclops/pkg/videoformat/fsv"
	"github.com/cyclopcam/cyclops/server/camera"
	"github.com/cyclopcam/cyclops/server/configdb"
	"github.com/cyclopcam/cyclops/server/monitor"
	"github.com/cyclopcam/logs"
)

// LiveCameras manages the list of running cameras.
// It runs a single thread that is responsible for starting and stopping cameras.
// Coordination with other systems happens primarily through the configuration
// database. LiveCameras wakes up every few seconds to see if the current
// running cameras match the configuration. If anything is out of sync, then
// cameras are stopped, started, or restarted, as needed. This same system also
// detects when cameras have stopped sending us packets, in which case the camera
// is taken offline, and we will then attempt to restart it.
//
// Lock Hierarchy (to avoid deadlocks):
// If you are going to hold camerasLock and recordStateLock at the same time, then
// you must first take camerasLock, and then recordStateLock.
type LiveCameras struct {
	ShutdownComplete chan bool // ShutdownComplete is closed when we are done shutting down

	log            logs.Log
	configDB       *configdb.ConfigDB
	shutdown       chan bool // The parent system closes this channel when it wants us to shutdown
	monitor        *monitor.Monitor
	archive        *fsv.Archive // archive can be nil, in which case we can't record
	ringBufferSize int

	camerasLock  sync.Mutex
	cameraFromID map[int64]*camera.Camera

	wake chan bool // Used to wake up the auto starter

	periodicWakeInterval   time.Duration // Interval between auto wake up and reconnect cameras that have stopped sending packets
	timeUntilCameraRestart time.Duration // Wait this long for a camera to be silent, before restarting it
	closeTestCameraAfter   time.Duration // Close the test camera after this long

	// In order to speed up the UX sequence of Test Camera, Add Camera, we hang onto the most recently
	// tested camera. This prevents an often multi-second delay that the user would experience
	// when adding a new camera to the system. The first delay is the initial test connection.
	// The second (unnecessary) delay is when adding that camera. So we keep the initial connection
	// from the test alive, thereby eliminating the second unnecessary delay.
	lastTestedCameraLock      sync.Mutex // Guards access to lastTestedCameraXXX
	lastTestedCamera          *camera.Camera
	lastTestedCameraConfig    configdb.Camera
	lastTestedCameraCreatedAt time.Time

	////////////////////////////////////////////////////////////
	// Recording related fields
	allCameraMonitorMsg chan *monitor.AnalysisState

	// Controls access to recorderState
	recordStateLock      sync.Mutex
	recordStates         map[int64]*cameraRecordState // Map from CameraID to recordState
	recordThreadShutdown chan bool
	recordThreadWake     chan bool
}

// State of a camera, regarding recording on/off
type cameraRecordState struct {
	recorderHD    *camera.VideoRecorder // If non-nil, then we are recording high res (and vice versa)
	recorderLD    *camera.VideoRecorder // If non-nil, then we are recording low res (and vice versa)
	lastDetection time.Time             // Last time when Monitor sent us an event containing an object detection
}

func (r *cameraRecordState) isRecording() bool {
	// Right now we always record in LD and HD, but some day we might allow LD-only recording.
	// The HD recording cost is so high, that adding LD is such a small overhead that it seems
	// worthwhile always doing LD when doing HD (but not the other way around).
	return r.recorderLD != nil
}

// Create a new LiveCameras object.
// archive can be nil, in which case we can't record.
// shutdown is a channel that the parent system will close when it wants us to shutdown.
func NewLiveCameras(logger logs.Log, configDB *configdb.ConfigDB, shutdown chan bool, monitor *monitor.Monitor, archive *fsv.Archive, ringBufferSize int) *LiveCameras {
	lc := &LiveCameras{
		ShutdownComplete:       make(chan bool),
		log:                    logs.NewPrefixLogger(logger, "LiveCameras:"),
		configDB:               configDB,
		shutdown:               shutdown,
		monitor:                monitor,
		archive:                archive,
		ringBufferSize:         ringBufferSize,
		cameraFromID:           map[int64]*camera.Camera{},
		wake:                   make(chan bool, 50),
		periodicWakeInterval:   10 * time.Second,
		timeUntilCameraRestart: 5 * time.Second,
		closeTestCameraAfter:   60 * time.Second,
		allCameraMonitorMsg:    monitor.AddWatcherAllCameras(),
		recordThreadShutdown:   make(chan bool),
		recordThreadWake:       make(chan bool, 50),
		recordStates:           map[int64]*cameraRecordState{},
	}
	return lc
}

// Start the runner thread, which is the only thread that starts and stops cameras
func (s *LiveCameras) Run() {
	go s.runThread()
	go s.recorderThread()
	s.wake <- true
}

// If any configuration might have changed, then wake up the auto starter
func (s *LiveCameras) ConfigurationChanged() {
	s.wake <- true
}

func (s *LiveCameras) CameraAdded(id int64) {
	s.wake <- true
}

func (s *LiveCameras) CameraChanged(id int64) {
	s.wake <- true
}

func (s *LiveCameras) CameraRemoved(id int64) {
	s.wake <- true
}

// Get camera from ID
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
func (s *LiveCameras) addCamera(cam *camera.Camera) {
	s.log.Infof("Adding camera %v", cam.ID())
	s.camerasLock.Lock()
	defer s.camerasLock.Unlock()
	s.cameraFromID[cam.ID()] = cam
	s.monitor.SetCameras(s.cameraListNoLock())
}

// Remove a running camera
// If the camera does not exist, then the function returns immediately
func (s *LiveCameras) removeCamera(cam *camera.Camera) {
	s.log.Infof("Removing camera %v", cam.ID())
	s.camerasLock.Lock()
	defer s.camerasLock.Unlock()
	delete(s.cameraFromID, cam.ID())
	s.monitor.SetCameras(s.cameraListNoLock())
	cam.Close(nil)
}

func (s *LiveCameras) CloseTestCamera() {
	s.lastTestedCameraLock.Lock()
	defer s.lastTestedCameraLock.Unlock()
	if s.lastTestedCamera != nil {
		s.log.Infof("Explicit closing test camera to '%v'", s.lastTestedCameraConfig.Host)
		s.lastTestedCamera.Close(nil)
		s.lastTestedCamera = nil
	}
}

func (s *LiveCameras) SaveTestCamera(cfg configdb.Camera, cam *camera.Camera) {
	s.log.Infof("Saving test camera to '%v'", cfg.Host)
	s.lastTestedCameraLock.Lock()
	defer s.lastTestedCameraLock.Unlock()
	if s.lastTestedCamera != nil {
		s.lastTestedCamera.Close(nil)
	}
	s.lastTestedCamera = cam
	s.lastTestedCameraConfig = cfg
	s.lastTestedCameraCreatedAt = time.Now()
}

// Run the camera auto-starter, which runs continuously in the backround,
// making sure we can reach all of the cameras.
// This is the only thread that starts and stops cameras, with the one exception
// of the camera testing function httpConfigTestCamera.
func (s *LiveCameras) runThread() {
	keepRunning := true
	for keepRunning {
		select {
		case <-time.After(s.periodicWakeInterval):
			s.startStopConfiguredCameras()
		case <-s.wake:
			s.startStopConfiguredCameras()
		case <-s.shutdown:
			// Note that we don't yet call Close(). This is just a legacy ordering thing,
			// from the way that the main Server.Shutdown() was implemented. Conceptually, it should
			// be OK to shutdown cameras here, because cameras should be removable from the system at
			// any time. Although, at present, having an explicit Close() is nice because it allows
			// the main Shutdown to wait for the sinks to drain. But I agree with my former self
			// that it should be possible to do these things in any order.
			keepRunning = false
			break
		}
	}
	s.log.Infof("Shutting down")

	s.monitor.RemoveWatcherAllCameras(s.allCameraMonitorMsg)

	s.log.Infof("Waiting for test camera to close")

	s.CloseTestCamera()

	s.log.Infof("Waiting for cameras to close")

	wg := sync.WaitGroup{}
	s.camerasLock.Lock()
	for _, cam := range s.cameraFromID {
		cam.Close(&wg)
	}
	s.camerasLock.Unlock()
	wg.Wait()

	s.log.Infof("Waiting for record thread to shutdown")

	<-s.recordThreadShutdown

	s.log.Infof("Shutdown complete")

	close(s.ShutdownComplete)
}

// This runs in the background every few seconds, invoked by the auto start thread.
func (s *LiveCameras) startStopConfiguredCameras() {
	// drain the wake channel, so that we can be responsive to any incoming wake messages
	for len(s.wake) > 0 {
		<-s.wake
	}

	configs := []*configdb.Camera{}
	if err := s.configDB.DB.Find(&configs).Error; err != nil {
		s.log.Errorf("Error loading cameras from config: %v", err)
		return
	}
	// Sort by most recently updated
	sort.Slice(configs, func(i, j int) bool {
		return configs[i].UpdatedAt > configs[j].UpdatedAt
	})

	// Close the last tested camera if timeout has expired
	s.lastTestedCameraLock.Lock()
	if s.lastTestedCamera != nil && time.Now().Sub(s.lastTestedCameraCreatedAt) > s.closeTestCameraAfter {
		s.log.Infof("Closing last tested camera '%v' because timeout has expired", s.lastTestedCameraConfig.Host)
		s.lastTestedCamera.Close(nil)
		s.lastTestedCamera = nil
	}
	s.lastTestedCameraLock.Unlock()

	// Close cameras that are no longer in our database
	stopList := []*camera.Camera{}
	cfgIDs := map[int64]bool{}
	for _, cfg := range configs {
		cfgIDs[cfg.ID] = true
	}
	for _, cam := range s.Cameras() {
		if _, found := cfgIDs[cam.ID()]; !found {
			stopList = append(stopList, cam)
		}
	}
	for _, cam := range stopList {
		s.log.Infof("Stopping camera %v (%v), because it's no longer configured", cam.ID(), cam.Name())
		cam.Close(nil)
		s.removeCamera(cam)
	}

	for _, cfg := range configs {
		// Why abort if there is a wake message?
		// Let's say we have a big system with 10 cameras, and a few of them are timing out and
		// not connecting. Maybe some of their IPs have changed.
		// The user goes into the config page, and enters the correct IP, and hits Save.
		// At that moment, we'll get a wake message. We want to abandon our previous loop that was
		// going through all those invalid IPs, and immediately start the new camera.
		// That's why we:
		// 1. Sort by most recently updated.
		// 2. Restart this function whenever we receive a wake message.
		if s.isShuttingDown() || len(s.wake) > 0 {
			break
		}

		if cam := s.CameraFromID(cfg.ID); cam != nil {
			if time.Now().Sub(cam.LastPacketAt()) > s.timeUntilCameraRestart {
				s.log.Warnf("Camera %v (%v) unresponsive. Restarting", cfg.ID, cfg.Name)
			} else if !cam.Config.DeepEquals(cfg) {
				// Initially my criteria here for restarting a camera was Camera.EqualsConnection().
				// However, when adding the concept of the long-lived name, and the VideoDB, I changed that,
				// because the VideoDB needs to know the long-lived name when writing events to the DB.
				// I debated hiding Camera.Config behind a mutex, so that we could update the config
				// of a running camera, but that seemed way too messy. Simply restarting the camera
				// is much cleaner.
				s.log.Warnf("Camera %v (%v) configuration out of date. Restarting", cfg.ID, cfg.Name)
			} else {
				continue
			}
			s.removeCamera(cam)
		}
		s.log.Infof("Starting camera %v (%v)", cfg.ID, cfg.Name)
		s.lastTestedCameraLock.Lock()
		var cam *camera.Camera
		if s.lastTestedCamera != nil && s.lastTestedCameraConfig.EqualsConnection(cfg) {
			s.log.Infof("Success using last tested camera '%v'", s.lastTestedCameraConfig.Host)
			cam = s.lastTestedCamera
			s.lastTestedCamera = nil
			cam.Config = *cfg // Update initial test config to final config in DB (which includes, at the very least, the camera ID and long-lived name)
		}
		s.lastTestedCameraLock.Unlock()

		if cam == nil {
			var err error
			if cam, err = camera.NewCamera(s.log, *cfg, s.ringBufferSize); err != nil {
				s.log.Errorf("Error creating camera %v (%v): %v", cfg.ID, cfg.Name, err)
			} else {
				if err = cam.Start(); err != nil {
					s.log.Errorf("Error starting camera %v (%v): %v", cfg.ID, cfg.Name, err)
					cam.Close(nil)
					cam = nil
				} else {
					s.log.Infof("Started camera %v (%v) successfully", cfg.ID, cfg.Name)
				}
			}
		}
		if cam != nil {
			s.addCamera(cam)
		}
	}
}

// Returns true if the system wants us to shutdown
func (s *LiveCameras) isShuttingDown() bool {
	select {
	case <-s.shutdown:
		return true
	default:
		return false
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
