package eventdb

import "github.com/cyclopcam/dbh"

/*
// Is the system armed or not
type ArmState string

const (
	ArmStateArmed    ArmState = "armed"    // System is armed, alarm is active
	ArmStateDisarmed ArmState = "disarmed" // System is disarmed
)
*/

// Type of event (eg arm, disarm, alarm)
type EventType string

const (
	EventTypeArm    EventType = "arm"    // Arm the system
	EventTypeDisarm EventType = "disarm" // Disarm the system
	EventTypeAlarm  EventType = "alarm"  // Alarm event, triggered by a camera
)

type EventDetailAlarm struct {
	CameraID int64 `json:"cameraId"` // ID of the camera that triggered the alarm
}

type EventDetailArm struct {
	UserID   int64  `json:"userId"`   // ID of the user
	DeviceID string `json:"deviceId"` // ID of the device that armed the system (eg phone ID)
}

type EventDetail struct {
	Arm   *EventDetailArm   `json:"arm,omitempty"`
	Alarm *EventDetailAlarm `json:"alarm,omitempty"`
}

/*
type Arm struct {
	ID     int64       `gorm:"primaryKey"`
	Time   dbh.IntTime `gorm:"not null"`
	UserID int64       `gorm:"not null"`
	State  ArmState    `gorm:"not null"`
}
*/

type Event struct {
	ID        int64       `gorm:"primaryKey"`
	Time      dbh.IntTime `gorm:"not null"`
	EventType EventType   `gorm:"not null"`
	Detail    *dbh.JSONField[EventDetail]
	InCloud   bool `gorm:"not null"`
}
