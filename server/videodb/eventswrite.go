package videodb

import (
	"time"

	"github.com/cyclopcam/cyclops/pkg/dbh"
	"github.com/cyclopcam/cyclops/pkg/gen"
	"github.com/cyclopcam/cyclops/pkg/nn"
)

type TrackedObject struct {
	ID       uint32
	Camera   uint32
	Class    uint32
	Boxes    []TrackedBox
	LastSeen time.Time // In case you're not updating Boxes, or Boxes is empty. Maybe you're not updating Boxes because the object hasn't moved.
}

// Returns the min/max observed time of this object.
// We can have any mix of Boxes and LastSeen, but if none of them are set,
// then we return time.Time{} for both.
func (t *TrackedObject) TimeBounds() (time.Time, time.Time) {
	firstSeen := t.LastSeen
	lastSeen := t.LastSeen
	if t.LastSeen.IsZero() {
		if len(t.Boxes) == 0 {
			return time.Time{}, time.Time{}
		}
		firstSeen = t.Boxes[0].Time
		lastSeen = t.Boxes[len(t.Boxes)-1].Time
	}
	if len(t.Boxes) != 0 {
		if t.Boxes[0].Time.Before(firstSeen) {
			firstSeen = t.Boxes[0].Time
		}
		if t.Boxes[len(t.Boxes)-1].Time.After(lastSeen) {
			lastSeen = t.Boxes[len(t.Boxes)-1].Time
		}
	}
	return firstSeen, lastSeen
}

type TrackedBox struct {
	Time time.Time
	Box  nn.Rect
}

// This is the way our users inform us of a new object detection.
// We'll get one of these calls on every frame where an object is detected.
// id must be unique enough that by the time it wraps around, the previous
// object is no longer in frame.
// Also, id must be unique across cameras.
// This is currently the way our 'monitor' package works, but I'm just codifying it here.
func (v *VideoDB) ObjectDetected(camera string, id uint32, box nn.Rect, class string, lastSeen time.Time) {
	// See comments above addBoxToTrackedObject for why we split this into two phases.
	trackedObjectCopy, err := v.addBoxToTrackedObject(camera, id, box, class, lastSeen)
	if err == nil {
		v.updateTilesWithNewDetection(&trackedObjectCopy)
	}
}

// Phase 1, where we hold currentLock and update our internal state.
// We return a shallow copy of the TrackedObject. This shallow copy does not have the Box history,
// because that is a potentially expensive copy, an we don't need that for our tile update.
// Our goal with splitting this into two phases is to get out of 'currentLock' before passing
// control onto the tile updater.
func (v *VideoDB) addBoxToTrackedObject(camera string, id uint32, box nn.Rect, class string, lastSeen time.Time) (TrackedObject, error) {
	v.currentLock.Lock()
	defer v.currentLock.Unlock()

	obj := v.current[id]

	if obj == nil {
		ids, err := v.StringsToID([]string{camera, class})
		if err != nil {
			v.log.Errorf("Failed to convert strings to ID: %v", err)
			return TrackedObject{}, err
		}
		cameraID, classID := ids[0], ids[1]

		obj = &TrackedObject{
			ID:       id,
			Camera:   cameraID,
			Class:    classID,
			LastSeen: lastSeen,
		}
		v.current[id] = obj
	}

	// Ignore boxes if they move less than this many pixels
	minMovement := 1

	if len(obj.Boxes) == 0 || obj.Boxes[len(obj.Boxes)-1].Box.MaxDelta(box) > minMovement {
		obj.Boxes = append(obj.Boxes, TrackedBox{
			Time: lastSeen,
			Box:  box,
		})
	}

	obj.LastSeen = lastSeen

	return TrackedObject{
		ID:       obj.ID,
		Camera:   obj.Camera,
		Class:    obj.Class,
		LastSeen: obj.LastSeen,
	}, nil
}

func (v *VideoDB) eventWriteThread() {
	v.log.Infof("Event write thread starting")
	keepRunning := true
	wakeInterval := 31 * time.Second
	for keepRunning {
		select {
		case <-v.shutdown:
			keepRunning = false
		case <-time.After(wakeInterval):
			v.writeAgingEventsToDB(false)
			// TODO: Figure out how old our FSV archive is, and keep the events in here in check with that.
			// It's a bit tricky, because the FSV archive limit is specified in bytes, not in seconds.
			// So we basically need to ask FSV how old the oldest file is, and then delete events
			// that are older than that.
			// It's pointless keeping events around when we've already deleted the camera footage.
		}
	}
	v.log.Infof("Flushing events")
	v.writeAgingEventsToDB(true)
	v.log.Infof("Event write thread exiting")
	close(v.writeThreadClosed)
}

// Determine if now is a good time to write our current state to the DB.
// If force is true, then write all objects to the DB.
func (v *VideoDB) writeAgingEventsToDB(force bool) {
	v.currentLock.Lock()
	defer v.currentLock.Unlock()

	if force {
		cameras := map[uint32]bool{}
		for _, c := range v.current {
			cameras[c.Camera] = true
		}
		for cam := range cameras {
			v.flushCameraToDB(cam)
		}
		return
	}

	now := time.Now()

	// Stale = object has not been seen for X seconds
	staleTimeout := 30 * time.Second

	// Old = object was first seen X seconds ago
	// oldTimeout defines the upper limit on how long Event objects will be in our database.
	// One reason we have this limit, is that in the event of a power outage, we would
	// have a decent chance of having written a long-running detection to disk. Imagine a
	// car parked in a driveway for hours or days. Such a detection would just sit there
	// forever, so having some kind of time limit seems like a good idea.
	oldTimeout := 5 * time.Minute

	// We want to limit the size of each Event record in the DB. I'm not sure if it's best
	// to limit the size of the records, or the max time, so I'm doing both.
	// See TestJSONSize. From that test, each frame is 40 bytes. So 300 * 40 = 12KB,
	// which seems like a reasonable upper limit on record size.
	maxFrames := 300

	// We flush a camera if any of these are true:
	// 1. All objects are stale
	// 2. Any object is old
	// 3. There are too many frames

	// Process each camera separately, because our Event records in the DB are specific
	// to a single camera.
	type cameraInfo struct {
		nObjects      int
		nStaleObjects int
		nOldObjects   int
		nFrames       int
	}

	cameras := make(map[uint32]*cameraInfo)

	for _, c := range v.current {
		cam := cameras[c.Camera]
		if cam == nil {
			cam = &cameraInfo{}
			cameras[c.Camera] = cam
		}
		firstSeen := c.Boxes[0].Time
		lastSeen := c.Boxes[len(c.Boxes)-1].Time
		if now.Sub(lastSeen) > staleTimeout {
			cam.nStaleObjects++
		}
		if now.Sub(firstSeen) > oldTimeout {
			cam.nOldObjects++
		}
		cam.nObjects++
		cam.nFrames += len(c.Boxes)
	}

	for cam, inf := range cameras {
		if inf.nStaleObjects == inf.nObjects || inf.nOldObjects > 0 || inf.nFrames > maxFrames {
			v.log.Infof("Flushing camera %v events to DB (total=%v stale=%v old=%v frames=%v)", cam, inf.nObjects, inf.nStaleObjects, inf.nOldObjects, inf.nFrames)
			v.flushCameraToDB(cam)
		}
	}
}

// Write all current objects to an Event record, and reset our state.
// You must already be holding currentLock before calling this function
func (v *VideoDB) flushCameraToDB(camera uint32) {
	// Find the earliest time. This will be our reference time.
	// Everything in the JSON blob is specified as milliseconds relative to base.
	basetime := time.Now()
	maxtime := time.Time{}
	otherCameraObjects := map[uint32]*TrackedObject{}
	for _, c := range v.current {
		if c.Camera == camera {
			if c.Boxes[0].Time.Before(basetime) {
				basetime = c.Boxes[0].Time
			}
			if c.Boxes[len(c.Boxes)-1].Time.After(maxtime) {
				maxtime = c.Boxes[len(c.Boxes)-1].Time
			}
		} else {
			otherCameraObjects[c.ID] = c
		}
	}

	var detectionsJSON dbh.JSONField[EventDetectionsJSON]
	for _, c := range v.current {
		if c.Camera == camera {
			obj := &ObjectJSON{
				ID:    c.ID,
				Class: c.Class,
			}
			// If appropriate, this would be a good place to filter out objects that are not moving.
			// We have an early filter that discards incoming frames which aren't moving enough,
			// but that filter can only see the past, not the future. At this point we have all samples,
			// so it might be possible to filter out jitter here, which would otherwise be allowed
			// through the early filter. I started on such a filter in compress.go, but did not finish it.
			for _, b := range c.Boxes {
				x1 := int16(gen.Clamp(b.Box.X, -32768, 32767))
				y1 := int16(gen.Clamp(b.Box.Y, -32768, 32767))
				x2 := int16(gen.Clamp(b.Box.X2(), -32768, 32767))
				y2 := int16(gen.Clamp(b.Box.Y2(), -32768, 32767))
				obj.Positions = append(obj.Positions, ObjectPositionJSON{
					Box:  [4]int16{x1, y1, x2, y2},
					Time: int32(b.Time.Sub(basetime).Milliseconds()),
				})
			}
			detectionsJSON.Data.Objects = append(detectionsJSON.Data.Objects, obj)
		}
	}

	ev := &Event{
		Time:       dbh.MakeIntTime(basetime),
		Duration:   int32(maxtime.Sub(basetime).Milliseconds()),
		Camera:     camera,
		Detections: &detectionsJSON,
	}

	if err := v.db.Create(ev).Error; err != nil {
		v.log.Errorf("Failed to write Event to DB: %v", err)
	}

	// Remove tracked objects belonging to 'camera'
	v.current = otherCameraObjects
}
