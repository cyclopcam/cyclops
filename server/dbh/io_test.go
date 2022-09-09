package dbh

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIDList(t *testing.T) {
	require.Equal(t, "1", idListToString(StringToIDList("1")))
	require.Equal(t, "", idListToString(StringToIDList("")))
	require.Equal(t, "1,2", idListToString(StringToIDList("1,2")))
	require.Equal(t, "0,0", idListToString(StringToIDList(",")))
	require.Equal(t, "0", idListToString(StringToIDList("_")))

	require.Equal(t, "()", IDListToSQLSet([]int64{}))
	require.Equal(t, "(0)", IDListToSQLSet([]int64{0}))
	require.Equal(t, "(5,6)", IDListToSQLSet([]int64{5, 6}))
}
