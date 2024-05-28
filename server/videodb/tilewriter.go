package videodb

import (
	"time"

	"github.com/cyclopcam/cyclops/pkg/gen"
)

// See places where we use this constant for an explanation of what it means
const tileWriterMaxBacktrack = 10 * time.Second

const debugTileWriter = false

func (v *VideoDB) tileWriteThread() {
	v.log.Infof("Event tile write thread starting")
	keepRunning := true
	wakeInterval := 13 * time.Second
	for keepRunning {
		select {
		case <-v.shutdown:
			keepRunning = false
		case <-time.After(wakeInterval):
			if debugTileWriter {
				v.debugDumpTilesToConsole()
			}
			v.flushOldTiles(false)
		}
	}
	v.log.Infof("Flushing all tiles")
	v.flushOldTiles(true)
	v.log.Infof("Event tile write thread exiting")
	close(v.tileWriteThreadClosed)
}

func (v *VideoDB) flushOldTiles(forceAll bool) {
	oldTiles := v.findAndRemoveOldTiles(forceAll)
	for camera, tile := range oldTiles {
		if err := v.writeTile(camera, tile); err != nil {
			v.log.Errorf("Failed to write tile: %v", err)
		}
	}
}

// Returns a map from camera to tilebuilder
// If forceAll is true, then we flush all tiles (this is used at shutdown).
func (v *VideoDB) findAndRemoveOldTiles(forceAll bool) map[uint32]*tileBuilder {
	v.currentTilesLock.Lock()
	defer v.currentTilesLock.Unlock()

	now := time.Now()

	// We assume that we're never going to get more than 1 builder per camera to write.
	// If we do, then we'll write a random one (whichever appears last), and then
	// on the next call to writeOldTiles, we'll write the other one.
	// Our write interval (13 seconds) is much faster than the duration of a tile (1024 seconds).
	writeQueue := map[uint32]*tileBuilder{}

	// Build up a completely new 'currentTiles' map, excluding any tiles that are old.
	newCurrentTiles := map[uint32][]*tileBuilder{}
	for camera, tiles := range v.currentTiles {
		newTiles := []*tileBuilder{}
		for _, tb := range tiles {
			endOfTile := tb.baseTime.Add(TileWidth * time.Second)
			if forceAll || now.Sub(endOfTile) > tileWriterMaxBacktrack*2 {
				// This tile is old, so write it to disk.
				writeQueue[camera] = tb
			} else {
				newTiles = append(newTiles, tb)
			}
		}
		newCurrentTiles[camera] = newTiles
	}

	v.currentTiles = newCurrentTiles

	return writeQueue
}

func (v *VideoDB) writeTile(camera uint32, tb *tileBuilder) error {
	v.log.Infof("Writing base-level tile for camera %v, tileIdx %v, resume %v", camera, tb.tileIdx, tb.isResume)
	// The blob contains the list of classes (one per 1024-bit line),
	// as well as the 1024-bit lines, compressed by RLE.
	blob := tb.writeBlob()
	et := EventTile{
		Level:  tb.level,
		Camera: camera,
		Start:  tb.tileIdx,
		Tile:   blob,
	}
	if tb.isResume {
		// I can't get GORM to perform an UPDATE here - it seems to think that the object doesn't have a primary key.
		// Perhaps it's because our 'level' field is zero. I've tried everything I could think of on the GORM metadata.
		//if err := v.db.Save(&et).Error; err != nil {
		//	return err
		//}
		if err := v.db.Exec("UPDATE event_tile SET tile = ? WHERE level = ? AND camera = ? AND start = ?", blob, tb.level, camera, tb.tileIdx).Error; err != nil {
			return err
		}
	} else {
		if err := v.db.Create(&et).Error; err != nil {
			return err
		}
	}
	return nil
}

// Tiles are 1024 seconds long, so if our system restarts, then we need to resume
// the production of the latest tile.
func (v *VideoDB) resumeLatestTiles() {
	// We take the lock to satisfy the race detector, but this function runs before
	// any of our background threads are started
	v.currentTilesLock.Lock()
	defer v.currentTilesLock.Unlock()

	currentTileIdx := timeToTileIdx(time.Now(), 0)
	tiles := []*EventTile{}
	if err := v.db.Where("level = 0 AND start = ?", currentTileIdx).Find(&tiles).Error; err != nil {
		v.log.Errorf("Failed to find latest tiles: %v", err)
		return
	}
	nTiles := 0
	for _, tile := range tiles {
		tb, err := readBlobIntoTileBuilder(tile.Start, 0, tile.Tile, v.maxClassesPerTile)
		if err != nil {
			v.log.Errorf("Failed to read tile blob camera:%v start:%v for resume: %v", tile.Camera, tile.Start, err)
			continue
		}
		v.currentTiles[tile.Camera] = append(v.currentTiles[tile.Camera], tb)
		nTiles++
	}
	v.log.Infof("Resumed %v tiles", nTiles)
}

// Update one or two tiles with a new detection.
func (v *VideoDB) updateTilesWithNewDetection(obj *TrackedObject) {
	// Find the current tile(s) for the camera, which span the time frame of the tracked object.
	// If these tiles don't exist, then create them.
	firstSeen, lastSeen := obj.TimeBounds()

	// Move the time ranges forward if necessary, so that we're not trying to update
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
	tileIdx1 := timeToTileIdx(firstSeen, 0)
	tileIdx2 := timeToTileIdx(lastSeen, 0)
	// tileIdx1 and tileIdx2 are likely equal. At most, tileIdx2 - tileIdx1 = 1,
	// whenever we're transitioning from one tile to the next.
	v.updateTileWithNewDetection(tileIdx1, obj)
	if tileIdx2 != tileIdx1 {
		v.updateTileWithNewDetection(tileIdx2, obj)
	}
}

func (v *VideoDB) updateTileWithNewDetection(tileIdx uint32, obj *TrackedObject) {
	v.currentTilesLock.Lock()
	defer v.currentTilesLock.Unlock()

	tiles := v.currentTiles[obj.Camera]
	if tiles == nil {
		tiles = []*tileBuilder{}
	}
	var builder *tileBuilder
	for _, tb := range tiles {
		if tb.tileIdx == tileIdx {
			builder = tb
			break
		}
	}
	if builder == nil {
		builder = newTileBuilder(0, tileIdxToTime(tileIdx, 0), v.maxClassesPerTile)
		tiles = append(tiles, builder)
	}
	if err := builder.updateObject(obj); err != nil {
		v.log.Warnf("Failed to update event tile: %v", err)
	}
	v.currentTiles[obj.Camera] = tiles
}

// This is a debug function
func (v *VideoDB) debugDumpTilesToConsole() {
	v.currentTilesLock.Lock()
	defer v.currentTilesLock.Unlock()

	seconds := time.Now().Unix()
	currentTileIdx := seconds / TileWidth

	v.log.Infof("Dumping current tiles (tileIdx %v, seconds to next: %v):", currentTileIdx, TileWidth-seconds%TileWidth)

	for camera, tiles := range v.currentTiles {
		for _, tb := range tiles {
			// compute the current time's position inside the tile, so that we can show a relevant window.
			// 1024 is too much to fit onto a console. If we had pixel-level control of the console, then this would
			// be different.
			delta := int(time.Now().Sub(tb.baseTime).Seconds())
			startPx := gen.Clamp(delta-50, 0, TileWidth)
			endPx := gen.Clamp(delta+50, 0, TileWidth)
			if startPx == endPx {
				v.log.Infof("camera %v, tile %v: %v classes -- out of time range", camera, tb.tileIdx, len(tb.classes), len(tb.objects))
			} else {
				v.log.Infof("camera %v, tile %v: %v classes, %v objects (%v - %v)", camera, tb.tileIdx, len(tb.classes), len(tb.objects), startPx, endPx)
				for clsId, cls := range tb.classes {
					v.log.Infof("  class %3d: %v", clsId, cls.formatRange(startPx, endPx))
				}
			}
		}
	}
}
