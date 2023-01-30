package monitor

import (
	"sync/atomic"
	"time"

	"github.com/bmharper/cyclops/pkg/log"
	"github.com/bmharper/cyclops/server/camera"
	"github.com/bmharper/cyclops/server/ncnn"
	"github.com/bmharper/cyclops/server/nn"
)

// monitor runs our neural networks on the camera streams

type Monitor struct {
	Log           log.Log
	detector      nn.ObjectDetector
	mustStop      atomic.Bool      // True if Stop() has been called
	looperStopped chan bool        // When looperStop channel is closed, then the looped has stopped
	cameras       []*monitorCamera // Cameras that we're monitoring

	//isPaused  atomic.Int32
}

type monitorCamera struct {
	camera  *camera.Camera
	objects []nn.Detection
}

func NewMonitor(logger log.Log) (*Monitor, error) {
	detector, err := ncnn.NewDetector("yolov7", "models/yolov7-tiny.param", "models/yolov7-tiny.bin")
	//detector, err := ncnn.NewDetector("yolov7", "/home/ben/dev/cyclops/models/yolov7-tiny.param", "/home/ben/dev/cyclops/models/yolov7-tiny.bin")
	if err != nil {
		return nil, err
	}
	m := &Monitor{
		Log:      logger,
		detector: detector,
	}
	m.start()
	return m, nil
}

// Close the monitor object.
func (m *Monitor) Close() {
	m.Log.Infof("Monitor shutting down")
	m.stop()
	m.detector.Close()
	m.Log.Infof("Monitor is closed")
}

// Stop listening to cameras
func (m *Monitor) stop() {
	m.mustStop.Store(true)
	<-m.looperStopped
}

// Start/Restart looper
func (m *Monitor) start() {
	m.mustStop.Store(false)
	m.looperStopped = make(chan bool)
	go m.loop()
}

// Set cameras and start monitoring
func (m *Monitor) SetCameras(cameras []*camera.Camera) {
	m.stop()
	m.cameras = nil
	for _, cam := range cameras {
		m.cameras = append(m.cameras, &monitorCamera{
			camera: cam,
		})
	}
	m.start()
}

// Loop runs until Close()
func (m *Monitor) loop() {
	lastErrAt := time.Time{}

	for !m.mustStop.Load() {
		time.Sleep(50 * time.Millisecond)

		for _, mcam := range m.cameras {
			if m.mustStop.Load() {
				break
			}
			img := mcam.camera.LowDecoder.LastImage()
			if img != nil {
				objects, err := m.detector.DetectObjects(img.NChan(), img.Pixels, img.Width, img.Height)
				if err != nil {
					if time.Now().Sub(lastErrAt) > 15*time.Second {
						m.Log.Errorf("Error detecting objects: %v", err)
						lastErrAt = time.Now()
					}
				} else {
					m.Log.Infof("Camera %v detected %v objects", mcam.camera.ID, len(objects))
					mcam.objects = objects
				}
			}
		}
	}
	close(m.looperStopped)
}

/*
// Pause any monitoring activity.
// Pause/Unpause is a counter, so for every call to Pause(), you must make an equivalent call to Unpause().
func (m *Monitor) Pause() {
	m.isPaused.Add(1)
}

// Reverse the action of one or more calls to Pause().
// Every call to Pause() must be matched by a call to Unpause().
func (m *Monitor) Unpause() {
	nv := m.isPaused.Add(-1)
	if nv < 0 {
		m.Log.Errorf("Monitor isPaused is negative. This is a bug")
	}
}
*/
