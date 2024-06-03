package videodb

import (
	"errors"
	"fmt"
	"time"

	"github.com/cyclopcam/cyclops/pkg/dbh"
	"gorm.io/gorm"
)

// This function is called after any level-0 tiles are written.
// Our job is to walk up the tile levels, and see if we can write
// any higher-level tiles. We only write higher level tiles once
// all the lower level tiles have been closed out (i.e. their time
// limit has been reached, so we won't be writing to them anymore).
//
// One non-obvious caveat is that we tolerate missing tiles from
// lower levels. A missing tile is equivalent to a string of zero
// bits. It means that either nothing interesting happened during
// that period, or we were switched off during that period.
func (v *VideoDB) buildHigherTiles(cameraToTileIdx map[uint32]uint32, cutoff time.Time) {
	maxTileIdx := uint32(0)
	for camera, tileIdx := range cameraToTileIdx {
		if tileIdx > maxTileIdx {
			maxTileIdx = tileIdx
		}
		v.buildHigherTilesForCamera(camera, tileIdx, cutoff)
	}

	v.setKV("lastTileIdx", maxTileIdx)
}

func endOfTile(tileIdx, level uint32) time.Time {
	return tileIdxToTime(tileIdx+1, level)
}

func tileKey(level, tileIdx uint32) string {
	return fmt.Sprintf("%v:%v", level, tileIdx)
}

// Build higher level tiles for the tiles from startTileIdx to cutoffTime
// We stop walking up the levels once we hit a tile that extends beyond cutoffTime.
// To put it another way, we only build tiles who's end time is before cutoffTime.
func (v *VideoDB) buildHigherTilesForCamera(camera, startTileIdx uint32, cutoffTime time.Time) {
	// We keep a cache of tiles that we've just built, so that higher up level builds don't require us to
	// reload them from the DB. This is especially important for building higher tile levels in real-time.
	// The cache avoids round-trip to the DB, and also the encoding/decoding of the tile bitmaps.
	cachedTiles := map[string]*tileBuilder{}

	for level := uint32(1); level <= uint32(v.maxTileLevel); level++ {
		startTileIdx /= 2

		// Fetch the list of tiles that exist before iterating. If the system has been shutdown for a long time,
		// and then booted up again, this scan would take very long if we didn't do this initial check.
		cutoffTileIdxChild := timeToTileIdx(cutoffTime, level-1)
		validTileIndices, err := dbh.ScanArray[uint32](v.db.Raw("SELECT start FROM event_tile WHERE camera = ? AND level = ? AND start >= ? AND start <= ?",
			camera, level-1, startTileIdx*2, cutoffTileIdxChild+1).Rows())
		if err != nil {
			v.log.Errorf("Failed to scan child tiles for camera %v, level %v, startTileIdx %v", camera, level-1, startTileIdx)
		}
		if len(validTileIndices) == 0 {
			// Not thing to do here - child level is empty
			continue
		}
		// Build up a hash table of available tile indices.
		// Also, find the range of available tile indices, to limit our scan range.
		validTileIndicesSet := map[uint32]bool{}
		minTileIdxAvailable := uint32(0xffffffff)
		maxTileIdxAvailable := uint32(0)
		for _, idx := range validTileIndices {
			validTileIndicesSet[idx] = true
			if idx < minTileIdxAvailable {
				minTileIdxAvailable = idx
			}
			if idx > maxTileIdxAvailable {
				maxTileIdxAvailable = idx
			}
		}
		startTileIdx = max(startTileIdx, minTileIdxAvailable/2)
		endTileIdx := maxTileIdxAvailable/2 + 1

		for tileIdx := startTileIdx; endOfTile(tileIdx, level).Before(cutoffTime) && tileIdx < endTileIdx; tileIdx++ {
			childIdx0 := tileIdx * 2
			childIdx1 := tileIdx*2 + 1
			if !validTileIndicesSet[childIdx0] && !validTileIndicesSet[childIdx1] {
				// We don't write empty tiles to the DB
				continue
			}
			children := [2]*tileBuilder{
				cachedTiles[tileKey(level-1, childIdx0)],
				cachedTiles[tileKey(level-1, childIdx1)],
			}
			// Load left child
			if children[0] == nil && validTileIndicesSet[childIdx0] {
				children[0], _ = v.loadAndDecodeTile(camera, level-1, childIdx0)
			}
			// Load right child
			if children[1] == nil && validTileIndicesSet[childIdx1] {
				children[1], _ = v.loadAndDecodeTile(camera, level-1, childIdx1)
			}
			if v.debugTileLevelBuild {
				v.log.Infof("Merging tiles %v,%v,%v and %v,%v,%v into %v,%v", camera, level-1, tileIdx*2, camera, level-1, tileIdx*2+1, level, tileIdx)
			}
			mergedBuiler, err := mergeTileBuilders(tileIdx, level, children[0], children[1], v.maxClassesPerTile)
			if err != nil {
				v.log.Errorf("Failed to merge tile blobs: %v", err)
				continue
			}
			cachedTiles[tileKey(level, tileIdx)] = mergedBuiler
			v.upsertTile(camera, mergedBuiler)
		}
	}
}

func (v *VideoDB) upsertTile(camera uint32, tb *tileBuilder) error {
	err := v.db.Exec("INSERT INTO event_tile (camera, level, start, tile) VALUES (?, ?, ?, ?) ON CONFLICT(camera, level, start) DO UPDATE SET tile = excluded.tile",
		camera, tb.level, tb.tileIdx, tb.writeBlob()).Error
	if err != nil {
		v.log.Errorf("Failed to upsert tile %v,%v,%v: %v", camera, tb.level, tb.tileIdx, err)
	}
	return err
}

func (v *VideoDB) loadAndDecodeTile(camera, level, tileIdx uint32) (*tileBuilder, error) {
	tile := EventTile{}
	if err := v.db.First(&tile, "camera = ? AND level = ? AND start = ?", camera, level, tileIdx).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			v.log.Errorf("Failed to load tile %v,%v,%v: %v", camera, level, tileIdx, err)
		}
		return nil, err
	}
	return readBlobIntoTileBuilder(tile.Start, tile.Level, tile.Tile, v.maxClassesPerTile)
}

// This is run once at startup, in case we've been offline for a long time.
// The time 'now' is passed as a parameter for the sake of unit tests.
func (v *VideoDB) fillMissingTiles(now time.Time) {
	lastTileIdx := uint32(0)
	v.getKV("lastTileIdx", &lastTileIdx)
	if lastTileIdx == 0 {
		// empty/uninitialized database
		return
	}
	recentCameraIDs, err := v.findRecentCameras(lastTileIdx)
	if err != nil {
		v.log.Errorf("Failed to find recent cameras: %v", err)
		return
	}

	cutoff := now.Add(-tileWriterFlushThreshold)
	mostRecentlyClosedTileIdx := timeToTileIdx(cutoff, 0) - 1 // The -1 is because we're using the end-time of tiles

	for _, camera := range recentCameraIDs {
		v.buildHigherTilesForCamera(camera, lastTileIdx+1, cutoff)
	}

	v.setKV("lastTileIdx", mostRecentlyClosedTileIdx)
}

// Find recent camera IDs from level 0 tiles
func (v *VideoDB) findRecentCameras(afterTileIdx uint32) ([]uint32, error) {
	//tileIdx := timeToTileIdx(time.Now().Add(-lookBack), 0)
	return dbh.ScanArray[uint32](v.db.Raw("SELECT camera FROM event_tile WHERE level = 0 AND start >= ? GROUP BY camera", afterTileIdx).Rows())
}
