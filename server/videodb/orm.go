package videodb

import "github.com/cyclopcam/cyclops/pkg/dbh"

// BaseModel is our base class for a GORM model.
// The default GORM Model uses int, but we prefer int64
type BaseModel struct {
	ID int64 `gorm:"primaryKey" json:"id"`
}

// An event is one or more frames of motion or object detection.
// For efficiency sake, we limit events in the database to a max size and duration.
type Event struct {
	BaseModel
	Time       dbh.IntTime                         `json:"time"`       // Start of event
	Duration   int32                               `json:"duration"`   // Duration of event in milliseconds
	Camera     uint32                              `json:"camera"`     // LongLived camera name (via lookup in 'strings' table)
	Detections *dbh.JSONField[EventDetectionsJSON] `json:"detections"` // Objects detected in the event
}

type EventDetectionsJSON struct {
	Objects []*ObjectJSON `json:"objects"` // Objects detected in the event
}

// An object detected by the camera.
type ObjectJSON struct {
	ID        uint32               `json:"id"`       // Can be used to track objects across separate Event records
	Class     uint32               `json:"class"`    // eg "person", "car" (via lookup in 'strings' table)
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
//type EventSummary struct {
//	BaseModel
//	Level   int16       `json:"level"`   // 0=32minute, 1=16minute, 2=8minute, 3=4minute, 4=2minute, 5=1minute, 6=30seconds, 7=15seconds, 8=7.5seconds, 9=3.75seconds
//	Camera  string      `json:"camera"`  // LongLived camera name
//	Time    dbh.IntTime `json:"time"`    // Start time of event segment
//	Classes string      `json:"classes"` // Comma-separated list of classes and counts that were detected, eg "person:3,car:1"
//}

// SYNC-EVENT-TILE-JSON
type EventTile struct {
	Camera uint32 `gorm:"primaryKey;autoIncrement:false" json:"camera"` // LongLived camera name (via lookup in 'strings' table)
	Level  uint32 `gorm:"primaryKey;autoIncrement:false" json:"level"`  // 0 = lowest level
	Start  uint32 `gorm:"primaryKey;autoIncrement:false" json:"start"`  // Start time of tile (unix seconds / (1024 * 2^level))...... Rename to tileIdx?
	Tile   []byte `json:"tile"`                                         // Compressed tile data
}
