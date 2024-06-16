package videodb

import "fmt"

// Read up to date event tiles.
// We merge tiles from the database and tiles that are currently being written.
func (v *VideoDB) getEventTiles(camera, level, startTileIdx, endTileIdx uint32) ([]EventTile, error) {
	if endTileIdx-startTileIdx > 20 {
		// Sanity check.
		// If you're fetching this many tiles, then fetch from a higher level.
		// 20 tiles at level 13 is 5 years. That's far beyond our design limit.
		return nil, fmt.Errorf("Too many tiles requested")
	}

	// Fetch tiles from DB
	tiles := []EventTile{}
	if err := v.db.Where("camera = ? AND level = ? AND tileIdx >= ? AND tileIdx <= ?", camera, level, startTileIdx, endTileIdx).Find(&tiles).Error; err != nil {
		return nil, err
	}

	idxToTile := map[uint32]EventTile{}
	for _, tile := range tiles {
		idxToTile[tile.Start] = tile
	}

	//now := time.Now()

	// If any of the tiles that are being fetched overlap the current time,
	// then we need to build up these tiles right now, from the latest tiles
	// that we're busy recording.
	for tileIdx := startTileIdx; tileIdx <= endTileIdx; tileIdx++ {
	}
	
	return nil, nil
}
