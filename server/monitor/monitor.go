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
	"github.com/cyclopcam/cyclops/pkg/accel"
	"github.com/cyclopcam/cyclops/pkg/gen"
	"github.com/cyclopcam/cyclops/pkg/log"
	"github.com/cyclopcam/cyclops/pkg/nn"
	"github.com/cyclopcam/cyclops/pkg/nnload"
	"github.com/cyclopcam/cyclops/server/camera"
)

/* monitor runs our neural networks on the camera streams

We process camera frames in phases:
1. Read frames from cameras (frameReader)
2. Process frames with neural networks (nnThread)
3. Analyze results from neural networks (analyzer)

We connect these phases with channels.

*/

type Monitor struct {
	Log                 log.Log
	detector            nn.ObjectDetector
	enabled             bool                   // If false, then we don't run the frame reader
	mustStopFrameReader atomic.Bool            // True if stopFrameReader() has been called
	mustStopNNThreads   atomic.Bool            // NN threads must exit
	analyzerQueue       chan analyzerQueueItem // Analyzer work queue. When closed, analyzer must exit.
	analyzerStopped     chan bool              // Analyzer thread has exited
	numNNThreads        int                    // Number of NN threads
	nnThreadStopWG      sync.WaitGroup         // Wait for all NN threads to exit
	frameReaderStopped  chan bool              // When frameReaderStopped channel is closed, then the frame reader has stopped
	nnFrameTime         time.Duration          // Average time for the neural network to process a frame
	nnThreadQueue       chan monitorQueueItem  // Queue of images to be processed by the neural network
	avgTimeNSPerFrameNN atomic.Int64           // Average time (ns) per frame, for just the neural network (time inside a thread)
	cocoClassFilter     map[int]bool           // COCO classes that we're interested in
	analyzerSettings    analyzerSettings       // Analyzer settings
	nextTrackedObjectID atomic.Int64           // Next ID to assign to a tracked object

	camerasLock sync.Mutex       // Guards access to cameras
	cameras     []*monitorCamera // Cameras that we're monitoring

	watchersLock sync.RWMutex // Guards access to watchers
	watchers     []watcher    // Channels to send detection results to
}

type watcher struct {
	cameraID int64
	ch       chan *AnalysisState
}

// monitoCamera is the internal data structure for managing a single camera that we are monitoring
type monitorCamera struct {
	camera *camera.Camera

	// Guards access to lastImg, lastDetection, analyzerState
	lock sync.Mutex

	// Guarded by 'lock' mutex.
	// If lastDetection is not nil, then this is the image that was used to generate the objects.
	// lastImg is garbage collected - it will not get reused for subsequent frames.
	// In other words, it is safe to lock the mutex, read the lastImg pointer, unlock the mutex,
	// and then use that pointer indefinitely thereafter.
	lastImg *cimg.Image

	// Guarded by 'lock' mutex.
	// Same comment applies here as to lastImg, in the sense that the contents of this object is immutable.
	lastDetection *nn.DetectionResult

	// Guared by 'lock' mutex.
	// Can be nil.
	// Same comment applies here as to lastImg, in the sense that the contents of this object is immutable.
	analyzerState *AnalysisState
}

type monitorQueueItem struct {
	camera *monitorCamera
	image  *accel.YUVImage
}

type analyzerQueueItem struct {
	camera    *monitorCamera
	detection *nn.DetectionResult
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
		if _, err := os.Stat(filepath.Join(abs, "yolov8s.json")); err == nil {
			basePath = abs
			break
		}
	}
	if basePath == "" {
		return nil, fmt.Errorf("Could not find models directory. Searched in [%v]", strings.Join(tryPaths, ", "))
	}
	logger.Infof("Loading NN models from '%v'", basePath)

	// On a Raspberry Pi 4, a single NN thread is best. But on my larger desktops, more threads helps.
	// I have some numbers in a spreadsheet. Basically, on a Pi, we want to have all cores processing
	// a single image at a time. But on desktop CPUs, we want one core per image.
	numCPU := runtime.NumCPU()
	nnThreads := int(math.Max(1, float64(numCPU)/4))
	nnThreadingModel := nn.ThreadingModeSingle
	if nnThreads == 1 {
		// If we're only running a single detection thread, then let the NN library use however
		// many cores it can.
		nnThreadingModel = nn.ThreadingModeParallel
	}

	// nnQueueSize should be at least equal to nnThreads, otherwise we'll never reach full utilization.
	// But perhaps we can use nnQueueSize as a throttle, to optimize the number of active threads.
	// One important point:
	// queueSize must be at least twice the size of nnThreads, so that our exit mechanism can work.
	// Once we signal mustStopNNThreads, we fill the queue with dummy jobs, so that the NN threads
	// can wake up from their channel receive operation, and exit.
	// If the queue size was too small, then this would deadlock.
	// nnQueueSize should not be less than 1, otherwise our backoff mechanism will never allow a
	// frame through.
	// SYNC-NN-THREAD-QUEUE-MIN-SIZE
	nnQueueSize := nnThreads * 3

	detector, err := nnload.LoadModel(filepath.Join(basePath, "yolov8s"), nnThreadingModel)
	//detector, err := ncnn.NewDetector("yolov7", filepath.Join(basePath, "yolov7-tiny.param"), filepath.Join(basePath, "yolov7-tiny.bin"), 320, 256)
	//detector, err := ncnn.NewDetector("yolov7", "/home/ben/dev/cyclops/models/yolov7-tiny.param", "/home/ben/dev/cyclops/models/yolov7-tiny.bin")
	if err != nil {
		return nil, err
	}

	// No idea what a good number is here. I expect analysis to be much
	// faster to run than NN, so provided this queue is large enough to
	// prevent bumps, it shouldn't matter too much.
	analysisQueueSize := 20

	logger.Infof("Starting %v NN detection threads", nnThreads)

	m := &Monitor{
		Log:             logger,
		detector:        detector,
		nnThreadQueue:   make(chan monitorQueueItem, nnQueueSize),
		analyzerQueue:   make(chan analyzerQueueItem, analysisQueueSize),
		analyzerStopped: make(chan bool),
		numNNThreads:    nnThreads,
		cocoClassFilter: cocoFilter(),
		analyzerSettings: analyzerSettings{
			positionHistorySize:       30,   // at 10 fps, 30 frames = 3 seconds
			maxAnalyzeObjectsPerFrame: 20,   // We have O(n^2) analysis functions, so we need to keep this small.
			minDistanceForObject:      0.05, // 5% of the frame width (0.05 * 320 = 16 pixels)
			minDiscreetPositions:      10,
			objectForgetTime:          5 * time.Second,
			verbose:                   false,
		},
		enabled: true,
	}
	for i := 0; i < m.numNNThreads; i++ {
		go m.nnThread()
	}
	if m.enabled {
		m.startFrameReader()
	}
	go m.analyzer()

	return m, nil
}

// Close the monitor object.
func (m *Monitor) Close() {
	m.Log.Infof("Monitor shutting down")

	// Stop reading images from cameras
	if m.enabled {
		m.stopFrameReader()
	}

	// Stop NN threads
	m.Log.Infof("Monitor waiting for NN threads")
	m.nnThreadStopWG.Add(m.numNNThreads)
	m.mustStopNNThreads.Store(true)
	for i := 0; i < m.numNNThreads; i++ {
		m.nnThreadQueue <- monitorQueueItem{}
	}
	m.nnThreadStopWG.Wait()

	// Stop analyzer
	m.Log.Infof("Monitor waiting for analyzer")
	close(m.analyzerQueue)

	// Close the C++ object
	m.detector.Close()

	m.Log.Infof("Monitor is closed")
}

// Return the most recent frame and detection result for a camera
func (m *Monitor) LatestFrame(cameraID int64) (*cimg.Image, *nn.DetectionResult, *AnalysisState, error) {
	cam := m.cameraByID(cameraID)
	if cam == nil {
		return nil, nil, nil, fmt.Errorf("Camera %v not found", cameraID)
	}
	cam.lock.Lock()
	defer cam.lock.Unlock()
	if cam.lastImg == nil {
		return nil, nil, nil, fmt.Errorf("No image available for camera %v", cameraID)
	}

	return cam.lastImg, cam.lastDetection, cam.analyzerState, nil
}

// Register to receive detection results.
// You must be careful to ensure that your receiver always processes a result
// immediately, and keeps the channel drained. If you don't do this, then
// the monitor will freeze, and obviously that's a really bad thing to happen
// to a security system.
func (m *Monitor) AddWatcher(cameraID int64) chan *AnalysisState {
	m.watchersLock.Lock()
	defer m.watchersLock.Unlock()
	ch := make(chan *AnalysisState, 100)
	m.watchers = append(m.watchers, watcher{
		cameraID: cameraID,
		ch:       ch,
	})
	return ch
}

// Unregister from detection results
func (m *Monitor) RemoveWatcher(ch chan *AnalysisState) {
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

func (m *Monitor) sendToWatchers(state *AnalysisState) {
	m.watchersLock.RLock()
	for _, watcher := range m.watchers {
		if watcher.cameraID == state.CameraID {
			if len(watcher.ch) >= cap(watcher.ch)*9/10 {
				// This should never happen. But as a safeguard against a monitor deadlock, we choose to drop frames.
				m.Log.Warnf("Monitor watcher on camera %v is falling behind. I am going to drop frames.", watcher.cameraID)
			} else {
				watcher.ch <- state
			}
		}
	}
	m.watchersLock.RUnlock()
}

func cocoFilter() map[int]bool {
	classes := []int{nn.COCOPerson, nn.COCOBicycle, nn.COCOCar, nn.COCOBus, nn.COCOMotorcycle, nn.COCOTruck}
	r := map[int]bool{}
	for _, c := range classes {
		r[c] = true
	}
	return r
}

func (m *Monitor) cameraByID(cameraID int64) *monitorCamera {
	m.camerasLock.Lock()
	defer m.camerasLock.Unlock()
	for _, cam := range m.cameras {
		if cam.camera.ID() == cameraID {
			return cam
		}
	}
	return nil
}

// Stop listening to cameras
func (m *Monitor) stopFrameReader() {
	m.mustStopFrameReader.Store(true)
	<-m.frameReaderStopped
}

// Start/Restart frame reader
func (m *Monitor) startFrameReader() {
	m.mustStopFrameReader.Store(false)
	m.frameReaderStopped = make(chan bool)
	go m.readFrames()
}

// Set cameras and start monitoring
func (m *Monitor) SetCameras(cameras []*camera.Camera) {
	// Stopping and starting the frame reader is the simplest solution to prevent
	// race conditions, but we could probably make this process more seamless, and
	// not have to stop the world whenever cameras are changed.
	if m.enabled {
		m.stopFrameReader()
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
		m.startFrameReader()
	}
}

type frameReaderCameraState struct {
	mcam               *monitorCamera
	lastFrameID        int64 // Last frame we've seen from this camera
	numFramesTotal     int64 // Number of frames from this camera that we've seen
	numFramesProcessed int64 // Number of frames from this camera that we've analyzed
}

func frameReaderStats(cameraStates []*frameReaderCameraState) (totalFrames, totalProcessed int64) {
	for _, state := range cameraStates {
		totalFrames += state.numFramesTotal
		totalProcessed += state.numFramesProcessed
	}
	return
}

// Read camera frames and send them off for analysis
func (m *Monitor) readFrames() {
	// Make our own private copy of cameras.
	// If the list of cameras changes, then SetCameras() will stop and restart this function
	m.camerasLock.Lock()
	looperCameras := []*frameReaderCameraState{}
	for _, mcam := range m.cameras {
		looperCameras = append(looperCameras, &frameReaderCameraState{
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
	for !m.mustStopFrameReader.Load() {
		idle := true
		for i := 0; i < len(looperCameras); i++ {
			if m.mustStopFrameReader.Load() {
				break
			}
			// SYNC-NN-THREAD-QUEUE-MIN-SIZE
			if len(m.nnThreadQueue) >= 2*cap(m.nnThreadQueue)/3 {
				// Our NN queue is full, so drop frames.
				break
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
		if m.mustStopFrameReader.Load() {
			break
		}
		if idle {
			time.Sleep(5 * time.Millisecond)
		}

		interval := 10 * math.Pow(1.5, float64(nStats))
		if interval > 5*60 {
			interval = 5 * 60
		}
		if time.Now().Sub(lastStats) > time.Duration(interval)*time.Second {
			nStats++
			totalFrames, totalProcessed := frameReaderStats(looperCameras)
			m.Log.Infof("%.0f%% frames analyzed by NN (%.1f ms per frame, per thread)", 100*float64(totalProcessed)/float64(totalFrames), float64(m.avgTimeNSPerFrameNN.Load())/1e6)
			lastStats = time.Now()
		}
	}
	close(m.frameReaderStopped)
}

// An NN processing thread
func (m *Monitor) nnThread() {
	lastErrAt := time.Time{}

	// I was originally tempted to reuse the same RGB image across iterations
	// of the loop (the 'rgb' variable). However, this doesn't actually help
	// performance at all, since we need to store a unique lastImg inside the
	// monitorCamera object.
	// I mean.. it did perhaps help performance a tiny bit, but it introduced
	// the bug of returning the incorrect lastImg for a camera (all cameras
	// would share the same lastImg).

	detectionParams := nn.NewDetectionParams()

	for {
		item, ok := <-m.nnThreadQueue
		if !ok || m.mustStopNNThreads.Load() {
			break
		}
		yuv := item.image
		rgb := cimg.NewImage(yuv.Width, yuv.Height, cimg.PixelFormatRGB)
		start := time.Now()
		yuv.CopyToCImageRGB(rgb)
		objects, err := m.detector.DetectObjects(nn.WholeImage(rgb.NChan(), rgb.Pixels, rgb.Width, rgb.Height), detectionParams)
		duration := time.Now().Sub(start)
		if m.avgTimeNSPerFrameNN.Load() == 0 {
			m.avgTimeNSPerFrameNN.Store(duration.Nanoseconds())
		} else {
			m.avgTimeNSPerFrameNN.Store((99*m.avgTimeNSPerFrameNN.Load() + duration.Nanoseconds()) / 100)
		}
		if err != nil {
			if time.Now().Sub(lastErrAt) > 15*time.Second {
				m.Log.Errorf("Error detecting objects: %v", err)
				lastErrAt = time.Now()
			}
		} else {
			//m.Log.Infof("Camera %v detected %v objects", mcam.camera.ID, len(objects))
			result := &nn.DetectionResult{
				CameraID:    item.camera.camera.ID(),
				ImageWidth:  yuv.Width,
				ImageHeight: yuv.Height,
				Objects:     objects,
			}
			item.camera.lock.Lock()
			item.camera.lastDetection = result
			item.camera.lastImg = rgb
			item.camera.lock.Unlock()

			if len(m.analyzerQueue) >= cap(m.analyzerQueue)*9/10 {
				// We do not expect this
				m.Log.Warnf("NN analyzer queue is falling behind - dropping frames")
			} else {
				m.analyzerQueue <- analyzerQueueItem{
					camera:    item.camera,
					detection: result,
				}
			}
		}
	}

	m.nnThreadStopWG.Done()
}
