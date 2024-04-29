package videodb

import (
	"time"

	"github.com/cyclopcam/cyclops/pkg/dbh"
	"github.com/cyclopcam/cyclops/pkg/gen"
	"github.com/cyclopcam/cyclops/pkg/nn"
)

type TrackedObject struct {
	ID     int64
	Camera string
	Class  string
	Boxes  []TrackedBox
}

type TrackedBox struct {
	Time time.Time
	Box  nn.Rect
}

func (v *VideoDB) ObjectDetected(camera string, id int64, box nn.Rect, class string) {
	v.currentLock.Lock()
	defer v.currentLock.Unlock()

	now := time.Now()
	var obj *TrackedObject
	for _, c := range v.current {
		if c.ID == id && c.Camera == camera {
			obj = c
			break
		}
	}

	if obj == nil {
		obj = &TrackedObject{
			Camera: camera,
			ID:     id,
			Class:  class,
		}
		v.current = append(v.current, obj)
	}

	// Ignore boxes if they move less than this many pixels
	minMovement := 1

	if len(obj.Boxes) == 0 || obj.Boxes[len(obj.Boxes)-1].Box.MaxDelta(box) > minMovement {
		obj.Boxes = append(obj.Boxes, TrackedBox{
			Time: now,
			Box:  box,
		})
	}
}

func (v *VideoDB) eventWriteThread() {
	v.log.Infof("Event write thread starting")
	keepRunning := true
	wakeInterval := 30 * time.Second
	for keepRunning {
		select {
		case <-v.shutdown:
			keepRunning = false
		case <-time.After(wakeInterval):
			v.writeOldObjects(false)
		}
	}
	v.log.Infof("Flushing events")
	v.writeOldObjects(true)
	v.log.Infof("Event write thread exiting")
	close(v.writeThreadClosed)
}

// Determine if now is a good time to write our current state to the DB.
// If force is true, then write all objects to the DB.
func (v *VideoDB) writeOldObjects(force bool) {
	v.currentLock.Lock()
	defer v.currentLock.Unlock()

	if force {
		cameras := map[string]bool{}
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

	cameras := make(map[string]*cameraInfo)

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
func (v *VideoDB) flushCameraToDB(camera string) {
	// Find the earliest time. This will be our reference time.
	// Everything in the JSON blob is specified as milliseconds relative to base.
	basetime := time.Now()
	maxtime := time.Time{}
	otherCameraObjects := []*TrackedObject{}
	for _, c := range v.current {
		if c.Camera == camera {
			if c.Boxes[0].Time.Before(basetime) {
				basetime = c.Boxes[0].Time
			}
			if c.Boxes[len(c.Boxes)-1].Time.After(maxtime) {
				maxtime = c.Boxes[len(c.Boxes)-1].Time
			}
		} else {
			otherCameraObjects = append(otherCameraObjects, c)
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
