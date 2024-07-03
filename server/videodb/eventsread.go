package videodb

import (
	"fmt"

	"github.com/cyclopcam/cyclops/pkg/dbh"
)

// Fetch event tiles in the range [startIdx, endIdx)
func (v *VideoDB) ReadEventTiles(camera string, level, startIdx, endIdx uint32) ([]*EventTile, error) {
	if level > uint32(v.maxTileLevel) {
		return nil, fmt.Errorf("Level %d is too high. Max level is %v", level, v.maxTileLevel)
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
	if level >= uint32(len(levels)) {
		// We're not busy building tiles at this level for this camera, so we're done here
		v.currentTilesLock.Unlock()
		return tiles, nil
	}

	haveIdx := []uint32{0} // Fill it with a single invalid "0" tile index so that we don't get an empty SQL set (i.e. "()"), which is illegal SQL

	for _, tile := range levels[level] {
		if tile.tileIdx >= startIdx && tile.tileIdx < endIdx {
			// Encode in-memory tile
			tiles = append(tiles, &EventTile{
				Camera: cameraID,
				Level:  level,
				Start:  tile.tileIdx,
				Tile:   tile.writeBlob(), // Might want to disable compression here - not sure of the cost
			})
			haveIdx = append(haveIdx, tile.tileIdx)
		}
	}
	v.currentTilesLock.Unlock()
	// ------------------------

	initialLen := len(tiles)
	if err := v.db.Where("camera = ? AND level = ? AND start >= ? AND start < ? AND start NOT IN "+dbh.SQLFormatIDArray(haveIdx),
		camera, level, startIdx, endIdx).Find(&tiles).Error; err != nil {
		return nil, err
	}
	if len(tiles) < initialLen {
		panic("gorm is zeroing out the tiles slice")
	}

	return tiles, nil
}
