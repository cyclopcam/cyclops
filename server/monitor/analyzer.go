package monitor

import (
	"math"
	"sort"
	"time"

	"github.com/bmharper/cyclops/server/nn"
	"github.com/bmharper/ringbuffer"
)

// Useful while debugging; include all classes (not just person, car, etc)
const includeAllClasses = true

type analyzerSettings struct {
	positionHistorySize       int           // Keep a ring buffer of the last N positions of each object
	maxAnalyzeObjectsPerFrame int           // Maximum number of objects to analyze per frame
	minDistanceForObject      float32       // Minimum distance that an object must travel to be considered a true detection (as a fraction of the frame width)
	minDiscreetPositions      int           // Minimum number of discreet positions that an object must have to be considered a true detection
	objectForgetTime          time.Duration // After this amount of time of not seeing an object, we believe it has left the frame, or was a false detection
	verbose                   bool          // Print out debug information
}

// A time and position where we saw an object
type timeAndPosition struct {
	time      time.Time
	detection nn.Detection
}

// Internal state of an object that we're tracking
type trackedObject struct {
	id             int64 // every new tracked object gets a unique id
	firstDetection nn.Detection
	cameraWidth    int
	cameraHeight   int
	lastPosition   nn.Rect // equivalent to mostRecent().detection.Box, but kept here for convenience/lookup speed
	history        ringbuffer.RingP[timeAndPosition]
	genuine        bool // True if we're convinced this is a genuine detection
}

// Internal state of the analyzer for a single camera
type analyzerCameraState struct {
	cameraID int64
	camera   *monitorCamera
	tracked  []*trackedObject
	lastSeen time.Time
}

// An object that was detected by the Object Detector, and is now being tracked by a post-process
// SYNC-TRACKED-OBJECT
type TrackedObject struct {
	ID      int64   `json:"id"`
	Class   int     `json:"class"`
	Box     nn.Rect `json:"box"`
	Genuine bool    `json:"genuine"`
}

// Result of post-process analysis on the Object Detection neural network output
// SYNC-ANALYSIS-STATE
type AnalysisState struct {
	CameraID int64               `json:"cameraID"`
	Input    *nn.DetectionResult `json:"input"`
	Objects  []TrackedObject     `json:"objects"`
}

func (t *trackedObject) mostRecent() timeAndPosition {
	return t.history.Peek(t.history.Len() - 1)
}

func (t *trackedObject) numDiscreetPositions() int {
	n := 0
	seen := map[int]bool{}
	for i := 0; i < t.history.Len(); i++ {
		pos := t.history.Peek(i).detection.Box
		hash := pos.X<<24 + pos.Y<<16 + pos.Width<<8 + pos.Height
		if !seen[hash] {
			n++
			seen[hash] = true
		}
	}
	return n
}

func (t *trackedObject) distanceFromOrigin() float32 {
	return t.firstDetection.Box.Center().Distance(t.mostRecent().detection.Box.Center())
}

func nextPowerOf2(n int) int {
	return 1 << int(math.Ceil(math.Log2(float64(n))))
}

func (m *Monitor) analyzer() {
	camStates := map[int64]*analyzerCameraState{} // Camera ID -> state

	for {
		item, ok := <-m.analyzerQueue
		if !ok {
			break
		}
		cam := camStates[item.camera.camera.ID]
		if cam == nil {
			cam = &analyzerCameraState{
				cameraID: item.camera.camera.ID,
				camera:   item.camera,
			}
			camStates[item.camera.camera.ID] = cam
		}
		m.analyzeFrame(cam, item)
		cam.lastSeen = time.Now()

		// Delete cameras that we haven't been seen in a while
		for camID, state := range camStates {
			if time.Now().Sub(state.lastSeen) > time.Hour {
				delete(camStates, camID)
			}
		}
	}
	m.analyzerStopped <- true
}

func (m *Monitor) analyzeFrame(cam *analyzerCameraState, item analyzerQueueItem) {
	settings := &m.analyzerSettings
	now := time.Now()
	positionHistorySize := nextPowerOf2(settings.positionHistorySize)

	// Discard detections of classes that we're not interested in
	shortList := make([]int, 0, 100)
	if includeAllClasses {
		for i := range item.detection.Objects {
			shortList = append(shortList, i)
		}
	} else {
		for i, det := range item.detection.Objects {
			if m.cocoClassFilter[det.Class] {
				shortList = append(shortList, i)
			}
		}
	}

	// Sort from largest to smallest, and retain only the top N
	if len(shortList) > settings.maxAnalyzeObjectsPerFrame {
		sort.Slice(shortList, func(i, j int) bool {
			return item.detection.Objects[shortList[i]].Box.Area() > item.detection.Objects[shortList[j]].Box.Area()
		})
		shortList = shortList[:settings.maxAnalyzeObjectsPerFrame]
	}

	// Greedily find the closest tracked object, but if there is no match with
	// a high enough IOU, then create a new tracked object.
	previousHasMatch := make([]bool, len(cam.tracked))
	for _, i := range shortList {
		det := item.detection.Objects[i]
		// Check if this detection is already in the recentDetections list
		bestJ := -1
		bestIOU := float32(0)
		for j, tracked := range cam.tracked {
			if !previousHasMatch[j] && det.Class == tracked.firstDetection.Class {
				iou := det.Box.IOU(tracked.lastPosition)
				if iou > bestIOU {
					bestIOU = iou
					bestJ = j
				}
			}
		}
		if bestJ != -1 {
			// Update the tracked object
			previousHasMatch[bestJ] = true
		} else {
			// Add a new object
			bestJ = len(cam.tracked)
			previousHasMatch = append(previousHasMatch, true) // keep the slice length the same
			objectID := m.nextTrackedObjectID.Add(1)
			cam.tracked = append(cam.tracked, &trackedObject{
				id:             objectID,
				firstDetection: det,
				history:        ringbuffer.NewRingP[timeAndPosition](positionHistorySize),
				cameraWidth:    item.detection.ImageWidth,
				cameraHeight:   item.detection.ImageHeight,
			})
			if m.analyzerSettings.verbose {
				m.Log.Infof("Analyzer (cam %v): New '%v' at %v,%v", cam.cameraID, nn.COCOClasses[det.Class], det.Box.Center().X, det.Box.Center().Y)
			}
		}
		cam.tracked[bestJ].lastPosition = det.Box
		cam.tracked[bestJ].history.Add(timeAndPosition{
			time:      now,
			detection: det,
		})
	}

	// Figure out if any of our tracked objects are genuine
	for _, tracked := range cam.tracked {
		if !tracked.genuine &&
			tracked.distanceFromOrigin() > settings.minDistanceForObject*float32(tracked.cameraWidth) &&
			tracked.numDiscreetPositions() > settings.minDiscreetPositions {
			if m.analyzerSettings.verbose {
				center := tracked.mostRecent().detection.Box.Center()
				m.Log.Infof("Analyzer (cam %v): Genuine '%v' at %v,%v (%.1f px, %v positions)", cam.cameraID, nn.COCOClasses[tracked.firstDetection.Class],
					center.X, center.Y, tracked.distanceFromOrigin(), tracked.numDiscreetPositions())
			}
			tracked.genuine = true
		}
	}

	// Handle objects that have disappeared
	remaining := []*trackedObject{}
	for _, tracked := range cam.tracked {
		if now.Sub(tracked.mostRecent().time) > settings.objectForgetTime {
			m.analyzeDisappearedObject(cam, tracked)
		} else {
			remaining = append(remaining, tracked)
		}
	}
	cam.tracked = remaining

	// Publish results so that live feed can display them in the app.
	// This is useful for debugging the analyzer.
	result := &AnalysisState{
		CameraID: cam.cameraID,
		Objects:  make([]TrackedObject, 0), // non-nil, so that we always get an array in our JSON output
		Input:    item.detection,
	}
	for _, tracked := range cam.tracked {
		obj := TrackedObject{
			ID:      tracked.id,
			Class:   tracked.firstDetection.Class,
			Box:     tracked.mostRecent().detection.Box,
			Genuine: tracked.genuine,
		}
		result.Objects = append(result.Objects, obj)
	}
	cam.camera.lock.Lock()
	cam.camera.analyzerState = result
	cam.camera.lock.Unlock()

	m.sendToWatchers(result)
}

// Decide what to do with an object that has disappeared
func (m *Monitor) analyzeDisappearedObject(cam *analyzerCameraState, tracked *trackedObject) {
	center := tracked.mostRecent().detection.Box.Center()
	distance := tracked.distanceFromOrigin()
	if m.analyzerSettings.verbose {
		m.Log.Infof("Analyzer (cam %v): '%v' at %v,%v disappeared, after moving %.1f pixels, %v discreet positions",
			cam.cameraID, nn.COCOClasses[tracked.firstDetection.Class], center.X, center.Y, distance, tracked.numDiscreetPositions())
	}
}
