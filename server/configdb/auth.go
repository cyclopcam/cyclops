package configdb

import (
	"net"
	"net/http"
	"strings"
)

const KeyMain = "main"

// Returns true if:
// 1. We are not using a VPN
// 2. We are using a VPN, but the caller is not reaching us from it
func (c *ConfigDB) IsCallerOnLAN(r *http.Request) bool {
	if c.VpnAllowedIPs.IP == nil {
		return true
	}
	ipStr, _, _ := strings.Cut(r.RemoteAddr, ":")
	remoteIP := net.ParseIP(ipStr)
	return !c.VpnAllowedIPs.Contains(remoteIP)
}

/*
// Returns the decrypted 32 byte token, or nil
func (c *ConfigDB) DecryptBearerToken(tokenBase64, publicKeyBase64, clientNonceBase64 string) []byte {
	token := [32]byte{}
	n, _ := base64.StdEncoding.Decode(token[:], []byte(tokenBase64))
	if n != 32 {
		return nil
	}

	nonce := [12]byte{}
	n, _ = base64.StdEncoding.Decode(nonce[:], []byte(clientNonceBase64))
	if n != 12 {
		return nil
	}

	sharedKey := c.ComputeSharedKey(publicKeyBase64)
	//c.CreateChaCha20(sharedKey, nonce[:])
	chacha, err := chacha20.NewUnauthenticatedCipher(sharedKey[:], nonce[:])
	if err != nil {
		return nil
	}

	chacha.XORKeyStream(token[:], token[:])
	return token[:]
}

//func (c *ConfigDB) CreateChaCha20(sharedKey [32]byte, nonce []byte) (*chacha20.Cipher, error) {
//	return chacha20.NewUnauthenticatedCipher(sharedKey[:], nonce)
//}

func (c *ConfigDB) ComputeSharedKey(publicKeyBase64 string) [32]byte {
	//c.keyLock.Lock()
	var ownPrivate [32]byte
	copy(ownPrivate[:], c.PrivateKey[:])
	//c.keyLock.Unlock()

	var publicKey [32]byte
	n, _ := base64.StdEncoding.Decode(publicKey[:], []byte(publicKeyBase64))
	if n != 32 {
		return [32]byte{}
	}

	shared := [32]byte{}
	curve25519.ScalarMult(&shared, &ownPrivate, &publicKey)

	return shared
}
*/

//func (c *ConfigDB) SetPrivateKey(privateKey wgtypes.Key) {
//	c.keyLock.Lock()
//	defer c.keyLock.Unlock()
//
//	c.privateKey = privateKey
//}
