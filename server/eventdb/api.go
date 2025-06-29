package eventdb

import (
	"time"

	"github.com/cyclopcam/dbh"
)

func (e *EventDB) Arm(userID int64, deviceID string) error {
	return e.ArmDisarm(true, userID, deviceID)
}

func (e *EventDB) Disarm(userID int64, deviceID string) error {
	return e.ArmDisarm(false, userID, deviceID)
}

func (e *EventDB) ArmDisarm(arm bool, userID int64, deviceID string) error {
	eventType := EventTypeDisarm
	if arm {
		eventType = EventTypeArm
	}
	detail := &EventDetail{
		Arm: &EventDetailArm{
			UserID:   userID,
			DeviceID: deviceID,
		},
	}

	e.alarmLock.Lock()
	e.armed = arm
	if !arm {
		e.alarmTriggered = false
	}
	e.alarmLock.Unlock()

	return e.AddEvent(eventType, detail)
}

func (e *EventDB) IsArmed() bool {
	e.alarmLock.Lock()
	defer e.alarmLock.Unlock()
	return e.armed
}

func (e *EventDB) IsAlarmTriggered() bool {
	e.alarmLock.Lock()
	defer e.alarmLock.Unlock()
	return e.alarmTriggered
}

func (e *EventDB) AddEvent(eventType EventType, detail *EventDetail) error {
	e.alarmLock.Lock()
	if eventType == EventTypeAlarm {
		if e.armed && !e.alarmTriggered {
			e.alarmTriggered = true
		}
	}
	e.alarmLock.Unlock()

	e.purgeOldRecords()

	event := &Event{
		Time:      dbh.MakeIntTime(time.Now()),
		EventType: eventType,
		Detail:    dbh.MakeJSONField(*detail),
		InCloud:   false,
	}
	if err := e.DB.Create(event).Error; err != nil {
		return err
	}
	return nil
}

// Get the list of events that need to be sent to the cloud.
func (e *EventDB) GetCloudQueue() ([]*Event, error) {
	oldest := dbh.MakeIntTime(time.Now().Add(-MaxCloudSendEventAge))
	var events []*Event
	if err := e.DB.Where("in_cloud = ? AND time > ?", false, oldest).Find(&events).Error; err != nil {
		return nil, err
	}
	return events, nil
}

// Mark the given events as being sent to the cloud.
func (e *EventDB) MarkInCloud(eventIDs []int64) error {
	if len(eventIDs) == 0 {
		return nil
	}
	if err := e.DB.Model(&Event{}).Where("id IN ?", eventIDs).Update("in_cloud", true).Error; err != nil {
		return err
	}
	return nil
}
