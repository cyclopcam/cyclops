package monitor

import (
	"fmt"
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
	"github.com/cyclopcam/cyclops/pkg/nnaccel"
	"github.com/cyclopcam/cyclops/pkg/nnload"
	"github.com/cyclopcam/cyclops/server/camera"
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
	nnDevice                  *nnaccel.Device        // If nil, then we're using NCNN
	detector                  nn.ObjectDetector      // NN object detector
	enableFrameReader         bool                   // If false, then we don't run the frame reader
	mustStopFrameReader       atomic.Bool            // True if stopFrameReader() has been called
	analyzerQueue             chan analyzerQueueItem // Analyzer work queue. When closed, analyzer must exit.
	analyzerStopped           chan bool              // Analyzer thread has exited
	numNNThreads              int                    // Number of NN threads
	nnBatchSize               int                    // Batch size for NN
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
	nnUnrecognizedClass       int                    // Special index for the "class unrecognized" class
	analyzerSettings          analyzerSettings       // Analyzer settings
	nextTrackedObjectID       idgen.Uint32           // Next ID to assign to a tracked object

	// Dump the first frame of each camera, immediately before it gets sent to the NN for processing.
	// You get the RGB from the camera, and an RGB that was resized and letterboxed for the NN.
	debugDumpFrames bool
	dumpLock        sync.Mutex
	hasDumpedFrame  map[string]bool

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

	// ModelsDir is the directory where we store NN models
	ModelsDir string

	// If true, force NCNN to run in multithreaded mode. Used to speed up unit tests.
	MaxSingleThreadPerformance bool

	// Run an additional high quality model, which is used to confirm the detection of a new object.
	// If EnableDualModel is true, then ModelWidth and ModelHeight are ignored.
	EnableDualModel bool

	// If specified along with ModelHeight, this is the desired size of the neural network resolution.
	// This was created for unit tests, where we'd test different resolutions.
	// Ignored if EnableDualModel is true.
	ModelWidth int

	// See ModelWidth for details. Either ModelWidth and ModelHeight must be zero, or both must be non-zero.
	ModelHeight int
}

// DefaultMonitorOptions returns a new MonitorOptions object with default values
func DefaultMonitorOptions() *MonitorOptions {
	return &MonitorOptions{
		EnableFrameReader:          true,
		ModelName:                  "yolov8m",
		ModelsDir:                  "/var/lib/cyclops/models",
		MaxSingleThreadPerformance: false,
	}
}

// Create a new monitor
func NewMonitor(logger logs.Log, options *MonitorOptions) (*Monitor, error) {
	// Commenting this out when switching to auto-downloaded model files.
	// My new methodology has just a single ModelsDir, and this is automatically
	// populated by downloading from models.cyclopcam.org
	//basePath := ""
	//for _, tryPath := range options.ModelPaths {
	//	abs, err := filepath.Abs(tryPath)
	//	if err != nil {
	//		logger.Warnf("Unable to resolve model path candidate '%v' to an absolute path: %v", tryPath, err)
	//		continue
	//	}
	//	if _, err := os.Stat(filepath.Join(abs, "yolov8s.json")); err == nil {
	//		basePath = abs
	//		break
	//	}
	//}
	//if basePath == "" {
	//	return nil, fmt.Errorf("Could not find models directory. Searched in [%v]", strings.Join(options.ModelPaths, ", "))
	//}
	logger.Infof("Loading NN models from '%v'", options.ModelsDir)

	// Default size for CPU inference.
	nnWidth, nnHeight := 320, 256

	// On a Raspberry Pi 4, a single NN thread is best. But on my larger desktops, more threads helps.
	// I have some numbers in a spreadsheet. Basically, on a Pi, we want to have all cores processing
	// a single image at a time. But on desktop CPUs, we want one core per image.
	// Raspberry Pi 4 and up share an L2/L3 cache, and this presumably aids in processing images serially,
	// using OpenMP and whatever other threading mechanisms NCNN uses internally.
	numCPU := runtime.NumCPU()
	nnThreads := numCPU
	nnBatchSize := 1
	if nnload.HaveAccelerator() {
		// If we can do model parallelism with NN accelerators, then we'll probably
		// use some kind of queue issued by a single CPU thread, instead of having
		// a bunch of CPU threads hitting the accelerator, so we'd probably still be
		// using just a single thread here.
		nnThreads = 1
		// This must match one of the standard models that we host on models.cyclopcam.org.
		// The YOLO models that Hailo provides are configured for 640x640.
		nnWidth, nnHeight = 640, 640
		// 8 is a decent batch size for Hailo 8L, and it's likely to be a good number for other accelerators too.
		// On Hailo 8L YOLOv8m, a batch size of 10 gives milder better perf (50 vs 48 fps), but 8 just feels right.
		nnBatchSize = 8
	} else if options.MaxSingleThreadPerformance {
		nnThreads = 1
	} else if numCPU > 4 {
		// Vague empirical fudge value for my Ryzen 5900X with hyperthreading enabled
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
	logger.Infof("Using %v NN threads, mode %v, batch size %v", nnThreads, nnThreadingModel, nnBatchSize)

	if options.ModelWidth != 0 {
		nnWidth = options.ModelWidth
		nnHeight = options.ModelHeight
	}

	// SYNC-NN-THREAD-QUEUE-MIN-SIZE
	nnQueueSize := nnBatchSize * nnThreads * 2

	logger.Infof("Loading NN model '%v' %v x %v", options.ModelName, nnWidth, nnHeight)

	modelSetup := nn.NewModelSetup()
	modelSetup.BatchSize = nnBatchSize

	// Objects that are cleaned up if we fail
	var device *nnaccel.Device
	var detector nn.ObjectDetector
	defer func() {
		if detector != nil {
			detector.Close()
		}
		if device != nil {
			device.Close()
		}
	}()

	var err error

	// If device is nil, then we're using NCNN
	accel := nnload.Accelerator()
	if accel != nil {
		device, err = accel.OpenDevice()
		if err != nil {
			logger.Infof("Failed to open NN accelerator device: %v. Falling back to NCNN", err)
		}
	}

	detector, err = nnload.LoadModel(logger, device, options.ModelsDir, options.ModelName, nnWidth, nnHeight, nnThreadingModel, modelSetup)
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
	seenAbstract := map[string]bool{}
	for _, v := range abstractClasses {
		if !seenAbstract[v] {
			classList = append(classList, v)
			seenAbstract[v] = true
		}
	}
	// Add a special "class unrecognized" class
	unrecognizedIdx := len(classList)
	classList = append(classList, "class unrecognized")

	classMap := map[string]int{}
	for i, c := range classList {
		classMap[c] = i
	}

	logger.Infof("Starting %v NN detection threads", nnThreads)

	m := &Monitor{
		Log:                 logger,
		nnDevice:            device,
		detector:            detector,
		nnThreadQueue:       make(chan monitorQueueItem, nnQueueSize),
		analyzerQueue:       make(chan analyzerQueueItem, analysisQueueSize),
		analyzerStopped:     make(chan bool),
		nnModelSetup:        modelSetup,
		numNNThreads:        nnThreads,
		nnBatchSize:         nnBatchSize,
		nnClassList:         classList,
		nnClassMap:          classMap,
		nnClassFilterSet:    makeClassFilter(classFilterList),
		nnClassAbstract:     abstractClasses,
		nnClassBoxMerge:     boxMergeClasses,
		nnAbstractClassSet:  makeAbstractClassSet(abstractClasses, classMap),
		nnUnrecognizedClass: unrecognizedIdx,
		analyzerSettings:    *newAnalyzerSettings(),
		watchers:            map[int64][]chan *AnalysisState{},
		watchersAllCameras:  []chan *AnalysisState{},
		enableFrameReader:   options.EnableFrameReader,
		debugDumpFrames:     true,
		hasDumpedFrame:      map[string]bool{},
	}

	// Prevent our cleanup defer func from deleting these objects
	device = nil
	detector = nil

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
	close(m.nnThreadQueue)
	m.nnThreadStopWG.Wait()

	// Stop analyzer
	m.Log.Infof("Monitor waiting for analyzer")
	close(m.analyzerQueue)

	// Close the C++ NN object
	m.detector.Close()

	m.Log.Infof("Monitor is closed")
}

// Return the list of all classes that the NN detects
func (m *Monitor) AllClasses() []string {
	return m.nnClassList
}

// Returns the map of concrete -> abstract NN classes
func (m *Monitor) AbstractClasses() map[string]string {
	return m.nnClassAbstract
}

// Returns the special index of the "class unrecognized" class if 'cls' is not recognized
func (m *Monitor) ClassToIdx(cls string) int {
	idx, ok := m.nnClassMap[cls]
	if !ok {
		return m.nnUnrecognizedClass
	}
	return idx
}

// Returns the class index of the special "class unrecognized" class
func (m *Monitor) UnrecognizedClassIdx() int {
	return m.nnUnrecognizedClass
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

// Stop listening to cameras.
// This function only returns once the frame reader thread has exited.
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
