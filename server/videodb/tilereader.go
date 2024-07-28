package videodb

// Decode a tile enough to be able to find the list of class IDs
// inside it, and return that list of IDs.
func GetClassIDsInTileBlob(tile []byte) ([]uint32, error) {
	tb, err := readBlobIntoTileBuilder(0, 0, tile, 1000, readBlobFlagSkipBitmaps)
	if err != nil {
		return nil, err
	}
	ids := []uint32{}
	for id := range tb.classes {
		ids = append(ids, id)
	}
	return ids, nil
}
