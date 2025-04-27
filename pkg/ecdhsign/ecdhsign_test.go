package ecdhsign

import (
	"testing"

	"github.com/stretchr/testify/require"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func TestSignChallenge(t *testing.T) {
	challenge := "abc"
	key1, _ := wgtypes.GeneratePrivateKey()
	key2, _ := wgtypes.GeneratePrivateKey()
	key1Public := key1.PublicKey()
	key2Public := key2.PublicKey()

	signed, err := SignChallenge([]byte(challenge), key1, key2Public)
	require.NoError(t, err)

	require.True(t, VerifyChallenge([]byte(challenge), signed, key2, key1Public))

	key2Corrupt := key2
	for i := 0; i < 10; i++ {
		key2Corrupt[i] = 0
	}
	require.False(t, VerifyChallenge([]byte(challenge), signed, key2Corrupt, key1Public))
}
