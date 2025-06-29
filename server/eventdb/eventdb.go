package eventdb

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/cyclopcam/dbh"
	"github.com/cyclopcam/logs"
	"gorm.io/gorm"
)

// If we're resyncing with the cloud, then ignore events older than this.
// If this was infinite, and a system was connected to the cloud for the first time,
// then there could be a gigantic backlog. So we need to limit this.
const MaxCloudSendEventAge = 24 * time.Hour

// Maximum number of events to keep in the database.
const MaxEventCount = 100000

// EventDB tracks high level events, such as alarm activations
type EventDB struct {
	Log logs.Log
	DB  *gorm.DB

	alarmLock      sync.Mutex // Guards access to armed as well as reading/writing the armed state to the DB, and alarm state
	armed          bool       // True if the system is currently armed
	alarmTriggered bool       // True if the alarm is active (siren blaring, calling for help)
	maxEventCount  int64      // Exposed for testing purposes
}

func NewEventDB(logger logs.Log, dbFilename string) (*EventDB, error) {
	os.MkdirAll(filepath.Dir(dbFilename), 0770)
	configDB, err := dbh.OpenDB(logger, dbh.MakeSqliteConfig(dbFilename), Migrations(logger), dbh.DBConnectFlagSqliteWAL)
	if err != nil {
		return nil, fmt.Errorf("Failed to open database %v: %w", dbFilename, err)
	}
	if err := os.Chmod(dbFilename, 0600); err != nil {
		return nil, fmt.Errorf("Failed to change permissions on database %v: %w", dbFilename, err)
	}

	edb := &EventDB{
		Log: logger,
		DB:  configDB,
	}

	// Read the armed and alarmed state from the DB.
	// We're looking for the most recent event of type EventTypeArm or EventTypeDisarm.
	var armEvent Event
	if err := edb.DB.Order("time DESC").Where("event_type IN (?, ?)", EventTypeArm, EventTypeDisarm).First(&armEvent).Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("Failed to read last arm/disarm event: %w", err)
		}
		// No events found, so assume disarmed
		edb.armed = false
	} else {
		edb.armed = armEvent.EventType == EventTypeArm
	}
	// If we're armed, then see if there has been an alarm activation since the arming
	if edb.armed {
		var alarmEvent Event
		if err := edb.DB.Order("time DESC").Where("event_type = ?", EventTypeAlarm).Where("id > ?", armEvent.ID).First(&alarmEvent).Error; err != nil {
			if err != gorm.ErrRecordNotFound {
				return nil, fmt.Errorf("Failed to read last alarm event: %w", err)
			}
			// No alarm events found, so assume not triggered
			edb.alarmTriggered = false
		} else {
			edb.alarmTriggered = true
		}
	}

	edb.purgeOldRecords()

	return edb, nil
}

func (e *EventDB) purgeOldRecords() {
	count := int64(0)
	e.DB.Model(&Event{}).Count(&count)
	if count > e.maxEventCount {
		nDelete := int(min(100, count/10))
		e.DB.Exec(fmt.Sprintf("DELETE FROM event WHERE id IN (SELECT id FROM event ORDER BY id ASC LIMIT %v)", nDelete))
		e.Log.Infof("Purged %v old events from the database", nDelete)
	}
}
