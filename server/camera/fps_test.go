package camera

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEstimateFPS(t *testing.T) {
	intervals := []time.Duration{
		66 * time.Millisecond,
		67 * time.Millisecond,
		66 * time.Millisecond,
	}
	fps := EstimateFPS(intervals)
	require.Equal(t, 15.0, fps)

	intervals = []time.Duration{
		100 * time.Millisecond,
		101 * time.Millisecond,
		99 * time.Millisecond,
		101 * time.Millisecond,
	}
	fps = EstimateFPS(intervals)
	require.Equal(t, 10.0, fps)

	intervals = []time.Duration{
		1000 * time.Millisecond,
		1001 * time.Millisecond,
		999 * time.Millisecond,
	}
	fps = EstimateFPS(intervals)
	require.Equal(t, 1.0, fps)

	intervals = []time.Duration{
		2000 * time.Millisecond,
		2001 * time.Millisecond,
		1999 * time.Millisecond,
	}
	fps = EstimateFPS(intervals)
	require.Equal(t, 0.5, fps)

	intervals = []time.Duration{
		4005 * time.Millisecond,
		4008 * time.Millisecond,
		3950 * time.Millisecond,
	}
	fps = EstimateFPS(intervals)
	require.Equal(t, 0.25, fps)
}
