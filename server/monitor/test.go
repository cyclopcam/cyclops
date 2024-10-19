package monitor

import (
	"time"

	"github.com/cyclopcam/cyclops/pkg/accel"
)

// Functions used by unit tests

// Create a fake camera for unit tests to reference
func (m *Monitor) InjectTestCamera() {
	m.camerasLock.Lock()
	cam := &monitorCamera{
		camera: nil,
	}
	m.cameras = append(m.cameras, cam)
	m.camerasLock.Unlock()
}

// Inject a frame for NN analysis, for use by unit tests
func (m *Monitor) InjectTestFrame(cameraIndex int, pts time.Time, img *accel.YUVImage) {
	m.camerasLock.Lock()
	camera := m.cameras[cameraIndex]
	m.camerasLock.Unlock()

	qitem := monitorQueueItem{
		monCam:   camera,
		image:    img,
		framePTS: pts,
	}
	m.nnThreadQueue <- qitem
}
