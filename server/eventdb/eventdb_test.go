package eventdb

import (
	"os"
	"testing"

	"github.com/cyclopcam/logs"
	"github.com/stretchr/testify/require"
)

func setup(t *testing.T, wipeDB bool) *EventDB {
	t.Helper()
	if wipeDB {
		os.Remove("test_eventdb.sqlite")
	}
	db, err := NewEventDB(logs.NewTestingLog(t), "test_eventdb.sqlite")
	if err != nil {
		t.Fatalf("Failed to create EventDB: %v", err)
	}
	return db
}

func cleanupDB(t *testing.T) {
	t.Helper()
	os.Remove("test_eventdb.sqlite")
	os.Remove("test_eventdb.sqlite-shm")
	os.Remove("test_eventdb.sqlite-wal")
}

func TestEventDB(t *testing.T) {
	db := setup(t, true)

	require.False(t, db.IsArmed())
	require.False(t, db.IsAlarmTriggered())
	q, err := db.GetCloudQueue()
	require.NoError(t, err)
	require.Empty(t, q)

	// Arm the device
	require.NoError(t, db.Arm(1, "device1"))
	require.True(t, db.IsArmed())
	require.False(t, db.IsAlarmTriggered())

	// Open a 2nd DB and ensure that we recognize the armed state.
	db2 := setup(t, false)
	require.True(t, db2.IsArmed())

	// Trigger the alarm
	require.NoError(t, db.AddEvent(EventTypeAlarm, &EventDetail{
		Alarm: &EventDetailAlarm{
			CameraID: 11,
		},
	}))
	require.True(t, db.IsAlarmTriggered())

	// Open a 3rd DB and ensure that we recognize the triggered state
	db3 := setup(t, false)
	require.True(t, db3.IsAlarmTriggered())

	// Ensure cloud queue is not empty
	q, err = db.GetCloudQueue()
	require.NoError(t, err)
	require.Len(t, q, 2)

	// Mark events as sent to cloud
	require.NoError(t, db.MarkInCloud([]int64{q[0].ID, q[1].ID}))
	q, err = db.GetCloudQueue()
	require.NoError(t, err)
	require.Empty(t, q)

	// Disarm
	require.NoError(t, db.Disarm(2, "device2"))
	require.False(t, db.IsArmed())
	require.False(t, db.IsAlarmTriggered())

	// Create many records, and test purging of old records.
	db.maxEventCount = 10
	for i := 0; i < 100; i++ {
		require.NoError(t, db.AddEvent(EventTypeAlarm, &EventDetail{Alarm: &EventDetailAlarm{CameraID: 11}}))
		count := int64(0)
		db.DB.Model(&Event{}).Count(&count)
		require.LessOrEqual(t, count, db.maxEventCount+5)
	}

	cleanupDB(t)
}
