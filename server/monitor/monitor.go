package monitor

import (
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bmharper/cimg/v2"
	"github.com/bmharper/cyclops/pkg/log"
	"github.com/bmharper/cyclops/server/camera"
	"github.com/bmharper/cyclops/server/ncnn"
	"github.com/bmharper/cyclops/server/nn"
)

// monitor runs our neural networks on the camera streams

type Monitor struct {
	Log                 log.Log
	detector            nn.ObjectDetector
	mustStopLooper      atomic.Bool           // True if stopLooper() has been called
	mustStopNNThreads   atomic.Bool           // NN threads must exit
	numNNThreads        int                   // Number of NN threads
	nnThreadStopWG      sync.WaitGroup        // Wait for all NN threads to exit
	looperStopped       chan bool             // When looperStop channel is closed, then the looped has stopped
	cameras             []*monitorCamera      // Cameras that we're monitoring
	nnFrameTime         time.Duration         // Average time for the neural network to process a frame
	nnThreadQueue       chan monitorQueueItem // Queue of images to be processed by the neural network
	avgTimeNSPerFrameNN atomic.Int64          // Average time (ns) per frame, for just the neural network (time inside a thread)

	//isPaused  atomic.Int32
}

type monitorCamera struct {
	camera *camera.Camera

	lock    sync.Mutex
	lastImg *cimg.Image    // Guarded by 'lock' mutex. If objects is not nil, then this is the image that was used to generate the objects.
	objects []nn.Detection // Guarded by 'lock' mutex
}

type monitorQueueItem struct {
	camera *monitorCamera
	image  *cimg.Image
}

func NewMonitor(logger log.Log) (*Monitor, error) {
	detector, err := ncnn.NewDetector("yolov7", "models/yolov7-tiny.param", "models/yolov7-tiny.bin")
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
	nThreads := 1
	queueSize := nThreads * 3

	m := &Monitor{
		Log:           logger,
		detector:      detector,
		nnThreadQueue: make(chan monitorQueueItem, queueSize),
		numNNThreads:  nThreads,
	}
	for i := 0; i < m.numNNThreads; i++ {
		go m.nnThread()
	}
	m.start()
	return m, nil
}

// Close the monitor object.
func (m *Monitor) Close() {
	m.Log.Infof("Monitor shutting down")

	// Stop reading images from cameras
	m.stopLooper()

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

// Stop listening to cameras
func (m *Monitor) stopLooper() {
	m.mustStopLooper.Store(true)
	<-m.looperStopped
}

// Start/Restart looper
func (m *Monitor) start() {
	m.mustStopLooper.Store(false)
	m.looperStopped = make(chan bool)
	go m.loop()
}

// Set cameras and start monitoring
func (m *Monitor) SetCameras(cameras []*camera.Camera) {
	m.stopLooper()
	m.cameras = nil
	for _, cam := range cameras {
		m.cameras = append(m.cameras, &monitorCamera{
			camera: cam,
		})
	}
	m.start()
}

type looperCameraState struct {
	lastID             int64
	numFramesTotal     int64 // Number of frames from this camera that we've seen
	numFramesProcessed int64 // Number of frames from this camera that we've analyzed
}

func looperStats(cameraStates map[*monitorCamera]*looperCameraState) (totalFrames int64, totalProcessed int64) {
	for _, state := range cameraStates {
		totalFrames += state.numFramesTotal
		totalProcessed += state.numFramesProcessed
	}
	return
}

// Loop runs until Close()
func (m *Monitor) loop() {
	cameraStates := map[*monitorCamera]*looperCameraState{}
	for _, mcam := range m.cameras {
		cameraStates[mcam] = &looperCameraState{}
	}

	// maintain camera index outside of main loop, so that we're not
	// biased towards processing the frames of the first cameras
	icam := uint(0)

	lastStats := time.Now()
	nStats := 0
	for !m.mustStopLooper.Load() {
		idle := true
		for i := 0; i < len(m.cameras); i++ {
			if m.mustStopLooper.Load() {
				break
			}
			if len(m.nnThreadQueue) >= 2*cap(m.nnThreadQueue)/3 {
				continue
			}

			// It's vital that this incrementing happens after the queue check above,
			// otherwise you don't get round robin behaviour.
			icam = (icam + 1) % uint(len(m.cameras))
			mcam := m.cameras[icam]
			camState := cameraStates[mcam]

			//m.Log.Infof("%v", icam)
			img, imgID := mcam.camera.LowDecoder.GetLastImageIfDifferent(camState.lastID)
			if img != nil {
				if camState.lastID == 0 {
					camState.numFramesTotal++
				} else {
					camState.numFramesTotal += imgID - camState.lastID
				}
				//m.Log.Infof("Got image %d from camera %s (%v / %v)", imgID, mcam.camera.Name, camState.numFramesProcessed, camState.numFramesTotal)
				camState.numFramesProcessed++
				camState.lastID = imgID
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

		interval := 3 * math.Pow(1.1, float64(nStats))
		if time.Now().Sub(lastStats) > time.Duration(interval)*time.Second {
			nStats++
			totalFrames, totalProcessed := looperStats(cameraStates)
			m.Log.Infof("%.0f%% frames analyzed by NN (%.1f ms per frame, per thread)", 100*float64(totalProcessed)/float64(totalFrames), float64(m.avgTimeNSPerFrameNN.Load())/1e6)
			lastStats = time.Now()
		}
	}
	close(m.looperStopped)
}

// An NN processing thread
func (m *Monitor) nnThread() {
	lastErrAt := time.Time{}

	for {
		item, ok := <-m.nnThreadQueue
		if !ok || m.mustStopNNThreads.Load() {
			break
		}
		img := item.image
		start := time.Now()
		objects, err := m.detector.DetectObjects(img.NChan(), img.Pixels, img.Width, img.Height)
		duration := time.Now().Sub(start)
		m.avgTimeNSPerFrameNN.Store((99*m.avgTimeNSPerFrameNN.Load() + duration.Nanoseconds()) / 100)
		if err != nil {
			if time.Now().Sub(lastErrAt) > 15*time.Second {
				m.Log.Errorf("Error detecting objects: %v", err)
				lastErrAt = time.Now()
			}
		} else {
			//m.Log.Infof("Camera %v detected %v objects", mcam.camera.ID, len(objects))
			item.camera.lock.Lock()
			item.camera.objects = objects
			item.camera.lastImg = img
			item.camera.lock.Unlock()
		}
	}

	m.nnThreadStopWG.Done()
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
