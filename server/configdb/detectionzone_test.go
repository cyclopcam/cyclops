package configdb

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDetectionZone(t *testing.T) {
	dz := NewDetectionZone(8, 6)
	dz.Active[1] = 0xe1
	dz.Active[3] = 0x0a

	encoded := dz.EncodeBase64()
	dz2, err := DecodeDetectionZoneBase64(encoded)
	require.NoError(t, err)
	require.Equal(t, dz.Width, dz2.Width)
	require.Equal(t, dz.Height, dz2.Height)
	require.Equal(t, dz.Active, dz2.Active)
}
