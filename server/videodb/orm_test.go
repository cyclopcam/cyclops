package videodb

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// Not really a test, just some debug code to figure out sizes of our JSON event storage
func TestJSONSize(t *testing.T) {
	nFrames := 1000
	j := ObjectJSON{}
	j.Class = "monkey"
	j.ID = 1
	for i := 0; i < nFrames; i++ {
		j.Positions = append(j.Positions, ObjectPositionJSON{
			Box:  [4]int16{100, 200, 300, 400},
			Time: int32(i * 1000),
		})
	}
	rb, err := json.Marshal(&j)
	require.NoError(t, err)
	t.Logf("Size of %d frames: %d bytes. Bytes per frame: %v", nFrames, len(rb), len(rb)/nFrames)
}
