package videodb

import "github.com/cyclopcam/cyclops/pkg/dbh"

// BaseModel is our base class for a GORM model.
// The default GORM Model uses int, but we prefer int64
type BaseModel struct {
	ID int64 `gorm:"primaryKey" json:"id"`
}

// An event is one or more frames of motion or object detection.
// For efficiency sake, we limit events in the database to 5 seconds.
type Event struct {
	BaseModel
	Time       dbh.IntTime                         `json:"time"`       // Start of event
	Duration   int32                               `json:"duration"`   // Duration of event in milliseconds
	Camera     string                              `json:"camera"`     // LongLived camera name
	Detections *dbh.JSONField[EventDetectionsJSON] `json:"detections"` // Objects detected in the event
}

type EventDetectionsJSON struct {
	Objects []*ObjectJSON `json:"objects"` // Objects detected in the event
}

// An object detected by the camera.
type ObjectJSON struct {
	ID        int64                `json:"id"`       // Can be used to track objects across Events
	Class     string               `json:"class"`    // eg "person", "car"
	Positions []ObjectPositionJSON `json:"position"` // Object positions throughout event
}

// Position of an object in a frame.
type ObjectPositionJSON struct {
	Box  [4]int16 `json:"box"`  // [X1,Y1,X2,Y2]
	Time int32    `json:"time"` // Time in milliseconds relative to start of event.
}

// An EventSummary record is stored for every 5 minute segment, in order to quickly
// produce a zoomed-out view of activity within a day, week, or month.
// If this record takes 40 bytes, and we have 288 5-minute segments in a day,
// then we use 11.5KB per camera per day. Per month, that is 345KB.
// During transmission (over the web), we can compress this down significantly, by
// sending only the "classes" list as a dense array, which would bring this information
// down by a factor of 10 or more. If the classes were single-digit integers, then
// we'd have 1 byte for every day separator, and let's say 3 'classes' bytes per segment,
// for 4 bytes total per segment. 4 * 288 = 1.15KB per day, or 34.5KB per month (per camera).
// This would compress very well, so we're probably looking at about 300 bytes per day,
// or 10KB per month, per camera.
type EventSummary struct {
	BaseModel
	Camera  string      `json:"camera"`  // LongLived camera name
	Time    dbh.IntTime `json:"time"`    // Start time of event segment
	Classes string      `json:"classes"` // Comma-separated list of classes that were detected
}
