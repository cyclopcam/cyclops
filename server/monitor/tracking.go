package monitor

import (
	"time"

	"github.com/bmharper/flatbush-go"
	"github.com/bmharper/ringbuffer"
	"github.com/cyclopcam/cyclops/pkg/nn"
)

// Process incoming objects, and track them spatially.
// objects is the list of objects detected in the current frame.
func (m *Monitor) trackDetectedObjects(cam *analyzerCameraState, objects []nn.ProcessedObject, isHQ bool, frameWidth, frameHeight int, framePTS time.Time) {
	positionHistorySize := nextPowerOf2(m.analyzerSettings.positionHistorySize)

	// Create spatial index on the currently tracked objects (cam.tracked)
	fb := flatbush.NewFlatbush[int32]()
	fb.Reserve(len(cam.tracked))
	for _, t := range cam.tracked {
		obj := &t.lastPosition
		fb.Add(obj.X, obj.Y, obj.X2(), obj.Y2())
	}
	fb.Finish()

	minSearchBuffer := int32(0.05 * float64(frameWidth))

	// Map from objects[i] to tracked[j]
	newToTracked := make([]int, len(objects))
	for i := 0; i < len(objects); i++ {
		newToTracked[i] = -1
	}

	// trackedHasMatch[j] is true if cam.tracked[j] has been matched to a new object
	trackedHasMatch := make([]bool, len(cam.tracked))

	// Search among cam.tracked (but only the indices in existingList), and find the
	// closest object to 'newObj', which has the same class.
	// Skip over objects that already have a match in trackedHasMatch.
	// Once the closest object is found, populate trackedHasMatch, and newToTracked.
	// Returns the index in cam.tracked of the best match.
	findClosestObjectFromList := func(newIndex int, existingList []int) int {
		newObj := &objects[newIndex]
		bestJ := -1
		bestIOU := float32(0)
		bestDistance := float32(9e20)
		for _, j := range existingList {
			if trackedHasMatch[j] {
				continue
			}
			oldObj := cam.tracked[j]
			if oldObj.firstDetection.Class != newObj.Class {
				continue
			}
			iou := newObj.Raw.Box.IOU(oldObj.lastPosition)
			distance := newObj.Raw.Box.Center().Distance(oldObj.lastPosition.Center())
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
		if bestJ != -1 {
			trackedHasMatch[bestJ] = true
			newToTracked[newIndex] = bestJ
		}
		return bestJ
	}

	// Phase 1:
	// Find existing objects that are reasonably close to the detected object
	nearbyIdx := []int{}
	for i := range objects {
		newObj := &objects[i]
		searchBufferX := max(minSearchBuffer, int32(0.8*float64(newObj.Raw.Box.Width)))
		searchBufferY := max(minSearchBuffer, int32(0.8*float64(newObj.Raw.Box.Height)))
		nearbyIdx = fb.SearchFast(newObj.Raw.Box.X-searchBufferX, newObj.Raw.Box.Y-searchBufferY, newObj.Raw.Box.X2()+searchBufferX, newObj.Raw.Box.Y2()+searchBufferY, nearbyIdx)
		findClosestObjectFromList(i, nearbyIdx)
	}

	// Phase 2:
	// Match detections to *any* current object, no matter how far.
	// This phase is critical, because we use multiple detections of an object
	// to validate that the detection was not a false positive.
	// For example, for "person" we require at least 3 sightings before believing
	// that it's a genuine detection. False positives are just as bad as false
	// negatives. So if the system is underpowered, and can only process 1 FPS,
	// then the person can move quite far between frames. If we were to create
	// a new "person" object for every frame, then we'd never hit our threshold
	// of 3 sightings.
	// NOTE: I'm not convinced that this 2nd phase is the correct thing to do.
	// It feels wrong, but I can't think of a cleaner solution.
	// In principle the best solution is to ensure you have enough NN FPS to cover
	// fast motion, so that subsequent boxes have a decent overlap, but our job
	// is to do the best we can with whatever hardware we get given.

	// Prune the list of existing tracked objects so that we only consider objects
	// that didn't get any matches in the first phase.
	unmatched := []int{}
	for i := 0; i < len(cam.tracked); i++ {
		if !trackedHasMatch[i] {
			unmatched = append(unmatched, i)
		}
	}

	// This is O(n*m), but hopefully by this stage n and m are small.
	for i := range objects {
		if newToTracked[i] != -1 {
			continue
		}
		findClosestObjectFromList(i, unmatched)
	}

	// Final list of all objects in cam.tracked which were found in this frame
	trackedAndFound := make([]bool, len(cam.tracked))

	// Update existing objects, and create new objects
	for i := range objects {
		newObj := &objects[i]
		bestJ := newToTracked[i]
		if bestJ == -1 {
			// Create a new object
			bestJ = len(cam.tracked)
			objectID := m.nextTrackedObjectID.Next()
			cam.tracked = append(cam.tracked, &trackedObject{
				id:             objectID,
				firstDetection: *newObj,
				history:        ringbuffer.NewRingP[timeAndPosition](positionHistorySize),
				cameraWidth:    frameWidth,
				cameraHeight:   frameHeight,
				totalSightings: 0,
			})
			if m.analyzerSettings.verbose {
				m.Log.Infof("Analyzer (cam %v): New '%v' at %v,%v", cam.cameraID, m.nnClassList[newObj.Class], newObj.Raw.Box.Center().X, newObj.Raw.Box.Center().Y)
			}
			trackedAndFound = append(trackedAndFound, true)
		} else {
			trackedAndFound[bestJ] = true
		}

		cam.tracked[bestJ].totalSightings++
		cam.tracked[bestJ].lastPosition = newObj.Raw.Box
		cam.tracked[bestJ].history.Add(timeAndPosition{
			time:      framePTS,
			detection: *newObj,
		})
	}

	if isHQ {
		// Update validation status to either "valid" or "invalid"
		cam.lastHQFrame = time.Now()
		for i := range cam.tracked {
			if trackedAndFound[i] {
				cam.tracked[i].validation = validationStatusValid
			} else {
				cam.tracked[i].validation = validationStatusInvalid
			}

			if m.debugValidation {
				msg := "True Positive"
				obj := cam.tracked[i]
				if obj.validation == validationStatusInvalid {
					msg = "False Positive"
				}
				m.Log.Infof("Analyzer (cam %v): %v '%v' at %v,%v", cam.cameraID, msg, m.nnClassList[obj.firstDetection.Class], obj.lastPosition.Center().X, obj.lastPosition.Center().Y)
			}
		}
	}
}
