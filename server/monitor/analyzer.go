package monitor

import (
	"math"
	"sort"
	"time"

	"github.com/bmharper/ringbuffer"
	"github.com/cyclopcam/cyclops/pkg/nn"
)

// If true, then alert on all classes in the COCO set
// If false, then only alert on the classes in defaultNNClassFilter()
const includeAllClasses = false

type analyzerSettings struct {
	positionHistorySize       int            // Keep a ring buffer of the last N positions of each object
	maxAnalyzeObjectsPerFrame int            // Maximum number of objects to analyze per frame
	minDistance               map[string]int // Minimum distance that an object must travel to be considered a true detection (in pixels)
	minDistanceDefault        int            // Default, if no class override
	minSightings              map[string]int // Minimum number of sightings that an object must have to be considered a true detection
	minSightingsDefault       int            // Default, if no class override
	objectForgetTime          time.Duration  // After this amount of time of not seeing an object, we believe it has left the frame, or was a false detection

	// If an object is determined by the HQ network to be a false positive, but we keep
	// seeing it in the LQ network, then re-run the HQ analysis every X seconds, to make
	// sure we haven't missed a genuine object.
	revalidateInterval time.Duration

	verbose bool // Print out debug information
}

func newAnalyzerSettings(verbose bool) *analyzerSettings {
	return &analyzerSettings{
		positionHistorySize:       30,  // at 10 fps, 30 frames = 3 seconds
		maxAnalyzeObjectsPerFrame: 100, // This is just a sanity thing, but perhaps we shouldn't have any limit
		minDistanceDefault:        5,   // 5 pixels
		minDistance: map[string]int{
			"person": 5, // People must be moving to be considered genuine
			// NOTE: You want the abstract classes all to have the same thresholds, otherwise you end up with
			// pointless HQ validation work that gets triggered as the concrete class triggers at a different
			// time to the abstract class.
			// SYNC-ABSTRACT-CLASSES
			"car":        0, // Vehicles can be stationary
			"motorcycle": 0, // Vehicles can be stationary
			"truck":      0, // Vehicles can be stationary
			"bus":        0, // Vehicles can be stationary
			"vehicle":    0, // Vehicles can be stationary
		},
		minSightingsDefault: 2,
		minSightings: map[string]int{
			"person": 3, // People are almost always alarmable events, so we need a super low false positive rate
		},
		objectForgetTime:   5 * time.Second,
		revalidateInterval: 2 * time.Second,
		verbose:            verbose,
	}
}

func (a *analyzerSettings) minSightingsForClass(cls string) int {
	if val, ok := a.minSightings[cls]; ok {
		return val
	}
	return a.minSightingsDefault
}

func (a *analyzerSettings) minDistanceForClass(cls string) int {
	if val, ok := a.minDistance[cls]; ok {
		return val
	}
	return a.minDistanceDefault
}

// A time and position where we saw an object
type timeAndPosition struct {
	time      time.Time
	detection nn.ProcessedObject
}

// Track whether we have run a frame through the high quality neural network, and what the result was
type validationStatus int

const (
	validationStatusNone    validationStatus = iota // We have not run a high quality NN to verify this object yet
	validationStatusWaiting                         // We are waiting for the HQ network to process a frame containing this object
	validationStatusValid                           // The HQ network has verified this object
	validationStatusInvalid                         // The HQ network did not identify this object
)

// Internal state of an object that we're tracking
type trackedObject struct {
	id                    uint32 // every new tracked object gets a unique id
	firstDetection        nn.ProcessedObject
	cameraWidth           int
	cameraHeight          int
	lastPosition          nn.Rect                           // equivalent to mostRecent().detection.Box, but kept here for convenience/lookup speed
	history               ringbuffer.RingP[timeAndPosition] // unfiltered ring buffer of recent detections
	totalSightings        int                               // Total number of times we've seen this object
	genuine               int                               // Number of frames for which we've considered this object genuine. 0 = not yet, 1 = first time, 2 = second time, etc.
	validation            validationStatus                  // HQ network validation
	sightingsAtValidation int                               // The value of totalSightings when we last ran validation on this object
	validationPosition    nn.Rect                           // Position where object was found by the LQ network, and we want to find the object in the same position in the HQ network
}

// Internal state of the analyzer for a single camera
type analyzerCameraState struct {
	cameraID    int64
	monCam      *monitorCamera
	tracked     []*trackedObject
	lastHQFrame time.Time
	lastSeen    time.Time
}

// SYNC-TIME-AND-POSITION
type TimeAndPosition struct {
	Time       time.Time `json:"-"`
	Box        nn.Rect   `json:"box"`
	Confidence float32   `json:"confidence"`
}

// An object that was detected by the Object Detector, and is now being tracked by a post-process
// SYNC-TRACKED-OBJECT
type TrackedObject struct {
	ID    uint32 `json:"id"`
	Class int    `json:"class"`

	// Number of times we've seen each class.
	// For example: {car: 2, truck: 8}.
	// This is only applicable to objects that are ambiguous - specifically vehicles which will often flop between car and
	// truck as different parts of the vehicle come into view.
	// This was just an idea, but I haven't needed it yet.
	//ClassCount map[int]int `json:"classCount"`

	// The class confidence margin, which is the difference between the confidence of the most likely class
	// and the confidence of the second most likely class.
	// I was initially hoping that this could help assist in rejecting false positives, but it doesn't.
	// Training the correct NN is the only way to truly address false positives.
	ConfidenceMargin float32 `json:"confidenceMargin"`

	// Number of frames that we have considered this object genuine.
	// If Genuine = 0, then we still don't consider it genuine.
	// If Genuine = 1, then this is the first time we consider it genuine.
	// If Genuine > 1, then we've considered it genuine for this many frames.
	Genuine int `json:"genuine"`

	// If Genuine = 1, then Frames contains all the historical frames that we know about.
	// In all other cases, Frames contains only the single most recent frame.
	// Frames is never empty.
	Frames []TimeAndPosition `json:"frames"`
}

func (t *TrackedObject) LastFrame() TimeAndPosition {
	return t.Frames[len(t.Frames)-1]
}

// Result of post-process analysis on the Object Detection neural network output
// SYNC-ANALYSIS-STATE
type AnalysisState struct {
	CameraID int64               `json:"cameraID"`
	Input    *nn.DetectionResult `json:"input"`
	Objects  []TrackedObject     `json:"objects"`
}

// Sent when the alarm is triggered
type AlarmEvent struct {
	CameraID int64
	Time     time.Time
}

func (t *trackedObject) mostRecent() timeAndPosition {
	return t.history.Peek(t.history.Len() - 1)
}

func (t *trackedObject) numDiscreetPositions() int {
	n := 0
	seen := map[int32]bool{}
	for i := 0; i < t.history.Len(); i++ {
		pos := t.history.Peek(i).detection.Raw.Box
		hash := pos.X<<24 + pos.Y<<16 + pos.Width<<8 + pos.Height
		if !seen[hash] {
			n++
			seen[hash] = true
		}
	}
	return n
}

func (t *trackedObject) distanceFromOrigin() float32 {
	return t.firstDetection.Raw.Box.Center().Distance(t.mostRecent().detection.Raw.Box.Center())
}

func (t *trackedObject) averageConfidence() float32 {
	avg := float32(0)
	count := t.history.Len()
	for i := 0; i < count; i++ {
		avg += t.history.Peek(i).detection.Raw.Confidence
	}
	return avg / float32(count)
}

func nextPowerOf2(n int) int {
	return 1 << int(math.Ceil(math.Log2(float64(n))))
}

func (m *Monitor) analyzer() {
	camStates := map[*monitorCamera]*analyzerCameraState{}
	m.Log.Infof("Analyzer starting")

	for {
		qItem, ok := <-m.analyzerQueue
		if !ok {
			break
		}
		// Note! Monitor will recreate it's monitorCamera objects whenever SetCameras() is called.
		// That's why we use *monitorCamera as the key of our camStates map.
		anzCam := camStates[qItem.monCam]
		if anzCam == nil {
			anzCam = &analyzerCameraState{
				cameraID: qItem.monCam.camera.ID(),
				monCam:   qItem.monCam,
			}
			camStates[qItem.monCam] = anzCam
		}
		m.analyzeFrame(anzCam, qItem)
		anzCam.lastSeen = time.Now()

		// Delete cameras that we haven't seen in a while
		for camID, state := range camStates {
			if time.Now().Sub(state.lastSeen) > time.Minute {
				delete(camStates, camID)
			}
		}
	}
	m.Log.Infof("Analyzer stopped")
	m.analyzerStopped <- true
}

// Create abstract objects for each detection, based on nnClassAbstract.
// For example, car -> vehicle, truck -> vehicle, etc.
func (m *Monitor) createAbstractObjects(objects []nn.ObjectDetection) []nn.ProcessedObject {
	processed := []nn.ProcessedObject{}
	orgLen := len(objects)
	for i := 0; i < orgLen; i++ {
		// Always add the original, regardless of whether it maps to an abstract class or not.
		// If we didn't do this, we'd be throwing away information (eg was the vehicle a car or a truck).
		processed = append(processed, nn.ProcessedObject{
			Raw:   objects[i],
			Class: objects[i].Class,
		})

		abstractClass := m.nnClassAbstract[m.nnClassList[objects[i].Class]]
		if abstractClass != "" {
			//fmt.Printf("abstractClass %v -> %v\n", m.nnClassList[objects[i].Class], abstractClass)
			abstractIdx, ok := m.nnClassMap[abstractClass]
			if !ok {
				panic("Abstract class not found in nnClassMap")
			}
			processed = append(processed, nn.ProcessedObject{
				Raw:   objects[i],
				Class: abstractIdx,
			})
		}
	}
	return processed
}

// Send a frame to the HQ model
func (m *Monitor) sendFrameForValidation(cam *analyzerCameraState, item analyzerQueueItem) {
	if m.analyzerSettings.verbose {
		m.Log.Infof("Analyzer (cam %v): Sending frame %v for validation", cam.cameraID, item.imgID)
	}
	m.nnThreadQueue <- monitorQueueItem{
		isHQ:     true,
		imgID:    item.imgID,
		monCam:   cam.monCam,
		yuv:      item.yuv,
		rgb:      item.rgb,
		framePTS: item.detection.FramePTS,
	}
}

func (m *Monitor) analyzeFrame(cam *analyzerCameraState, item analyzerQueueItem) {
	settings := &m.analyzerSettings
	framePTS := item.detection.FramePTS
	now := time.Now()

	// New abstract class strategy:
	// Delay processing of abstract classes until later
	processed := make([]nn.ProcessedObject, len(item.detection.Objects))
	for i := 0; i < len(item.detection.Objects); i++ {
		processed[i] = nn.ProcessedObject{
			Raw:   item.detection.Objects[i],
			Class: item.detection.Objects[i].Class,
		}
	}

	// Keep all
	//keepDetections := make([]int, len(item.detection.Objects))
	//for i := 0; i < len(item.detection.Objects); i++ {
	//	keepDetections[i] = i
	//}

	keepDetections := nn.MergeSimilarObjects(processed, m.nnClassBoxMerge, m.nnClassList, 0.9)

	/*
		// Create abstract objects before merging, because this tends to create duplicates.
		// For example, you'll often get a car and a truck detection of the same object.
		processed := m.createAbstractObjects(item.detection.Objects)

		// If a small pickup ends up producing a car and a truck with very similar boxes, and we create two
		// abstract vehicle objects out of those, then delete one of those vehicles, so that we only
		// end up with one vehicle.
		// MergeSimilarObjects() was my first stab at this, but that was before introducing the concept
		// of abstract classes.
		keepDetections := nn.MergeSimilarAbstractObjects(processed, m.nnAbstractClassSet, 0.9)

		// Merge objects together such as 'car' and 'truck' if they have tight overlap
		// NOTE: I've removed this after implementing abstract classes.
		// Abstract classes seem like a more robust approach.
		//keepDetections := nn.MergeSimilarObjects(objects, m.nnClassBoxMerge, m.nnClassList, 0.9)
	*/

	// Discard detections of classes that we're not interested in
	shortList := make([]int, 0, 100)
	if includeAllClasses {
		shortList = keepDetections
	} else {
		for _, i := range keepDetections {
			if m.nnClassFilterSet[m.nnClassList[processed[i].Class]] {
				shortList = append(shortList, i)
			}
		}
	}

	// Sort from largest to smallest, and retain only the top N
	if len(shortList) > settings.maxAnalyzeObjectsPerFrame {
		sort.Slice(shortList, func(i, j int) bool {
			return processed[shortList[i]].Raw.Box.Area() > processed[shortList[j]].Raw.Box.Area()
		})
		shortList = shortList[:settings.maxAnalyzeObjectsPerFrame]
	}

	filteredProcessed := []nn.ProcessedObject{}
	for _, i := range shortList {
		filteredProcessed = append(filteredProcessed, processed[i])
	}
	processed = filteredProcessed

	// Map every detected/processed object to an existing tracked object.
	// If there is no match, then create a new tracked object.
	m.trackDetectedObjects(cam, processed, item.isHQ, item.imgID, item.detection.ImageWidth, item.detection.ImageHeight, framePTS)

	// Upgrade objects from genuine = 0 to genuine = 1, if sufficient criteria is met.
	// If necessary, schedule this frame for further analysis by the HQ network.
	m.investigateGenuineness(cam, item, now)

	// Handle objects that have disappeared
	remaining := []*trackedObject{}
	for _, tracked := range cam.tracked {
		elapsed := framePTS.Sub(tracked.mostRecent().time)
		if elapsed > settings.objectForgetTime &&
			(tracked.validation != validationStatusWaiting || elapsed > time.Minute) {
			m.analyzeDisappearedObject(cam, tracked)
		} else {
			remaining = append(remaining, tracked)
		}
	}
	cam.tracked = remaining

	// Publish results
	result := &AnalysisState{
		CameraID: cam.cameraID,
		Objects:  make([]TrackedObject, 0), // non-nil, so that we always get an array in our JSON output
		Input:    item.detection,
	}
	for _, tracked := range cam.tracked {
		obj := TrackedObject{
			ID:      tracked.id,
			Class:   tracked.firstDetection.Class,
			Genuine: tracked.genuine,
		}
		// In the default case (not genuine, or was already genuine previously), send only the most recent frame
		startFrame := tracked.history.Len() - 1
		if tracked.genuine == 1 {
			// If this is the first time that the object is considered genuine, then send all frames
			startFrame = 0
		}
		for i := startFrame; i < tracked.history.Len(); i++ {
			pos := tracked.history.Peek(i)
			obj.Frames = append(obj.Frames, TimeAndPosition{
				Time:       pos.time,
				Box:        pos.detection.Raw.Box,
				Confidence: pos.detection.Raw.Confidence,
			})
		}
		result.Objects = append(result.Objects, obj)
	}
	cam.monCam.lock.Lock()
	//fmt.Printf("cam.camera.analyzerState = result (%v). %p = %p\n", cam.cameraID, cam.camera, result)
	cam.monCam.analyzerState = result
	cam.monCam.lock.Unlock()

	m.sendToWatchers(result)
}

// Decide what to do with an object that has disappeared
func (m *Monitor) analyzeDisappearedObject(cam *analyzerCameraState, tracked *trackedObject) {
	center := tracked.mostRecent().detection.Raw.Box.Center()
	distance := tracked.distanceFromOrigin()
	if m.analyzerSettings.verbose {
		m.Log.Infof("Analyzer (cam %v): '%v' at %v,%v disappeared, after moving %.1f pixels, %v discreet positions",
			cam.cameraID, m.nnClassList[tracked.firstDetection.Class], center.X, center.Y, distance, tracked.numDiscreetPositions())
	}
}
