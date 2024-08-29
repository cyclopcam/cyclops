package videodb

import (
	"errors"
	"fmt"
	"time"

	"github.com/cyclopcam/cyclops/pkg/dbh"
	"github.com/cyclopcam/cyclops/server/defs"
)

var ErrNoVideoFound = errors.New("No video found")

// TileRequest is a request to read tiles.
// Do ONE of the following:
// 1. Populate StartIdx and EndIdx
// 2. Populate Indices
type TileRequest struct {
	Level    uint32
	StartIdx uint32 // inclusive
	EndIdx   uint32 // exclusive
	Indices  map[uint32]bool
}

// Fetch event tiles in the range [startIdx, endIdx)
func (v *VideoDB) ReadEventTiles(camera string, request TileRequest) ([]*EventTile, error) {
	if request.Level > uint32(v.maxTileLevel) {
		return nil, fmt.Errorf("Level %d is too high. Max level is %v", request.Level, v.maxTileLevel)
	}
	if len(request.Indices) > 0 && request.EndIdx-request.StartIdx != 0 {
		return nil, fmt.Errorf("Specify either Indices or StartIdx/EndIdx, not both")
	}

	cameraID, err := v.StringToID(camera)
	if err != nil {
		return nil, err
	}

	tiles := []*EventTile{}

	// First get latest state, so we avoid reading these from DB
	// ----------------------
	v.currentTilesLock.Lock()
	levels := v.currentTiles[cameraID]
	if request.Level >= uint32(len(levels)) {
		// We don't build tiles at this level for this camera, so we're done here
		v.currentTilesLock.Unlock()
		return tiles, nil
	}

	haveIdx := []uint32{0} // Fill it with a single invalid "0" tile index so that we don't get an empty SQL set (i.e. "()"), which is illegal SQL
	haveIdxSet := map[uint32]bool{}

	for _, tile := range levels[request.Level] {
		if (tile.tileIdx >= request.StartIdx && tile.tileIdx < request.EndIdx) || request.Indices[tile.tileIdx] {
			// Encode in-memory tile
			tiles = append(tiles, &EventTile{
				Camera: cameraID,
				Level:  request.Level,
				Start:  tile.tileIdx,
				Tile:   tile.writeBlob(), // Might want to disable compression here - not sure how to quantify the CPU cost/network bandwidth tradeoff
			})
			haveIdx = append(haveIdx, tile.tileIdx)
			haveIdxSet[tile.tileIdx] = true
		}
	}
	v.currentTilesLock.Unlock()
	// ------------------------

	dbTiles := []*EventTile{}

	if len(request.Indices) != 0 {
		// Produce a list of tile indices to fetch, but exclude the tiles that we've already fetched from memory
		trimmedTiles := []uint32{}
		for idx := range request.Indices {
			if !haveIdxSet[idx] {
				trimmedTiles = append(trimmedTiles, idx)
			}
		}
		if len(trimmedTiles) != 0 {
			if err := v.db.Where("camera = ? AND level = ? AND start IN "+dbh.SQLFormatIDArray(trimmedTiles),
				cameraID, request.Level).Find(&dbTiles).Error; err != nil {
				return nil, err
			}
		}
	} else {
		if err := v.db.Where("camera = ? AND level = ? AND start >= ? AND start < ? AND start NOT IN "+dbh.SQLFormatIDArray(haveIdx),
			cameraID, request.Level, request.StartIdx, request.EndIdx).Find(&dbTiles).Error; err != nil {
			return nil, err
		}
	}

	// GORM will trim the incoming slice, so we can't just pass 'tiles' into GORM. We must pass a fresh slice to GORM, and then merge results.
	tiles = append(tiles, dbTiles...)

	return tiles, nil
}

// Find the timestamp of the oldest recorded frame for the given camera.
// Returns *ErrNoVideoFound* if no video footage can be found for the camera.
func (v *VideoDB) VideoStartTimeForCamera(camera string) (time.Time, error) {
	oldestTime := time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, res := range defs.AllResolutions {
		stream := v.Archive.StreamInfo(VideoStreamNameForCamera(camera, res))
		if stream != nil && stream.StartTime.Before(oldestTime) {
			oldestTime = stream.StartTime
		}
	}
	if oldestTime.Year() != 9999 {
		return oldestTime, nil
	}
	return time.Time{}, ErrNoVideoFound
}

func (v *VideoDB) ReadEvents(camera string, startTime, endTime time.Time) ([]*Event, error) {
	cameraID, err := v.StringToID(camera)
	if err != nil {
		return nil, err
	}

	events := []*Event{}
	if err := v.db.Where("camera = ? AND time >= ? AND time < ?", cameraID, startTime, endTime).Find(&events).Error; err != nil {
		return nil, err
	}
	return events, nil
}
