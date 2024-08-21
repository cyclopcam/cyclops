package videodb

import (
	"time"

	"github.com/cyclopcam/cyclops/pkg/gen"
)

// See places where we use this constant for an explanation of what it means
const tileWriterMaxBacktrack = 10 * time.Second

// We flush tiles that are this old. We add buffer beyond tileWriterMaxBacktrack
// to ensure we don't have any gaps.
const tileWriterFlushThreshold = 2 * tileWriterMaxBacktrack

func (v *VideoDB) tileWriteThread() {
	v.log.Infof("Event tile write thread starting")
	keepRunning := true
	wakeInterval := 61 * time.Second
	wakeCounter := 0
	for keepRunning {
		select {
		case <-v.shutdown:
			keepRunning = false
		case <-time.After(wakeInterval):
			if v.debugTileWriter && wakeCounter%1 == 0 {
				v.debugDumpTilesToConsole()
			}
			//cutoff := time.Now().Add(-tileWriterFlushThreshold)
			//v.flushOldTiles(cutoff)
			v.writeAllCurrentTiles()
			wakeCounter++
		}
	}
	v.log.Infof("Flushing all tiles")
	//v.flushOldTiles(time.Now().Add(100000 * time.Hour))
	v.writeAllCurrentTiles()
	v.log.Infof("Event tile write thread exiting")
	close(v.tileWriteThreadClosed)
}

// map[cameraID]*tileBuilder -> map[cameraID]tileIdx
func makeCameraTileIdxMap(in map[uint32]*tileBuilder) map[uint32]uint32 {
	out := map[uint32]uint32{}
	for camera, tb := range in {
		out[camera] = tb.tileIdx
	}
	return out
}

func maxTileIdxAtLevel0(oldTiles map[uint32][]*tileBuilder) uint32 {
	maxIdx := uint32(0)
	for _, tiles := range oldTiles {
		for _, tb := range tiles {
			if tb.level == 0 && tb.tileIdx > maxIdx {
				maxIdx = tb.tileIdx
			}
		}
	}
	return maxIdx
}

// Write all in-memory tiles to the database.
// This is called periodically (eg once a minute).
func (v *VideoDB) writeAllCurrentTiles() {
	//--------------------------------------------
	v.currentTilesLock.Lock()
	// Make copies of all tiles, to minimize our time inside the lock.
	// Each tile is a 128 byte bitmap for each class that we track.
	// Let's imagine 16 cameras, with 5 classes, and we're tracking 13 levels.
	// 16 * 5 * 13 * 128 = 133120 bytes. This is a trivial amount of memory
	// to copy once a minute.
	tileCopies := map[uint32][]*tileBuilder{}
	cloneToOriginal := map[*tileBuilder]*tileBuilder{}
	for cameraID, levels := range v.currentTiles {
		for _, level := range levels {
			for _, tile := range level {
				if tile.updateTick != tile.updateTickAtLastWrite {
					//v.log.Debugf("%v %v %v : %v -> %v", cameraID, tile.level, tile.tileIdx, tile.updateTickAtLastWrite, tile.updateTick)
					clone := tile.clone()
					tileCopies[cameraID] = append(tileCopies[cameraID], clone)
					cloneToOriginal[clone] = tile
				}
			}
		}
	}
	v.currentTilesLock.Unlock()
	//--------------------------------------------

	tx := v.db.Begin()
	if tx.Error != nil {
		v.log.Errorf("writeAllCurrentTiles failed to start transaction: %v", tx.Error)
		return
	}
	defer tx.Rollback()

	for camera, tiles := range tileCopies {
		for _, tile := range tiles {
			//v.log.Infof("Writing tile for camera %v, level %v, tileIdx %v", camera, tile.level, tile.tileIdx)
			v.upsertTile(tx, camera, tile)
		}
	}

	// Set lastTileIdx, even though we can get rid of the concept of back-filling tiles now that
	// we write all levels at a regular interval. This is 100% legacy now, since we've commented out
	// the call to fillMissingTiles() at startup.
	cutoff := time.Now().Add(-tileWriterFlushThreshold)
	sealedTileIdx := timeToTileIdx(cutoff, 0)
	v.setKV("lastTileIdx", sealedTileIdx, tx)

	if err := tx.Commit().Error; err != nil {
		v.log.Errorf("writeAllCurrentTiles failed to commit transaction: %v", err)
	}

	// Update the updateTickAtLastWrite for all tiles that were written.
	v.currentTilesLock.Lock()
	for _, tiles := range tileCopies {
		for _, clone := range tiles {
			cloneToOriginal[clone].updateTickAtLastWrite = clone.updateTick
		}
	}
	v.currentTilesLock.Unlock()

	// Remove tiles that are clearly in the past
	v.findAndRemoveOldTiles(cutoff)
}

/*
// Returns the list of tiles that were written (even if the write failed)
// Writes all tiles who's end time is before cutoff
func (v *VideoDB) flushOldTiles(cutoff time.Time) map[uint32][]*tileBuilder {
	oldTiles := v.findAndRemoveOldTiles(cutoff)

	if len(oldTiles) == 0 {
		return oldTiles
	}

	tx := v.db.Begin()
	if tx.Error != nil {
		v.log.Errorf("flushOldTiles failed to start transaction: %v", tx.Error)
		return oldTiles
	}
	defer tx.Rollback()

	for camera, tiles := range oldTiles {
		for _, tile := range tiles {
			v.log.Infof("Writing tile for camera %v, level %v, tileIdx %v", camera, tile.level, tile.tileIdx)
			v.upsertTile(tx, camera, tile)
		}
	}

	if len(oldTiles) != 0 {
		maxIdx := maxTileIdxAtLevel0(oldTiles)
		if maxIdx == 0 {
			v.log.Errorf("maxTileIdxAtLevel0 returned 0. How can high level tiles flush but not level 0?")
		} else {
			v.log.Infof("Setting lastTileIdx to %v", maxIdx)
			v.setKV("lastTileIdx", maxIdx, tx)
		}
	}

	if err := tx.Commit().Error; err != nil {
		v.log.Errorf("flushOldTiles failed to commit transaction: %v", err)
	}

	return oldTiles
}
*/

// Finds all tiles who's end time is before cutoff, and removes them from currentTiles.
// Returns those old tiles as a map from camera to tilebuilder.
func (v *VideoDB) findAndRemoveOldTiles(cutoff time.Time) map[uint32][]*tileBuilder {
	v.currentTilesLock.Lock()
	defer v.currentTilesLock.Unlock()

	// First do a quick scan to see if there are any old level-0 tiles.
	// If there are no old level-0 tiles, then we can be 100% sure that there aren't any
	// old higher-level tiles. Level 0 tiles occur most frequently, so you can't have
	// a higher level tile ending without a level 0 tile ending.
	// See readme.md of VideoDB to visualize this.
	// Since this function runs every 13 seconds, we don't want to rebuild the map every time
	// if there's nothing to do.
	hasOldTiles := false
	for _, levels := range v.currentTiles {
		for _, tb := range levels[0] {
			if endOfTile(tb.tileIdx, tb.level).Before(cutoff) {
				hasOldTiles = true
				break
			}
		}
		if hasOldTiles {
			break
		}
	}
	if !hasOldTiles {
		// This is the most common code path
		return nil
	}

	// Map from CameraID to list of tiles that were written for that camera
	oldTiles := map[uint32][]*tileBuilder{}

	// Build up a completely new 'currentTiles' map, excluding any tiles that are old.
	// At the end of this function, we throw newCurrentTiles away, if removeOldTiles is false.
	// The performance hit of building up the new tiles list is negligible.
	newCurrentTiles := map[uint32][][]*tileBuilder{}

	for camera, levelsForCamera := range v.currentTiles {
		nCameraTiles := 0
		newLevels := make([][]*tileBuilder, v.maxTileLevel+1)
		for level, tiles := range levelsForCamera {
			newTiles := []*tileBuilder{}
			for _, tb := range tiles {
				if endOfTile(tb.tileIdx, tb.level).Before(cutoff) {
					// This tile is old
					oldTiles[camera] = append(oldTiles[camera], tb)
				} else {
					newTiles = append(newTiles, tb)
					nCameraTiles++
				}
			}
			newLevels[level] = newTiles
		}
		if nCameraTiles != 0 {
			newCurrentTiles[camera] = newLevels
		}
	}

	v.currentTiles = newCurrentTiles

	return oldTiles
}

// Tiles are 1024 seconds long, so if our system restarts, then we need to resume
// the production of the latest tile.
// This function is called when v.currentTiles is empty.
func (v *VideoDB) resumeLatestTiles() {
	// We take the lock to satisfy the race detector, but this function runs before
	// any of our background threads are started
	v.currentTilesLock.Lock()
	defer v.currentTilesLock.Unlock()

	// Make sure this is still true (i.e. no code drift)
	if len(v.currentTiles) != 0 {
		v.log.Errorf("resumeLatestTiles called with non-empty currentTiles")
	}

	now := time.Now()
	nTilesTotal := 0
	for level := uint32(0); level <= uint32(v.maxTileLevel); level++ {
		nTilesAtLevel := 0
		currentTileIdx := timeToTileIdx(now, level)
		tiles := []*EventTile{}
		if err := v.db.Where("level = ? AND start = ?", level, currentTileIdx).Find(&tiles).Error; err != nil {
			v.log.Errorf("Failed to find latest tiles: %v", err)
			return
		}
		for _, tile := range tiles {
			tb, err := readBlobIntoTileBuilder(tile.Start, tile.Level, tile.Tile, v.maxClassesPerTile, 0)
			if err != nil {
				v.log.Errorf("Failed to read tile blob camera:%v level:%v start:%v for resume: %v", tile.Camera, tile.Level, tile.Start, err)
				continue
			}
			levelsForCamera := v.currentTilesForCamera(tile.Camera)
			levelsForCamera[level] = append(levelsForCamera[tile.Level], tb)
			nTilesAtLevel++
			nTilesTotal++
		}
		if nTilesAtLevel != 0 {
			v.log.Infof("Resumed %v tiles at level %v", nTilesAtLevel, level)
		}
	}
	v.log.Infof("Resumed %v tiles in total", nTilesTotal)
}

// Ensures that the current tiles slice for this camera has [maxTileLevels+1] entries,
// and returns it.
// WARNING! You must be holding currentTilesLock when calling this function
func (v *VideoDB) currentTilesForCamera(camera uint32) [][]*tileBuilder {
	levels := v.currentTiles[camera]
	added := false
	for len(levels) < v.maxTileLevel+1 {
		levels = append(levels, []*tileBuilder{})
		added = true
	}
	if added {
		v.currentTiles[camera] = levels
	}
	return levels
}

// Update tiles at all levels with a new detection.
func (v *VideoDB) updateTilesWithNewDetection(obj *TrackedObject) {
	// Find the current tile(s) for the camera, which span the time frame of the tracked object.
	// If these tiles don't exist, then create them.
	firstSeen, lastSeen := obj.TimeBounds()
	//v.log.Infof("firstSeen: %v, lastSeen: %v", firstSeen, lastSeen)

	// Clamp the back end of the time range if necessary, so that we're not trying to update
	// something far in the past. The firstSeen time on a tracked object could be
	// hours ago, but we're only interested here in updating real-time information.
	// The historical tiles have already been dealt with. We're only going to add
	// data to at most two tiles.
	// The -10 second limit is chosen so that it is much less than TileWidth (1024 seconds),
	// and also large enough that even if we're under load, we won't miss an event.
	maxBacktrack := time.Now().Add(-tileWriterMaxBacktrack)
	if firstSeen.Before(maxBacktrack) {
		firstSeen = maxBacktrack
	}
	if lastSeen.Before(maxBacktrack) {
		lastSeen = maxBacktrack
	}

	v.currentTilesLock.Lock()
	defer v.currentTilesLock.Unlock()

	for level := uint32(0); level <= uint32(v.maxTileLevel); level++ {
		tileIdx1 := timeToTileIdx(firstSeen, level)
		tileIdx2 := timeToTileIdx(lastSeen, level)
		// tileIdx1 and tileIdx2 are likely equal. At most, tileIdx2 - tileIdx1 = 1,
		// whenever we're transitioning from one tile to the next. The higher up we go in levels,
		// the more likely it is that tileIdx1 == tileIdx2
		v.updateTileWithNewDetection(level, tileIdx1, obj)
		if tileIdx2 != tileIdx1 {
			v.updateTileWithNewDetection(level, tileIdx2, obj)
		}
	}
}

func (v *VideoDB) updateTileWithNewDetection(level, tileIdx uint32, obj *TrackedObject) {
	levelsForCamera := v.currentTilesForCamera(obj.Camera)
	var builder *tileBuilder
	for _, tb := range levelsForCamera[level] {
		if tb.tileIdx == tileIdx {
			builder = tb
			break
		}
	}
	if builder == nil {
		builder = newTileBuilder(level, tileIdxToTime(tileIdx, level), v.maxClassesPerTile)
		levelsForCamera[level] = append(levelsForCamera[level], builder)
	}
	if err := builder.updateObject(obj); err != nil {
		v.log.Warnf("Failed to update event tile: %v", err)
	}
}

// This is a debug function
func (v *VideoDB) debugDumpTilesToConsole() {
	v.currentTilesLock.Lock()
	defer v.currentTilesLock.Unlock()

	seconds := time.Now().Unix()
	currentTileIdx := seconds / TileWidth
	maxLevel := 3 // Arbitrary clipping to avoid filling the screen

	v.log.Infof("Dumping current tiles (tileIdx %v, seconds to next: %v):", currentTileIdx, TileWidth-seconds%TileWidth)

	for camera, levelsForCamera := range v.currentTiles {
		for level := 0; level <= maxLevel; level++ {
			for _, tb := range levelsForCamera[level] {
				// compute the current time's position inside the tile, so that we can show a relevant window.
				// 1024 is too much to fit onto a console. If we had pixel-level control of the console, then this would
				// be different.
				factor := float64(uint32(1) << level)
				delta := int(time.Now().Sub(tb.baseTime).Seconds() / factor)
				startPx := gen.Clamp(delta-50, 0, TileWidth)
				endPx := gen.Clamp(delta+50, 0, TileWidth)
				if startPx == endPx {
					v.log.Infof("camera %v, level %v, tile %v: %v classes -- out of time range", camera, tb.level, tb.tileIdx, len(tb.classes))
				} else {
					v.log.Infof("camera %v, level %v, tile %v: %v classes, %v objects (%v - %v)", camera, tb.level, tb.tileIdx, len(tb.classes), len(tb.objects), startPx, endPx)
					for clsId, cls := range tb.classes {
						v.log.Infof("  class %3d: %v", clsId, cls.formatRange(startPx, endPx))
					}
				}
			}
		}
	}
}
