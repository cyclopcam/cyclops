package videodb

import (
	"time"

	"github.com/cyclopcam/dbh"
)

// BaseModel is our base class for a GORM model.
// The default GORM Model uses int, but we prefer int64
type BaseModel struct {
	ID int64 `gorm:"primaryKey" json:"id"`
}

// An event is one or more frames of motion or object detection.
// For efficiency sake, we limit events in the database to a max size and duration.
// SYNC-VIDEODB-EVENT
type Event struct {
	BaseModel
	Time       dbh.IntTime                         `json:"time"`       // Start of event
	Duration   int32                               `json:"duration"`   // Duration of event in milliseconds
	Camera     uint32                              `json:"camera"`     // LongLived camera name (via lookup in 'strings' table)
	Detections *dbh.JSONField[EventDetectionsJSON] `json:"detections"` // Objects detected in the event
}

// Return the end time of the event.
func (e *Event) EndTime() time.Time {
	return e.Time.Get().Add(time.Duration(e.Duration) * time.Millisecond)
}

// SYNC-VIDEODB-EVENTDETECTIONS
type EventDetectionsJSON struct {
	Resolution [2]int        `json:"resolution"` // Resolution of the camera on which the detection was run.
	Objects    []*ObjectJSON `json:"objects"`    // Objects detected in the event
}

// An object detected by the camera.
// SYNC-VIDEODB-OBJECT
type ObjectJSON struct {
	ID            uint32               `json:"id"`            // Can be used to track objects across separate Event records
	Class         uint32               `json:"class"`         // eg "person", "car" (via lookup in 'strings' table)
	Positions     []ObjectPositionJSON `json:"positions"`     // Object positions throughout event
	NumDetections int32                `json:"numDetections"` // Total number of detections witnessed for this object, before filtering out irrelevant box movements (eg box jiggling around by a few pixels)
}

// Position of an object in a frame.
// SYNC-VIDEODB-OBJECTPOSITION
type ObjectPositionJSON struct {
	Box        [4]int16 `json:"box"`        // [X1,Y1,X2,Y2]
	Time       int32    `json:"time"`       // Time in milliseconds relative to start of event.
	Confidence float32  `json:"confidence"` // NN confidence of detection (0..1)
}

// SYNC-EVENT-TILE-JSON
type EventTile struct {
	Camera uint32 `gorm:"primaryKey;autoIncrement:false" json:"camera"` // LongLived camera name (via lookup in 'strings' table)
	Level  uint32 `gorm:"primaryKey;autoIncrement:false" json:"level"`  // 0 = lowest level
	Start  uint32 `gorm:"primaryKey;autoIncrement:false" json:"start"`  // Start time of tile (unix seconds / (1024 * 2^level))...... Rename to tileIdx?
	Tile   []byte `json:"tile"`                                         // Compressed tile data
}
