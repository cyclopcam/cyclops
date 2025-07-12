package eventdb

import "github.com/cyclopcam/dbh"

// Type of event (eg arm, disarm, alarm)
type EventType string

const (
	// SYNC-EVENT-TYPES
	EventTypeArm    EventType = "arm"    // Arm the system
	EventTypeDisarm EventType = "disarm" // Disarm the system
	EventTypeAlarm  EventType = "alarm"  // Alarm event, triggered by a camera
)

type AlarmType string

const (
	// SYNC-ALARM-TYPES
	AlarmTypeCameraObject AlarmType = "camera-object" // Camera detected an object
	AlarmTypePanic        AlarmType = "panic"         // Panic button pressed
)

type EventDetailAlarm struct {
	AlarmType AlarmType `json:"alarmType"` // Type of alarm (eg camera object, panic)
	CameraID  int64     `json:"cameraId"`  // ID of the camera that triggered the alarm
}

type EventDetailArm struct {
	UserID   int64  `json:"userId"`   // ID of the user
	DeviceID string `json:"deviceId"` // ID of the device that armed/disarmed the system (eg phone ID)
}

type EventDetail struct {
	Arm   *EventDetailArm   `json:"arm,omitempty"`   // Must be populated for EventTypeArm and EventTypeDisarm
	Alarm *EventDetailAlarm `json:"alarm,omitempty"` // Must be populated for EventTypeAlarm
}

// SYNC-EVENT
type Event struct {
	ID        int64                       `json:"id" gorm:"primaryKey"`
	Time      dbh.IntTime                 `json:"time" gorm:"not null"`
	EventType EventType                   `json:"eventType" gorm:"not null"`
	Detail    *dbh.JSONField[EventDetail] `json:"detail"`
	InCloud   bool                        `json:"inCloud" gorm:"not null"`
}
