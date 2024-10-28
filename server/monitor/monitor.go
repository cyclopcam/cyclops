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
	"github.com/cyclopcam/cyclops/pkg/idgen"
	"github.com/cyclopcam/cyclops/pkg/nn"
	"github.com/cyclopcam/cyclops/pkg/nnload"
	"github.com/cyclopcam/cyclops/server/camera"
	"github.com/cyclopcam/cyclops/server/perfstats"
	"github.com/cyclopcam/logs"
)

// If not specified, then this is our list of classes that we pay attention to.
// Other classes (such as potplant, frisbee, etc) are ignored.
// The classes in our abstract list are implicitly also inside this list.
var defaultClassFilterList = []string{
	nn.COCOClasses[nn.COCOPerson],
	nn.COCOClasses[nn.COCOBicycle],
	nn.COCOClasses[nn.COCOCar],
	nn.COCOClasses[nn.COCOBus],
	nn.COCOClasses[nn.COCOMotorcycle],
	nn.COCOClasses[nn.COCOTruck],
	// abstract classes
	"vehicle",
}

// If we get detection boxes of any of these pairs, and the boxes have very high
// IoU, then we merge them into the same object. The type of the object is the
// right side of the map. For example, given {"truck": "car"} in the map, and we
// have a car/truck pair, the resulting object will be a "car".
var boxMergeClasses = map[string]string{
	"truck": "car",
}

// Class map from concrete to abstract (eg car -> vehicle, truck -> vehicle)
// NOTE: If you add new mappings here, also add them to defaultClassFilterList
var abstractClasses = map[string]string{
	"car":        "vehicle",
	"motorcycle": "vehicle",
	"truck":      "vehicle",
	"bus":        "vehicle",
}

/*
	Monitor runs our neural networks on the camera streams

We process camera frames in phases:
1. Read frames from cameras (frameReader)
2. Process frames with a neural network (nnThread)
3. Analyze results from the neural network (analyzer)

We connect these phases with channels.
*/
type Monitor struct {
	Log                       logs.Log
	detector                  nn.ObjectDetector
	enableFrameReader         bool                   // If false, then we don't run the frame reader
	mustStopFrameReader       atomic.Bool            // True if stopFrameReader() has been called
	mustStopNNThreads         atomic.Bool            // NN threads must exit
	analyzerQueue             chan analyzerQueueItem // Analyzer work queue. When closed, analyzer must exit.
	analyzerStopped           chan bool              // Analyzer thread has exited
	numNNThreads              int                    // Number of NN threads
	nnModelSetup              *nn.ModelSetup         // Configuration of NN models
	nnThreadStopWG            sync.WaitGroup         // Wait for all NN threads to exit
	frameReaderStopped        chan bool              // When frameReaderStopped channel is closed, then the frame reader has stopped
	nnThreadQueue             chan monitorQueueItem  // Queue of images to be processed by the neural network
	avgTimeNSPerFrameNNPrep   atomic.Int64           // Average time (ns) per frame, for prep of an image before it hits the NN
	avgTimeNSPerFrameNNDet    atomic.Int64           // Average time (ns) per frame, for just the neural network (time inside a thread)
	hasShownResolutionWarning atomic.Bool            // True if we've shown a warning about camera resolution vs NN resolution
	nnClassList               []string               // All the classes that the NN emits (in their native order)
	nnClassMap                map[string]int         // Map from class name to class index
	nnClassFilterSet          map[string]bool        // NN classes that we're interested in (eg person, car)
	nnClassBoxMerge           map[string]string      // Merge overlapping boxes eg car/truck -> car
	nnClassAbstract           map[string]string      // Remap classes to a more abstract class (eg car -> vehicle, truck -> vehicle)
	nnAbstractClassSet        map[int]bool           // Set of abstract class indices
	analyzerSettings          analyzerSettings       // Analyzer settings
	nextTrackedObjectID       idgen.Uint32           // Next ID to assign to a tracked object

	debugDumpFrames bool // Dump the first frame of each camera, immediately before it gets sent to the NN for processing.
	dumpLock        sync.Mutex
	hasDumpedCamera map[int64]bool

	camerasLock sync.Mutex       // Guards access to cameras
	cameras     []*monitorCamera // Cameras that we're monitoring

	watchersLock       sync.RWMutex                    // Guards access to watchers, watchersAllCameras
	watchers           map[int64][]chan *AnalysisState // Keys are CameraID. Values are channels to send detection results to
	watchersAllCameras []chan *AnalysisState           // Agents watching all cameras
}

// monitorCamera is the internal data structure for managing a single camera that we are monitoring
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

	// Guarded by 'lock' mutex.
	// Can be nil.
	// Same comment applies here as to lastImg, in the sense that the contents of this object is immutable.
	analyzerState *AnalysisState
}

type monitorQueueItem struct {
	monCam   *monitorCamera
	image    *accel.YUVImage
	framePTS time.Time
}

type analyzerQueueItem struct {
	monCam    *monitorCamera
	detection *nn.DetectionResult
}

type MonitorOptions struct {
	// EnableFrameReader is allowed to be false for unit tests, so that the tests can feed the monitor
	// frames directly, without having the monitor pull frames from the cameras.
	EnableFrameReader bool

	// ModelName is the NN model name, such as "yolov8m"
	ModelName string

	// ModelPaths is a list of directories to search for NN models
	ModelPaths []string

	// If true, force NCNN to run in multithreaded mode. Used to speed up unit tests.
	MaxSingleThreadPerformance bool
}

// DefaultMonitorOptions returns a new MonitorOptions object with default values
func DefaultMonitorOptions() *MonitorOptions {
	return &MonitorOptions{
		EnableFrameReader:          true,
		ModelName:                  "yolov8m",
		ModelPaths:                 []string{"models", "/var/lib/cyclops/models"},
		MaxSingleThreadPerformance: false,
	}
}

// Create a new monitor
func NewMonitor(logger logs.Log, options *MonitorOptions) (*Monitor, error) {
	basePath := ""
	for _, tryPath := range options.ModelPaths {
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
		return nil, fmt.Errorf("Could not find models directory. Searched in [%v]", strings.Join(options.ModelPaths, ", "))
	}
	logger.Infof("Loading NN models from '%v'", basePath)

	// On a Raspberry Pi 4, a single NN thread is best. But on my larger desktops, more threads helps.
	// I have some numbers in a spreadsheet. Basically, on a Pi, we want to have all cores processing
	// a single image at a time. But on desktop CPUs, we want one core per image.
	// Raspberry Pi 4 and up share an L2/L3 cache, and this presumably aids in processing images serially,
	// using OpenMP and whatever other threading mechanisms NCNN uses internally.
	numCPU := runtime.NumCPU()
	nnThreads := numCPU
	if nnload.HaveAccelerator() {
		// If we can do model parallelism with NN accelerators, then we'll probably
		// use some kind of queue issued by a single CPU thread, instead of having
		// a bunch of CPU threads hitting the accelerator.
		nnThreads = 1
	} else if options.MaxSingleThreadPerformance {
		nnThreads = 1
	} else if numCPU > 4 {
		nnThreads = numCPU / 2
	} else {
		// Raspberry Pi, or some other SBC (4 cores)
		nnThreads = 1
	}
	nnThreadingModel := nn.ThreadingModeSingle
	if nnThreads == 1 {
		// If we're only running a single detection thread, then let the NN library use however
		// many cores it can.
		nnThreadingModel = nn.ThreadingModeParallel
	}
	logger.Infof("Using %v NN threads, mode %v", nnThreads, nnThreadingModel)

	// nnQueueSize should be at least equal to nnThreads, otherwise we'll never reach full utilization.
	// But perhaps we can use nnQueueSize as a throttle, to optimize the number of active threads.
	// One important point:
	// queueSize must be at least twice the size of nnThreads, so that our exit mechanism can work.
	// Once we signal mustStopNNThreads, we fill the queue with dummy jobs, so that the NN threads
	// can wake up from their channel receive operation, and exit.
	// If the queue size was too small, then this would deadlock.
	// nnQueueSize must not be less than 1, otherwise our backoff mechanism will never allow a
	// frame through.
	// SYNC-NN-THREAD-QUEUE-MIN-SIZE
	nnQueueSize := nnThreads * 3

	logger.Infof("Loading NN model '%v'", options.ModelName)

	modelSetup := nn.NewModelSetup()

	detector, err := nnload.LoadModel(logger, basePath, options.ModelName, nnThreadingModel, modelSetup)
	if err != nil {
		return nil, err
	}

	logger.Infof("NN resolution is %v x %v", detector.Config().Width, detector.Config().Height)
	logger.Infof("NN batch size %v", modelSetup.BatchSize)
	logger.Infof("NN prob threshold %.2f, NMS IoU threshold %.2f", modelSetup.ProbabilityThreshold, modelSetup.NmsIouThreshold)

	classFilterList := defaultClassFilterList
	logger.Infof("Paying attention to the following classes: %v", strings.Join(classFilterList, ","))

	// No idea what a good number is here. I expect analysis to be much
	// faster to run than NN, so provided this queue is large enough to
	// prevent bumps, it shouldn't matter too much.
	// Analysis is where we watch the movement of boxes, after they've
	// been emitted by the NN.
	analysisQueueSize := 20

	classList := detector.Config().Classes
	for _, v := range abstractClasses {
		classList = append(classList, v)
	}
	classMap := map[string]int{}
	for i, c := range classList {
		classMap[c] = i
	}

	logger.Infof("Starting %v NN detection threads", nnThreads)

	m := &Monitor{
		Log:                logger,
		detector:           detector,
		nnThreadQueue:      make(chan monitorQueueItem, nnQueueSize),
		analyzerQueue:      make(chan analyzerQueueItem, analysisQueueSize),
		analyzerStopped:    make(chan bool),
		nnModelSetup:       modelSetup,
		numNNThreads:       nnThreads,
		nnClassList:        classList,
		nnClassMap:         classMap,
		nnClassFilterSet:   makeClassFilter(classFilterList),
		nnClassAbstract:    abstractClasses,
		nnClassBoxMerge:    boxMergeClasses,
		nnAbstractClassSet: makeAbstractClassSet(abstractClasses, classMap),
		analyzerSettings: analyzerSettings{
			positionHistorySize:         30,   // at 10 fps, 30 frames = 3 seconds
			maxAnalyzeObjectsPerFrame:   20,   // We have O(n^2) analysis functions, so we need to keep this small.
			minDistanceForObject:        0.02, // 2% of the frame width (0.02 * 320 = 6 pixels)
			minDiscreetPositionsDefault: 2,
			minDiscreetPositions: map[string]int{
				"person": 3, // People move much slower than cars, and people are almost always alarmable events, so we need a super low false positive rate
			},
			objectForgetTime: 5 * time.Second,
			verbose:          false,
		},
		watchers:           map[int64][]chan *AnalysisState{},
		watchersAllCameras: []chan *AnalysisState{},
		enableFrameReader:  options.EnableFrameReader,
		debugDumpFrames:    false,
		hasDumpedCamera:    map[int64]bool{},
	}
	//m.sendTestImageToNN()
	for i := 0; i < m.numNNThreads; i++ {
		go m.nnThread()
	}
	if m.enableFrameReader {
		m.startFrameReader()
	}
	go m.analyzer()

	return m, nil
}

//func (m *Monitor) sendTestImageToNN() {
//	img := cimg.NewImage(m.detector.Config().Width, m.detector.Config().Height, cimg.PixelFormatRGB)
//	m.detector.DetectObjects(nn.WholeImage(img.NChan(), img.Pixels, img.Width, img.Height), nn.NewDetectionParams())
//}

// Close the monitor object.
func (m *Monitor) Close() {
	m.Log.Infof("Monitor shutting down")

	// Stop reading images from cameras
	if m.enableFrameReader {
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

// Return the list of all classes that the NN detects
func (m *Monitor) AllClasses() []string {
	return m.nnClassList
}

// Returns the number of items awaiting processing in the NN queue
func (m *Monitor) NNThreadQueueLength() int {
	return len(m.nnThreadQueue)
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

	//fmt.Printf("LatestFrame %v = %p, analyzerState = %p\n", cameraID, cam, cam.analyzerState)

	return cam.lastImg, cam.lastDetection, cam.analyzerState, nil
}

// SYNC-WATCHER-CHANNEL-SIZE
const WatcherChannelSize = 100

// Register to receive detection results for a specific camera.
// You must be careful to ensure that your receiver always processes a result
// immediately, and keeps the channel drained. If you don't do this, then
// the monitor will freeze, and obviously that's a really bad thing to happen
// to a security system.
func (m *Monitor) AddWatcher(cameraID int64) chan *AnalysisState {
	m.watchersLock.Lock()
	defer m.watchersLock.Unlock()
	ch := make(chan *AnalysisState, WatcherChannelSize)
	m.watchers[cameraID] = append(m.watchers[cameraID], ch)
	return ch
}

// Unregister from detection results for a specific camera
func (m *Monitor) RemoveWatcher(cameraID int64, ch chan *AnalysisState) {
	m.watchersLock.Lock()
	defer m.watchersLock.Unlock()
	for i, w := range m.watchers[cameraID] {
		if w == ch {
			m.watchers[cameraID] = gen.DeleteFromSliceUnordered(m.watchers[cameraID], i)
			return
		}
	}
	m.Log.Warnf("Monitor.RemoveWatcher failed to find channel for camera %v", cameraID)
}

// Add a watcher that is interested in all camera activity
func (m *Monitor) AddWatcherAllCameras() chan *AnalysisState {
	m.watchersLock.Lock()
	defer m.watchersLock.Unlock()
	ch := make(chan *AnalysisState, WatcherChannelSize)
	m.watchersAllCameras = append(m.watchersAllCameras, ch)
	return ch
}

// Unregister from detection results of all cameras
func (m *Monitor) RemoveWatcherAllCameras(ch chan *AnalysisState) {
	m.watchersLock.Lock()
	defer m.watchersLock.Unlock()
	for i, wch := range m.watchersAllCameras {
		if wch == ch {
			m.watchersAllCameras = gen.DeleteFromSliceUnordered(m.watchersAllCameras, i)
			return
		}
	}
	m.Log.Warnf("Monitor.RemoveWatcherAllCameras failed to find channel")
}

func (m *Monitor) sendToWatchers(state *AnalysisState) {
	m.watchersLock.RLock()
	// Regarding our behaviour here to drop frames:
	// Perhaps it would be better not to drop frames, but simply to stall.
	// This would presumably wake up the threads that consume the analysis.
	// HOWEVER - if a watcher is waiting on IO, then waking up other threads
	// wouldn't help.
	for _, ch := range m.watchers[state.CameraID] {
		// SYNC-WATCHER-CHANNEL-SIZE
		if len(ch) >= cap(ch)*9/10 {
			// This should never happen. But as a safeguard against a monitor stalls, we choose to drop frames.
			m.Log.Warnf("Monitor watcher on camera %v is falling behind. I am going to drop frames.", state.CameraID)
		} else {
			ch <- state
		}
	}
	for _, ch := range m.watchersAllCameras {
		if len(ch) >= cap(ch)*9/10 {
			// This should never happen. But as a safeguard against a monitor stalls, we choose to drop frames.
			m.Log.Warnf("Monitor watcher on all cameras is falling behind. I am going to drop frames.")
		} else {
			ch <- state
		}
	}
	m.watchersLock.RUnlock()
}

// Resolve class names to integer indices.
// If a class is not found, it is ignored.
//func lookupClassIndices(classes []string, allClasses []string) []int {
//	clsToIndex := map[string]int{}
//	for i, c := range allClasses {
//		clsToIndex[c] = i
//	}
//	r := []int{}
//	for _, c := range classes {
//		if idx, ok := clsToIndex[c]; ok {
//			r = append(r, idx)
//		}
//	}
//	return r
//}

// Return a set containing all the abstract class indices
func makeAbstractClassSet(abstractClasses map[string]string, classMap map[string]int) map[int]bool {
	r := map[int]bool{}
	for _, abstract := range abstractClasses {
		r[classMap[abstract]] = true
	}
	return r
}

func makeClassFilter(classes []string) map[string]bool {
	r := map[string]bool{}
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
	if m.enableFrameReader {
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

	// Remove watchers for cameras that no longer exist
	// Hmmm.. I'm undecided about this. It's possibly in the territory of
	// "unwanted action at a distance".
	// Imagine a scenario where a watcher is added, and then a camera blips
	// for a few seconds. It gets removed and re-added. During that time, the
	// agent watching was OK with just having things go silent for a few seconds,
	// and then return. It didn't anticipate having to re-add the watcher.
	//m.watchersLock.Lock()
	//newWatchers := map[int64][]watcher{}
	//for _, cam := range cameras {
	//	newWatchers[cam.ID()] = m.watchers[cam.ID()]
	//}
	//m.watchers = newWatchers
	//m.watchersLock.Unlock()

	if m.enableFrameReader {
		m.startFrameReader()
	}
}

// State internal to the NN frame reader, for each camera
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

// Read camera frames and send them off for analysis.
// A single thread runs this operation.
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
		// Why do we have this inner loop?
		// We keep it so that we can detect when to idle.
		// If we complete a loop over all looperCameras, and we didn't have any work to do,
		// then we idle for a few milliseconds.
		for i := 0; i < len(looperCameras); i++ {
			if m.mustStopFrameReader.Load() {
				break
			}
			// SYNC-NN-THREAD-QUEUE-MIN-SIZE
			if len(m.nnThreadQueue) >= 2*cap(m.nnThreadQueue)/3 {
				// Our NN queue is 2/3 full, so drop frames.
				break
			}

			// It's vital that this incrementing happens after the queue check above,
			// otherwise you don't get round robin behaviour.
			icam = (icam + 1) % uint(len(looperCameras))
			camState := looperCameras[icam]
			mcam := camState.mcam

			//m.Log.Infof("%v", icam)
			img, imgID, imgPTS := mcam.camera.LowDecoder.GetLastImageIfDifferent(camState.lastFrameID)
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
					monCam:   mcam,
					image:    img,
					framePTS: imgPTS,
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
		interval = max(interval, 5)
		interval = min(interval, 3600)
		if time.Now().Sub(lastStats) > time.Duration(interval)*time.Second {
			nStats++
			totalFrames, totalProcessed := frameReaderStats(looperCameras)
			m.Log.Infof("%.0f%% frames analyzed by NN. %v Threads. Times per frame: (%.1f ms Prep, %.1f ms NN)",
				100*float64(totalProcessed)/float64(totalFrames),
				m.numNNThreads,
				float64(m.avgTimeNSPerFrameNNPrep.Load())/1e6,
				float64(m.avgTimeNSPerFrameNNDet.Load())/1e6,
			)
			lastStats = time.Now()
		}
	}
	close(m.frameReaderStopped)
}

// Perform image format conversions and resizing so that we can send to our NN.
// We should consider having the resizing done by ffmpeg.
// Images returned are (originalRgb, nnScaledRgb)
func (m *Monitor) prepareImageForNN(yuv *accel.YUVImage) (nn.ResizeTransform, *cimg.Image, *cimg.Image) {
	start := time.Now()
	nnConfig := m.detector.Config()
	nnWidth := nnConfig.Width
	nnHeight := nnConfig.Height

	xform := nn.IdentityResizeTransform()

	rgb := cimg.NewImage(yuv.Width, yuv.Height, cimg.PixelFormatRGB)
	yuv.CopyToCImageRGB(rgb)
	if (rgb.Width > nnWidth || rgb.Height > nnHeight) && m.hasShownResolutionWarning.CompareAndSwap(false, true) {
		m.Log.Warnf("Camera image size %vx%v is larger than NN input size %vx%v", rgb.Width, rgb.Height, nnWidth, nnHeight)
	}
	originalRgb := rgb

	if rgb.Width != nnWidth || rgb.Height != nnHeight {
		scaleX := float32(nnWidth) / float32(rgb.Width)
		scaleY := float32(nnHeight) / float32(rgb.Height)
		scale := min(scaleX, scaleY)
		xform.ScaleX = scale
		xform.ScaleY = scale
		newWidth := int(float32(rgb.Width)*scale + 0.5)
		newHeight := int(float32(rgb.Height)*scale + 0.5)
		resizeParams := cimg.ResizeParams{
			CheapSRGBFilter: true,
		}
		rgb = cimg.ResizeNew(rgb, newWidth, newHeight, &resizeParams)
		if newWidth != nnWidth || newHeight != nnHeight {
			// Insert resized image into black canvas. There will be a block on the right
			// or the bottom of black pixels. This is not ideal. We should really aim to
			// have NNs that are a closer match to the aspect ratio of our camera images.
			// We could get rid of this step if we made ResizeNew capable of accepting
			// a 'view' instead of a contiguous image, but that would require changes
			// to cimg.
			big := cimg.NewImage(nnWidth, nnHeight, cimg.PixelFormatRGB)
			big.CopyImage(rgb, 0, 0)
			rgb = big
		}
	}
	perfstats.UpdateMovingAverage(&m.avgTimeNSPerFrameNNPrep, time.Now().Sub(start).Nanoseconds())
	return xform, originalRgb, rgb
}

//func (m *Monitor) scaleDetectionsToOriginalImage(orgWidth, orgHeight, nnWidth, nnHeight int, detections []nn.ObjectDetection) {
//	xscale := float32(orgWidth) / float32(nnWidth)
//	yscale := float32(orgHeight) / float32(nnHeight)
//	for i := range detections {
//		d := &detections[i]
//		d.Box.X = int(float32(d.Box.X) * xscale)
//		d.Box.Y = int(float32(d.Box.Y) * yscale)
//		d.Box.Width = int(float32(d.Box.Width) * xscale)
//		d.Box.Height = int(float32(d.Box.Height) * yscale)
//	}
//}

func (m *Monitor) dumpFrame(rgb *cimg.Image, cam *camera.Camera) {
	m.dumpLock.Lock()
	hasDumped := m.hasDumpedCamera[cam.ID()]
	if hasDumped {
		m.dumpLock.Unlock()
		return
	}
	m.hasDumpedCamera[cam.ID()] = true
	m.dumpLock.Unlock()

	b, _ := cimg.Compress(rgb, cimg.MakeCompressParams(cimg.Sampling(cimg.Sampling420), 85, cimg.Flags(0)))
	os.WriteFile(fmt.Sprintf("frame-%v.jpg", cam.Name()), b, 0644)
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

	// Resizing image for NN inference:
	// When implementing support for the Hailo8L on Raspberry Pi5, the easiest
	// thing to do was to use the pretrained YOLOv8 model, which has an input
	// size of 640x640. Our cameras are typically setup to emit 2nd stream
	// images as a lower resolution (eg 320 x 256). Until this time, my NCNN
	// YOLOv8 used an input resolution of 320 x 256, so it perfectly matched
	// the camera 2nd streams. So my decision at the time of implementing
	// support for the Hailo8L was to simply add black padding around the
	// 320x256 images, to make them 640x640. This is not ideal. We should
	// either be using larger 2nd stream images from the camera, or creating
	// a custom Hailo8L YOLOv8 model with a smaller input resolution.
	// But now you know why we do it this way. It's not the best, just the
	// easiest, an good enough for now.

	// For Hailo/Accelerators, these parameters are defined at model setup time,
	// but for NCNN, we control them with each detection. We should probably get
	// rid of the per-detection mechanism so that it all goes in through one mechanism.
	detectionParams := nn.NewDetectionParams()
	detectionParams.ProbabilityThreshold = m.nnModelSetup.ProbabilityThreshold
	detectionParams.NmsIouThreshold = m.nnModelSetup.NmsIouThreshold

	for frameCount := 0; true; frameCount++ {
		item, ok := <-m.nnThreadQueue
		if !ok || m.mustStopNNThreads.Load() {
			break
		}
		yuv := item.image
		xformRgbToNN, rgbPure, rgbNN := m.prepareImageForNN(yuv)
		if m.debugDumpFrames {
			m.dumpFrame(rgbNN, item.monCam.camera)
		}
		start := time.Now()
		objects, err := m.detector.DetectObjects(nn.WholeImage(rgbNN.NChan(), rgbNN.Pixels, rgbNN.Width, rgbNN.Height), detectionParams)
		xformRgbToNN.ApplyBackward(objects)
		perfstats.UpdateMovingAverage(&m.avgTimeNSPerFrameNNDet, time.Now().Sub(start).Nanoseconds())
		if err != nil {
			if time.Now().Sub(lastErrAt) > 15*time.Second {
				m.Log.Errorf("Error detecting objects: %v", err)
				lastErrAt = time.Now()
			}
		} else {
			//m.Log.Infof("Camera %v detected %v objects", mcam.camera.ID, len(objects))
			result := &nn.DetectionResult{
				CameraID:    item.monCam.camera.ID(),
				ImageWidth:  yuv.Width,
				ImageHeight: yuv.Height,
				Objects:     objects,
				FramePTS:    item.framePTS,
			}
			item.monCam.lock.Lock()
			item.monCam.lastDetection = result
			item.monCam.lastImg = rgbPure
			item.monCam.lock.Unlock()

			if len(m.analyzerQueue) >= cap(m.analyzerQueue)*9/10 {
				// We do not expect this
				m.Log.Warnf("NN analyzer queue is falling behind - dropping frames")
			} else {
				m.analyzerQueue <- analyzerQueueItem{
					monCam:    item.monCam,
					detection: result,
				}
			}
		}
	}

	m.nnThreadStopWG.Done()
}
