package kibi

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKibi(t *testing.T) {
	require.Equal(t, "0 bytes", Bytes(0))
	require.Equal(t, "1 bytes", Bytes(1))
	require.Equal(t, "1023 bytes", Bytes(1023))
	require.Equal(t, "1 KB", Bytes(1024))
	require.Equal(t, "1 MB", Bytes(1024*1024))
	require.Equal(t, "35 MB", Bytes(35*1024*1024))
	require.Equal(t, "1023 MB", Bytes(1023*1024*1024))
	require.Equal(t, "1 GB", Bytes(1024*1024*1024))
	require.Equal(t, "1 TB", Bytes(1024*1024*1024*1024))
	require.Equal(t, "1 PB", Bytes(1024*1024*1024*1024*1024))

	goodParse := func(expected int64, s string) {
		val, err := Parse(s)
		require.NoError(t, err)
		require.Equal(t, expected, val)
	}

	goodParse(int64(0), "0")
	goodParse(int64(50), "50 bytes")
	goodParse(int64(50), "50")
	goodParse(int64(50*1024), "50 kb")
	goodParse(int64(50*1024), "50 KB")
	goodParse(int64(50*1024), "50 K")
	goodParse(int64(50*1024*1024), "50 mb")
	goodParse(int64(50*1024*1024*1024), "50 gb")
	goodParse(int64(50*1024*1024*1024*1024), "50 tb")
	goodParse(int64(50*1024*1024*1024*1024*1024), "50 pb")

	badParse := func(s string) {
		_, err := Parse(s)
		require.Error(t, err)
	}

	badParse("")
	badParse("50 pbz")
	badParse("50.1")
}
