package fsv

import (
	"testing"

	"github.com/cyclopcam/cyclops/pkg/videoformat/rf1"
	"github.com/stretchr/testify/require"
)

func TestRf1Flags(t *testing.T) {
	flagsA := []rf1.IndexNALUFlags{
		rf1.IndexNALUFlagKeyFrame,
		rf1.IndexNALUFlagEssentialMeta,
		rf1.IndexNALUFlagAnnexB,
	}
	flagsB := []NALUFlags{
		NALUFlagKeyFrame,
		NALUFlagEssentialMeta,
		NALUFlagAnnexB,
	}
	for i := range flagsA {
		require.Equal(t, flagsB[i], copyRf1FlagsToFsv(flagsA[i]))
		require.Equal(t, flagsA[i], copyFsvFlagsToRf1(flagsB[i]))
	}
}
