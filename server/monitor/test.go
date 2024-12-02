package monitor

import (
	"fmt"
	"time"

	"github.com/cyclopcam/cyclops/pkg/accel"
	"github.com/cyclopcam/cyclops/server/camera"
	"github.com/cyclopcam/cyclops/server/configdb"
)

// Functions used by unit tests

// Create a fake camera for unit tests to reference
func (m *Monitor) InjectTestCamera() {
	m.camerasLock.Lock()
	id := len(m.cameras) + 1
	fakeCamera := &camera.Camera{
		Config: configdb.Camera{
			LongLivedName: fmt.Sprintf("cam%v", id),
		},
	}
	fakeCamera.Config.ID = int64(id)
	cam := &monitorCamera{
		camera: fakeCamera,
	}
	m.cameras = append(m.cameras, cam)
	m.camerasLock.Unlock()
}

// Inject a frame for NN analysis, for use by unit tests
func (m *Monitor) InjectTestFrame(cameraIndex int, imgID int64, pts time.Time, img *accel.YUVImage) {
	m.camerasLock.Lock()
	camera := m.cameras[cameraIndex]
	m.camerasLock.Unlock()

	qitem := monitorQueueItem{
		isHQ:     false,
		imgID:    imgID,
		monCam:   camera,
		yuv:      img,
		rgb:      nil,
		framePTS: pts,
	}
	m.nnThreadQueue <- qitem
}
