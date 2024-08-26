package videox

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEncodeAnnexB(t *testing.T) {
	verify := func(raw, expect []byte) {
		e1 := EncodeAnnexB(raw, 0, AnnexBEncodeFlagAddEmulationPreventionBytes)
		e2 := EncodeAnnexB(raw, 3, AnnexBEncodeFlagAddEmulationPreventionBytes)
		// NALU prefix
		require.Equal(t, len(e1)+3, len(e2))
		require.Equal(t, []byte{0, 0, 1}, e2[:3])
		// encoded byte stream
		require.Equal(t, expect, e1)
		require.Equal(t, expect, e2[3:])
		// round trip
		decoded := DecodeAnnexB(e1)
		require.Equal(t, raw, decoded)
	}

	verify([]byte{}, []byte{})
	verify([]byte{0}, []byte{0})
	verify([]byte{0, 0}, []byte{0, 0})
	verify([]byte{0, 0, 1}, []byte{0, 0, 3, 1})

	{
		// this one needs the 2nd pass, where we increase the buffer size
		nEscapes := 100
		raw := make([]byte, nEscapes*3)
		encoded := make([]byte, nEscapes*4)
		for i := 0; i < nEscapes; i++ {
			raw[i*3] = 0
			raw[i*3+1] = 0
			raw[i*3+2] = 2
			encoded[i*4] = 0
			encoded[i*4+1] = 0
			encoded[i*4+2] = 3
			encoded[i*4+3] = 2
		}
		verify(raw, encoded)
	}

}
