package monitor

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bmharper/cimg/v2"
	"github.com/bmharper/cyclops/pkg/accel"
	"github.com/bmharper/cyclops/pkg/gen"
	"github.com/bmharper/cyclops/pkg/log"
	"github.com/bmharper/cyclops/server/camera"
	"github.com/bmharper/cyclops/server/ncnn"
	"github.com/bmharper/cyclops/server/nn"
)

// monitor runs our neural networks on the camera streams

type Monitor struct {
	Log                 log.Log
	detector            nn.ObjectDetector
	enabled             bool                  // If false, then we don't run the looper
	mustStopLooper      atomic.Bool           // True if stopLooper() has been called
	mustStopNNThreads   atomic.Bool           // NN threads must exit
	numNNThreads        int                   // Number of NN threads
	nnThreadStopWG      sync.WaitGroup        // Wait for all NN threads to exit
	looperStopped       chan bool             // When looperStop channel is closed, then the looped has stopped
	nnFrameTime         time.Duration         // Average time for the neural network to process a frame
	nnThreadQueue       chan monitorQueueItem // Queue of images to be processed by the neural network
	avgTimeNSPerFrameNN atomic.Int64          // Average time (ns) per frame, for just the neural network (time inside a thread)

	camerasLock sync.Mutex       // Guards access to cameras
	cameras     []*monitorCamera // Cameras that we're monitoring

	watchersLock sync.RWMutex // Guards access to watchers
	watchers     []watcher    // Channels to send detection results to
}

type watcher struct {
	cameraID int64
	ch       chan *nn.DetectionResult
}

type monitorCamera struct {
	camera *camera.Camera

	// Guards access to lastImg and objects
	lock sync.Mutex

	// Guarded by 'lock' mutex.
	// If objects is not nil, then this is the image that was used to generate the objects.
	// lastImg is garbage collected - it will not get reused for subsequent frames.
	// In other words, it is safe to lock the mutex, read the lastImg pointer, unlock the mutex,
	// and then use that pointer indefinitely thereafter.
	lastImg *cimg.Image

	// Guarded by 'lock' mutex.
	// Same comment applies to objects as to lastImg, in the sense that the contents of objects is immutable.
	lastDetection *nn.DetectionResult
}

type monitorQueueItem struct {
	camera *monitorCamera
	image  *accel.YUVImage
}

func NewMonitor(logger log.Log) (*Monitor, error) {
	tryPaths := []string{"models", "/var/lib/cyclops/models"}
	basePath := ""
	for _, tryPath := range tryPaths {
		abs, err := filepath.Abs(tryPath)
		if err != nil {
			logger.Warnf("Unable to resolve model path candidate '%v' to an absolute path: %v", tryPath, err)
			continue
		}
		if _, err := os.Stat(filepath.Join(abs, "yolov7-tiny.param")); err == nil {
			basePath = abs
			break
		}
	}
	if basePath == "" {
		return nil, fmt.Errorf("Could not find models directory. Searched in [%v]", strings.Join(tryPaths, ", "))
	}
	logger.Infof("Loading NN models from '%v'", basePath)

	detector, err := ncnn.NewDetector("yolov7", filepath.Join(basePath, "yolov7-tiny.param"), filepath.Join(basePath, "yolov7-tiny.bin"))
	//detector, err := ncnn.NewDetector("yolov7", "/home/ben/dev/cyclops/models/yolov7-tiny.param", "/home/ben/dev/cyclops/models/yolov7-tiny.bin")
	if err != nil {
		return nil, err
	}

	// queueSize should be at least equal to nThreads, otherwise we'll never reach full utilization.
	// But perhaps we can use queueSize as a throttle, to optimize the number of active threads.
	// It's not clear yet how many threads is optimal.
	// One more important point:
	// queueSize must be at least twice the size of nThreads, so that our exit mechanism can work.
	// Once we signal mustStopNNThreads, we fill the queue with dummy jobs, so that the NN threads
	// can wake up from their channel receive operation, and exit.
	// If the queue size was too small, then this would deadlock.

	// On a Raspberry Pi 4, a single NN thread is best. But on my larger desktops, more threads helps.
	// I haven't looked into ncnn's threading strategy yet.
	nThreads := int(math.Max(1, float64(runtime.NumCPU())/4))
	queueSize := nThreads * 3

	logger.Infof("Starting %v NN detection threads", nThreads)

	m := &Monitor{
		Log:           logger,
		detector:      detector,
		nnThreadQueue: make(chan monitorQueueItem, queueSize),
		numNNThreads:  nThreads,
		enabled:       true,
	}
	for i := 0; i < m.numNNThreads; i++ {
		go m.nnThread()
	}
	if m.enabled {
		m.startLooper()
	}
	return m, nil
}

// Close the monitor object.
func (m *Monitor) Close() {
	m.Log.Infof("Monitor shutting down")

	// Stop reading images from cameras
	if m.enabled {
		m.stopLooper()
	}

	// Stop NN threads
	m.Log.Infof("Monitor waiting for NN threads")
	m.nnThreadStopWG.Add(m.numNNThreads)
	m.mustStopNNThreads.Store(true)
	for i := 0; i < m.numNNThreads; i++ {
		m.nnThreadQueue <- monitorQueueItem{}
	}
	m.nnThreadStopWG.Wait()

	// Close the C++ object
	m.detector.Close()

	m.Log.Infof("Monitor is closed")
}

// Return the most recent frame and detection result for a camera
func (m *Monitor) LatestFrame(cameraID int64) (*cimg.Image, *nn.DetectionResult, error) {
	cam := m.cameraByID(cameraID)
	if cam == nil {
		return nil, nil, fmt.Errorf("Camera %v not found", cameraID)
	}
	cam.lock.Lock()
	defer cam.lock.Unlock()
	if cam.lastImg == nil {
		return nil, nil, fmt.Errorf("No image available for camera %v", cameraID)
	}

	return cam.lastImg, cam.lastDetection, nil
}

// Register to receive detection results.
// You must be careful to ensure that your receiver always processes a result
// immediately, and keeps the channel drained. If you don't do this, then
// the monitor will freeze, and obviously that's a really bad thing to happen
// to a security system.
func (m *Monitor) AddWatcher(cameraID int64) chan *nn.DetectionResult {
	m.watchersLock.Lock()
	defer m.watchersLock.Unlock()
	ch := make(chan *nn.DetectionResult, 100)
	m.watchers = append(m.watchers, watcher{
		cameraID: cameraID,
		ch:       ch,
	})
	return ch
}

// Unregister from detection results
func (m *Monitor) RemoveWatcher(ch chan *nn.DetectionResult) {
	m.watchersLock.Lock()
	defer m.watchersLock.Unlock()
	for i, w := range m.watchers {
		if w.ch == ch {
			m.watchers = gen.DeleteFromSliceUnordered(m.watchers, i)
			return
		}
	}
	m.Log.Warnf("Monitor.RemoveWatcher failed to find channel")
}

func (m *Monitor) cameraByID(cameraID int64) *monitorCamera {
	m.camerasLock.Lock()
	defer m.camerasLock.Unlock()
	for _, cam := range m.cameras {
		if cam.camera.ID == cameraID {
			return cam
		}
	}
	return nil
}

// Stop listening to cameras
func (m *Monitor) stopLooper() {
	m.mustStopLooper.Store(true)
	<-m.looperStopped
}

// Start/Restart looper
func (m *Monitor) startLooper() {
	m.mustStopLooper.Store(false)
	m.looperStopped = make(chan bool)
	go m.loop()
}

// Set cameras and start monitoring
func (m *Monitor) SetCameras(cameras []*camera.Camera) {
	if m.enabled {
		m.stopLooper()
	}

	newCameras := []*monitorCamera{}
	for _, cam := range cameras {
		newCameras = append(newCameras, &monitorCamera{
			camera: cam,
		})
	}

	m.camerasLock.Lock()
	m.cameras = newCameras
	m.camerasLock.Unlock()

	if m.enabled {
		m.startLooper()
	}
}

type looperCameraState struct {
	mcam               *monitorCamera
	lastFrameID        int64 // Last frame we've seen from this camera
	numFramesTotal     int64 // Number of frames from this camera that we've seen
	numFramesProcessed int64 // Number of frames from this camera that we've analyzed
}

func looperStats(cameraStates []*looperCameraState) (totalFrames, totalProcessed int64) {
	for _, state := range cameraStates {
		totalFrames += state.numFramesTotal
		totalProcessed += state.numFramesProcessed
	}
	return
}

// Loop runs until Close()
func (m *Monitor) loop() {
	// Make our own private copy of cameras.
	// If the list of cameras changes, then SetCameras() will stop and restart the looper.
	m.camerasLock.Lock()
	looperCameras := []*looperCameraState{}
	for _, mcam := range m.cameras {
		looperCameras = append(looperCameras, &looperCameraState{
			mcam: mcam,
		})
	}
	m.camerasLock.Unlock()

	// Maintain camera index outside of main loop, so that we're not
	// biased towards processing the frames of the first camera(s).
	// I still need to figure out how to boost priority for cameras
	// that have likely activity in them.
	icam := uint(0)

	lastStats := time.Now()

	nStats := 0
	for !m.mustStopLooper.Load() {
		idle := true
		for i := 0; i < len(looperCameras); i++ {
			if m.mustStopLooper.Load() {
				break
			}
			if len(m.nnThreadQueue) >= 2*cap(m.nnThreadQueue)/3 {
				continue
			}

			// It's vital that this incrementing happens after the queue check above,
			// otherwise you don't get round robin behaviour.
			icam = (icam + 1) % uint(len(looperCameras))
			camState := looperCameras[icam]
			mcam := camState.mcam

			//m.Log.Infof("%v", icam)
			img, imgID := mcam.camera.LowDecoder.GetLastImageIfDifferent(camState.lastFrameID)
			if img != nil {
				if camState.lastFrameID == 0 {
					camState.numFramesTotal++
				} else {
					camState.numFramesTotal += imgID - camState.lastFrameID
				}
				//m.Log.Infof("Got image %d from camera %s (%v / %v)", imgID, mcam.camera.Name, camState.numFramesProcessed, camState.numFramesTotal)
				camState.numFramesProcessed++
				camState.lastFrameID = imgID
				idle = false
				m.nnThreadQueue <- monitorQueueItem{
					camera: mcam,
					image:  img,
				}
			}
		}
		if m.mustStopLooper.Load() {
			break
		}
		if idle {
			time.Sleep(5 * time.Millisecond)
		}

		interval := 3 * math.Pow(1.05, float64(nStats))
		if interval > 5*60 {
			interval = 5 * 60
		}
		if time.Now().Sub(lastStats) > time.Duration(interval)*time.Second {
			nStats++
			totalFrames, totalProcessed := looperStats(looperCameras)
			m.Log.Infof("%.0f%% frames analyzed by NN (%.1f ms per frame, per thread)", 100*float64(totalProcessed)/float64(totalFrames), float64(m.avgTimeNSPerFrameNN.Load())/1e6)
			lastStats = time.Now()
		}
	}
	close(m.looperStopped)
}

// An NN processing thread
func (m *Monitor) nnThread() {
	lastErrAt := time.Time{}
	var rgb *cimg.Image

	for {
		item, ok := <-m.nnThreadQueue
		if !ok || m.mustStopNNThreads.Load() {
			break
		}
		yuv := item.image
		if rgb == nil || rgb.Width != yuv.Width || rgb.Height != yuv.Height {
			rgb = cimg.NewImage(yuv.Width, yuv.Height, cimg.PixelFormatRGB)
		}
		yuv.CopyToCImageRGB(rgb)
		start := time.Now()
		objects, err := m.detector.DetectObjects(rgb.NChan(), rgb.Pixels, rgb.Width, rgb.Height)
		duration := time.Now().Sub(start)
		m.avgTimeNSPerFrameNN.Store((99*m.avgTimeNSPerFrameNN.Load() + duration.Nanoseconds()) / 100)
		if err != nil {
			if time.Now().Sub(lastErrAt) > 15*time.Second {
				m.Log.Errorf("Error detecting objects: %v", err)
				lastErrAt = time.Now()
			}
		} else {
			//m.Log.Infof("Camera %v detected %v objects", mcam.camera.ID, len(objects))
			result := &nn.DetectionResult{
				CameraID:    item.camera.camera.ID,
				ImageWidth:  yuv.Width,
				ImageHeight: yuv.Height,
				Objects:     objects,
			}
			item.camera.lock.Lock()
			item.camera.lastDetection = result
			item.camera.lastImg = rgb
			item.camera.lock.Unlock()

			m.watchersLock.RLock()
			for _, watcher := range m.watchers {
				if watcher.cameraID == item.camera.camera.ID {
					if len(watcher.ch) >= cap(watcher.ch)*9/10 {
						// This should never happen. But as a safeguard against a monitor deadlock, we choose to drop frames.
						m.Log.Warnf("NN detection watcher on camera %v is falling behind. I am going to drop frames.", watcher.cameraID)
					} else {
						watcher.ch <- result
					}
				}
			}
			m.watchersLock.RUnlock()
		}
	}

	m.nnThreadStopWG.Done()
}
