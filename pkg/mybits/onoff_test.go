package mybits

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func testOnOff(t *testing.T, bits []byte, expectedLength int) {
	output := make([]byte, 100)
	encodedLength, err := EncodeOnoff(bits, output)
	require.NoError(t, err)
	require.Equal(t, expectedLength, encodedLength)

	decoded := make([]byte, len(bits))
	decodedBits, err := DecodeOnoff(output[:encodedLength], decoded)
	require.NoError(t, err)
	require.Equal(t, len(bits)*8, decodedBits)

	// stress the "buffer not large enough" decode path
	decoded = make([]byte, len(bits)-1)
	decodedBits, err = DecodeOnoff(output[:encodedLength], decoded)
	require.Equal(t, ErrOutOfSpace, err)

	if encodedLength > 0 {
		// stress the "buffer not large enough" encode path
		encodedLength, err = EncodeOnoff(bits, output[:encodedLength-1])
		require.Equal(t, ErrOutOfSpace, err)
	}
}

func TestOnOff(t *testing.T) {
	testOnOff(t, []byte{0x00}, 1)
	testOnOff(t, []byte{0xff}, 2)
	testOnOff(t, []byte{0xff, 0x00}, 3)
	testOnOff(t, []byte{0xff, 0x10}, 5)
}
