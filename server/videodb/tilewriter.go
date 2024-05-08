package videodb

import "time"

// #include "tile.h"
import "C"

func (v *VideoDB) tileWriteThread() {
	v.log.Infof("Event tile write thread starting")
	keepRunning := true
	wakeInterval := 47 * time.Second
	for keepRunning {
		select {
		case <-v.shutdown:
			keepRunning = false
		case <-time.After(wakeInterval):
			if err := v.writeTiles(); err != nil {
				v.log.Warnf("Failed to write event summaries: %v", err)
			}
		}
	}
	v.log.Infof("Event tile write thread exiting")
	close(v.summaryWriteThreadClosed)
}

func (v *VideoDB) writeTiles() error {
	// Our job here is to find any events from the 'event' table that have not been captured in the
	// event_summary table, and write them there.

	lastEvents := []Event{}
	if err := v.db.Raw("SELECT camera, max(time) FROM event GROUP BY camera").Scan(&lastEvents).Error; err != nil {
		return err
	}

	//lastSummaries := []EventSummary{}
	//if err := v.db.Raw("SELECT camera, max(time) FROM event_summary GROUP BY camera").Scan(&lastSummaries).Error; err != nil {
	//	return err
	//}

	return nil
}

func (v *VideoDB) updateTileWithNewDetection(obj *TrackedObject) {
	// Find the current tile for the camera, or create a new tile if one doesn't already exist
}
