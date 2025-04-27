package ecdhsign

import (
	"crypto/hmac"
	"crypto/sha256"

	"golang.org/x/crypto/curve25519"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// SignChallenge signs a challenge message using the ECDH key as input to a SHA256 HMAC.
func SignChallenge(challenge []byte, privateKey, publicKey wgtypes.Key) ([]byte, error) {
	// Generate shared secret
	sharedSecret, err := curve25519.X25519(privateKey[:], publicKey[:])
	if err != nil {
		return nil, err
	}

	//fmt.Printf("Shared secret: %x (from %x and %x), Message %x\n", sharedSecret, privateKey, publicKey, challenge)

	// Create HMAC with SHA256
	h := hmac.New(sha256.New, sharedSecret)
	h.Write(challenge)
	return h.Sum(nil), nil
}

// VerifyChallenge verifies a challenge message using the ECDH key as input to a SHA256 HMAC.
func VerifyChallenge(challenge, signature []byte, privateKey, publicKey wgtypes.Key) bool {
	// Generate shared secret
	sharedSecret, err := curve25519.X25519(privateKey[:], publicKey[:])
	if err != nil {
		return false
	}

	// Create HMAC with SHA256
	h := hmac.New(sha256.New, sharedSecret)
	h.Write(challenge)
	expectedSignature := h.Sum(nil)

	return hmac.Equal(signature, expectedSignature)
}
