package kibi

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKibi(t *testing.T) {
	require.Equal(t, "0 bytes", FormatBytes(0))
	require.Equal(t, "1 bytes", FormatBytes(1))
	require.Equal(t, "1023 bytes", FormatBytes(1023))
	require.Equal(t, "1 KB", FormatBytes(1024))
	require.Equal(t, "1 MB", FormatBytes(1024*1024))
	require.Equal(t, "35 MB", FormatBytes(35*1024*1024))
	require.Equal(t, "1023 MB", FormatBytes(1023*1024*1024))
	require.Equal(t, "1 GB", FormatBytes(1024*1024*1024))
	require.Equal(t, "1 TB", FormatBytes(1024*1024*1024*1024))
	require.Equal(t, "1 PB", FormatBytes(1024*1024*1024*1024*1024))

	goodParse := func(expected int64, s string) {
		val, err := ParseBytes(s)
		require.NoError(t, err)
		require.Equal(t, expected, val)
	}

	goodParse(int64(0), "0")
	goodParse(int64(12345), "12345")
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
		_, err := ParseBytes(s)
		require.Error(t, err)
	}

	badParse("")
	badParse("50 pbz")
	badParse("50.1")
}
