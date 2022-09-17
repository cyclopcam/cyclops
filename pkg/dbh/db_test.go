package dbh

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDBNotExist(t *testing.T) {
	require.False(t, DBNotExistRegex.MatchString(`does not exist`))
	require.True(t, DBNotExistRegex.MatchString(`database "foobar" does not exist`))
	require.False(t, DBNotExistRegex.MatchString(`table "foobar" does not exist`))
	require.False(t, DBNotExistRegex.MatchString(`"foobar" does not exist`))
}
