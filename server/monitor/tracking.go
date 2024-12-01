package monitor

import (
	"time"

	"github.com/bmharper/flatbush-go"
	"github.com/bmharper/ringbuffer"
	"github.com/cyclopcam/cyclops/pkg/nn"
)

// Process incoming objects, and track them spatially.
// objects is the list of objects detected in the current frame.
// When performing tracking on the LQ network, we're very lenient. This is because objects
// can move quite far from frame to frame, especially if the NN framerate is low.
// However, when performing tracking on the HQ network, we're analyzing the exact
// same frame twice. First on the LQ network, and then on the HQ network. So in this case
// we impose reasonably strict spatial matching criteria.
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
	trackedIoU := make([]float32, len(cam.tracked))

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
		if isHQ {
			// For validation match, we require a non-zero IoU.
			// Note that IoU gets small very quickly when pixel sizes are small.
			// The IoU of the two boxes {119 0 16 29} -> {120 0 17 28} (X,Y,W,H) is 0.81.
			// They are off by just 1 pixel in width, height, and X position, and yet their IoU drops down to 0.81.
			bestIOU = 0.2
		}
		for _, j := range existingList {
			if trackedHasMatch[j] {
				continue
			}
			oldObj := cam.tracked[j]
			if oldObj.firstDetection.Class != newObj.Class {
				continue
			}
			oldPosition := oldObj.lastPosition
			if isHQ {
				oldPosition = oldObj.validationPosition
			}
			iou := newObj.Raw.Box.IOU(oldPosition)
			distance := newObj.Raw.Box.Center().Distance(oldPosition.Center())
			// For LQ match:
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
			trackedIoU[bestJ] = bestIOU
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
	// We only run this brute force match on the LQ network.
	if !isHQ {
		for i := range objects {
			if newToTracked[i] != -1 {
				continue
			}
			findClosestObjectFromList(i, unmatched)
		}
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
			if m.analyzerSettings.verbose {
				m.Log.Infof("Analyzer (cam %v): Existing '%v' at %v,%v", cam.cameraID, m.nnClassList[newObj.Class], newObj.Raw.Box.Center().X, newObj.Raw.Box.Center().Y)
			}
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
			obj := cam.tracked[i]
			if trackedAndFound[i] {
				obj.validation = validationStatusValid
			} else {
				obj.validation = validationStatusInvalid
			}

			if m.analyzerSettings.verbose {
				iou := float32(-1)
				if i < len(trackedIoU) {
					// I fully expect i < len(trackedIoU), but this is just defensive coding because I keep letting these arrays get out of sync.
					iou = trackedIoU[i]
				}
				msg := "True Positive"
				if obj.validation == validationStatusInvalid {
					msg = "False Positive"
				}
				m.Log.Infof("Analyzer (cam %v): %v '%v' at %v,%v (IoU %.2f, %v -> %v)",
					cam.cameraID, msg, m.nnClassList[obj.firstDetection.Class], obj.lastPosition.Center().X, obj.lastPosition.Center().Y,
					iou, obj.validationPosition, obj.lastPosition)
			}
		}
	}
}

// Inspect all objects that are not yet considered genuine, and if applicable upgrade them.
func (m *Monitor) investigateGenuineness(cam *analyzerCameraState, item analyzerQueueItem, now time.Time) {
	sendFrameForValidation := false

	// Figure out if any of our tracked objects are genuine, and increment the genuine counter for those that are
	for _, tracked := range cam.tracked {
		makeGenuine := false

		if tracked.genuine == 0 {
			needValidation := false
			makeGenuine, needValidation = m.investigateIfObjectIsGenuine(cam, item, tracked, now)
			sendFrameForValidation = sendFrameForValidation || needValidation
		} else {
			tracked.genuine++
		}

		if makeGenuine {
			//item.rgb.WriteJPEG("false-positive-culprit.jpg", cimg.MakeCompressParams(cimg.Sampling444, 99, 0), 0644) // If you need to analyze the frame where it all went wrong
			if m.analyzerSettings.verbose {
				cls := m.nnClassList[tracked.firstDetection.Class]
				center := tracked.mostRecent().detection.Raw.Box.Center()
				m.Log.Infof("Analyzer (cam %v): Genuine '%v' at %v,%v (%.1f px, %v positions)", cam.cameraID, cls,
					center.X, center.Y, tracked.distanceFromOrigin(), tracked.numDiscreetPositions())
			}
			tracked.genuine = 1
		}
	}

	if sendFrameForValidation {
		m.sendFrameForValidation(cam, item)
	}
}

// Investigate if an object should become genuine
func (m *Monitor) investigateIfObjectIsGenuine(cam *analyzerCameraState, item analyzerQueueItem, tracked *trackedObject, now time.Time) (makeGenuine, sendFrameForValidation bool) {
	settings := &m.analyzerSettings
	cls := m.nnClassList[tracked.firstDetection.Class]

	// This check happens before any of the other decision tree, because it's an obviously correct
	// decision to always make.
	if tracked.validation == validationStatusValid {
		makeGenuine = true
		return
	}

	//m.Log.Infof("distanceFromOrigin %.2f", tracked.distanceFromOrigin())
	if tracked.distanceFromOrigin() >= float32(settings.minDistanceForClass(cls)) &&
		tracked.totalSightings >= settings.minSightingsForClass(cls) {
		// Decide whether this object is genuine, or if we should run validation, etc
		if m.nnDetectorHQ == nil {
			// There is no HQ detector, so this object becomes genuine
			makeGenuine = true
		} else if !item.isHQ {
			// LQ observation of object
			if tracked.validation == validationStatusInvalid && now.Sub(cam.lastHQFrame) > settings.revalidateInterval && tracked.totalSightings > tracked.sightingsAtValidation {
				// Reset validation status, because this object seems to be sticky
				tracked.validation = validationStatusNone
			}
			switch tracked.validation {
			case validationStatusNone:
				sendFrameForValidation = true
				tracked.validation = validationStatusWaiting
				tracked.sightingsAtValidation = tracked.totalSightings
				tracked.validationPosition = tracked.lastPosition
			case validationStatusWaiting:
				// do nothing
			case validationStatusInvalid:
				// generally do nothing, except for the above case where revalidateInterval has lapsed, which we deal with above.
			case validationStatusValid:
				// The check at the top of function where we do "tracked.validation == validationStatusValid" should have caught this.
				m.Log.Errorf("This code in investigateIfObjectIsGenuine should be unreachable")
			}
		}
	}

	return
}
