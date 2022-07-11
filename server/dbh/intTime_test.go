package dbh

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestIntTime(t *testing.T) {
	t1 := IntTime(0)
	a := time.Date(2022, time.February, 3, 4, 5, 6, 777*1000*1000, time.UTC)
	t1.Set(a)
	b := t1.Get()

	require.Equal(t, a, b)
}
