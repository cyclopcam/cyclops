package videodb

// !!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
// NOTE: The code in this file is unfinished work, and may not be useful at all. I need more data before continuing.
// !!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!

/*
type SpatialFilterParams struct {
	MinMove int // If X,Y coordinates move by less than this amount, then discard the frame
}

// It would be nice if we could make this 'const'
var DefaultSpatialFilterParams SpatialFilterParams

// Filter out small movements of the object boxes, so that we can reduce
// the number of frames that we need to store.
func RemoveRedundantFrames(positions []ObjectPosition, params *SpatialFilterParams) []ObjectPosition {
	if params == nil {
		params = &DefaultSpatialFilterParams
	}
	keep := make([]int, len(positions))
	for i := 0; i < len(positions); i++ {
		keep[i] = i
	}
	//minMove := params.MinMove
	prevN := len(keep) + 1
	for prevN != len(keep) {
		prevN = len(keep)
		keep = keep[:0]
		for i := 1; i < len(positions)-1; i++ {
			// Look at frame prior and after this one, and determine if we need it
			// mmkay - this is rando speculative work. I need real data before I can continue here.
		}

	}
	return positions
}

func init() {
	DefaultSpatialFilterParams.MinMove = 5
}
*/
