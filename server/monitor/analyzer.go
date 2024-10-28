package monitor

import (
	"math"
	"sort"
	"time"

	"github.com/bmharper/ringbuffer"
	"github.com/cyclopcam/cyclops/pkg/nn"
)

/*
YOLOv7-tiny will often produce two overlapping boxes for a single thing.
We don't make any attempt to filter that out, but hope instead that we can
move toward larger models.

AutoRecorder:
The job of the auto recorder is to record video samples of interesting events.
An interest event includes an item of interest (primarily a person, but perhaps
also a car or bicycle). That item of interest has a certain trajectory through
the frame. The trajectory is simply a 2D path. We don't need too many instances
of similar trajectories, so once we have a few samples of a given kind, we can
stop collecting more of the same.
But this makes me wonder if this approach really works. Most people don't actually
want to simulate breaking into their own yards. It's a big thing to ask of some
people, and even the athletic ones might not cover cases, especially unpleasant ones,
that involve damaging their walls, plants, or are dangerous.
I'm looking at my own cameras, and thinking that good old polygon zones are probably
the right solution.

*/

// If true, then alert on all classes in the COCO set
// If false, then only alert on the classes in defaultNNClassFilter()
const includeAllClasses = false

type analyzerSettings struct {
	positionHistorySize         int            // Keep a ring buffer of the last N positions of each object
	maxAnalyzeObjectsPerFrame   int            // Maximum number of objects to analyze per frame
	minDistanceForObject        float32        // Minimum distance that an object must travel to be considered a true detection (as a fraction of the frame width)
	minDiscreetPositions        map[string]int // For each class, the minimum number of discreet positions that an object must have to be considered a true detection
	minDiscreetPositionsDefault int            // The default minimum number of discreet positions that an object must have to be considered a true detection
	objectForgetTime            time.Duration  // After this amount of time of not seeing an object, we believe it has left the frame, or was a false detection
	verbose                     bool           // Print out debug information
}

func (a *analyzerSettings) minDiscreetPositionsForClass(cls string) int {
	if val, ok := a.minDiscreetPositions[cls]; ok {
		return val
	}
	return a.minDiscreetPositionsDefault
}

// A time and position where we saw an object
type timeAndPosition struct {
	time      time.Time
	detection nn.ObjectDetection
}

// Internal state of an object that we're tracking
type trackedObject struct {
	id             uint32 // every new tracked object gets a unique id
	firstDetection nn.ObjectDetection
	cameraWidth    int
	cameraHeight   int
	lastPosition   nn.Rect // equivalent to mostRecent().detection.Box, but kept here for convenience/lookup speed
	history        ringbuffer.RingP[timeAndPosition]
	genuine        bool // True if we're convinced this is a genuine detection
}

// Internal state of the analyzer for a single camera
type analyzerCameraState struct {
	cameraID int64
	monCam   *monitorCamera
	tracked  []*trackedObject
	lastSeen time.Time
}

// An object that was detected by the Object Detector, and is now being tracked by a post-process
// SYNC-TRACKED-OBJECT
type TrackedObject struct {
	ID         uint32    `json:"id"`
	LastSeen   time.Time `json:"lastSeen"`
	Box        nn.Rect   `json:"box"`
	Class      int       `json:"class"`
	Genuine    bool      `json:"genuine"`
	Confidence float32   `json:"confidence"`
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
	seen := map[int32]bool{}
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

func (t *trackedObject) averageConfidence() float32 {
	avg := float32(0)
	count := t.history.Len()
	for i := 0; i < count; i++ {
		avg += t.history.Peek(i).detection.Confidence
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
func (m *Monitor) createAbstractObjects(objects []nn.ObjectDetection) []nn.ObjectDetection {
	orgLen := len(objects)
	for i := 0; i < orgLen; i++ {
		abstractClass := m.nnClassAbstract[m.nnClassList[objects[i].Class]]
		if abstractClass != "" {
			//fmt.Printf("abstractClass %v -> %v\n", m.nnClassList[objects[i].Class], abstractClass)
			abstractIdx, ok := m.nnClassMap[abstractClass]
			if !ok {
				panic("Abstract class not found in nnClassMap")
			}
			objects = append(objects, nn.ObjectDetection{
				Class:         abstractIdx,
				ConcreteClass: objects[i].Class,
				Confidence:    objects[i].Confidence,
				Box:           objects[i].Box,
			})
		}
	}
	return objects
}

func (m *Monitor) analyzeFrame(cam *analyzerCameraState, item analyzerQueueItem) {
	settings := &m.analyzerSettings
	itemPTS := item.detection.FramePTS
	positionHistorySize := nextPowerOf2(settings.positionHistorySize)

	// Create abstract objects before merging, because this tends to create duplicates.
	// For example, you'll often get a car and a truck detection of the same object.
	objects := m.createAbstractObjects(item.detection.Objects)

	// If a small pickup ends up producing a car and a truck with very similar boxes, and we create two
	// abstract vehicle objects out of those, then delete one of those vehicles, so that we only
	// end up with one vehicle.
	// MergeSimilarObjects() was my first stab at this, but that was before introducing the concept
	// of abstract classes.
	keepDetections := nn.MergeSimilarAbstractObjects(objects, m.nnAbstractClassSet, 0.9)

	// Merge objects together such as 'car' and 'truck' if they have tight overlap
	//keepDetections := nn.MergeSimilarObjects(objects, m.nnClassBoxMerge, m.nnClassList, 0.9)

	//keepDetections := make([]int, len(objects))
	//for i := range objects {
	//	keepDetections[i] = i
	//}

	// Discard detections of classes that we're not interested in
	shortList := make([]int, 0, 100)
	if includeAllClasses {
		shortList = keepDetections
	} else {
		for _, i := range keepDetections {
			if m.nnClassFilterSet[m.nnClassList[objects[i].Class]] {
				shortList = append(shortList, i)
			}
		}
	}

	// Sort from largest to smallest, and retain only the top N
	if len(shortList) > settings.maxAnalyzeObjectsPerFrame {
		sort.Slice(shortList, func(i, j int) bool {
			return objects[shortList[i]].Box.Area() > objects[shortList[j]].Box.Area()
		})
		shortList = shortList[:settings.maxAnalyzeObjectsPerFrame]
	}

	// Greedily find the closest tracked object, but if there is no match with
	// a high enough IOU, then create a new tracked object.
	previousHasMatch := make([]bool, len(cam.tracked))
	for _, i := range shortList {
		det := objects[i]
		// Check if this detection is already in the recentDetections list
		bestJ := -1
		bestIOU := float32(0)
		bestDistance := float32(9e20)
		for j, tracked := range cam.tracked {
			if !previousHasMatch[j] && det.Class == tracked.firstDetection.Class {
				iou := det.Box.IOU(tracked.lastPosition)
				distance := det.Box.Center().Distance(tracked.lastPosition.Center())
				// We allow objects to have zero overlap, because our effective framerate (i.e. NN framerate)
				// is often low enough that an object can move a significant distance between frames, so much
				// that the boxes don't overlap at all.
				// So if iou is zero, then we fall back to distance between rectangle centers.
				if iou > bestIOU {
					bestIOU = iou
					bestJ = j
				} else if bestIOU == 0 && distance < bestDistance {
					bestDistance = distance
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
			objectID := m.nextTrackedObjectID.Next()
			cam.tracked = append(cam.tracked, &trackedObject{
				id:             objectID,
				firstDetection: det,
				history:        ringbuffer.NewRingP[timeAndPosition](positionHistorySize),
				cameraWidth:    item.detection.ImageWidth,
				cameraHeight:   item.detection.ImageHeight,
			})
			if m.analyzerSettings.verbose {
				m.Log.Infof("Analyzer (cam %v): New '%v' at %v,%v", cam.cameraID, m.nnClassList[det.Class], det.Box.Center().X, det.Box.Center().Y)
			}
		}
		cam.tracked[bestJ].lastPosition = det.Box
		cam.tracked[bestJ].history.Add(timeAndPosition{
			time:      itemPTS,
			detection: det,
		})
	}

	// Figure out if any of our tracked objects are genuine
	for _, tracked := range cam.tracked {
		if !tracked.genuine &&
			tracked.distanceFromOrigin() > settings.minDistanceForObject*float32(tracked.cameraWidth) &&
			tracked.numDiscreetPositions() > settings.minDiscreetPositionsForClass(m.nnClassList[tracked.firstDetection.Class]) {
			if m.analyzerSettings.verbose {
				center := tracked.mostRecent().detection.Box.Center()
				m.Log.Infof("Analyzer (cam %v): Genuine '%v' at %v,%v (%.1f px, %v positions)", cam.cameraID, m.nnClassList[tracked.firstDetection.Class],
					center.X, center.Y, tracked.distanceFromOrigin(), tracked.numDiscreetPositions())
			}
			tracked.genuine = true
		}
	}

	// Handle objects that have disappeared
	remaining := []*trackedObject{}
	for _, tracked := range cam.tracked {
		if itemPTS.Sub(tracked.mostRecent().time) > settings.objectForgetTime {
			m.analyzeDisappearedObject(cam, tracked)
		} else {
			remaining = append(remaining, tracked)
		}
	}
	cam.tracked = remaining

	// Publish results so that live feed can display them in the app.
	// This is useful for debugging the analyzer, and people just like to see it operate.
	// In addition, this goes into the event database. It's not just for live viewing!
	result := &AnalysisState{
		CameraID: cam.cameraID,
		Objects:  make([]TrackedObject, 0), // non-nil, so that we always get an array in our JSON output
		Input:    item.detection,
	}
	for _, tracked := range cam.tracked {
		mostRecent := tracked.mostRecent()
		obj := TrackedObject{
			ID:         tracked.id,
			Class:      tracked.firstDetection.Class,
			Box:        mostRecent.detection.Box,
			Genuine:    tracked.genuine,
			Confidence: tracked.averageConfidence(),
			LastSeen:   mostRecent.time,
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
	center := tracked.mostRecent().detection.Box.Center()
	distance := tracked.distanceFromOrigin()
	if m.analyzerSettings.verbose {
		m.Log.Infof("Analyzer (cam %v): '%v' at %v,%v disappeared, after moving %.1f pixels, %v discreet positions",
			cam.cameraID, m.nnClassList[tracked.firstDetection.Class], center.X, center.Y, distance, tracked.numDiscreetPositions())
	}
}
