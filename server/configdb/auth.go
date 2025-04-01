package configdb

import (
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/cyclopcam/www"
)

const KeyMain = "main"

// VerifiedIdentity is an identity that accounts.cyclopcam.org has verified
type VerifiedIdentity struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
}

// Returns true if:
// 1. We are not using a VPN
// 2. We are using a VPN, but the caller is not reaching us from it
func (c *ConfigDB) IsCallerOnLAN(r *http.Request) bool {
	if c.VpnAllowedIP.IP == nil {
		return true
	}
	ipStr, _, _ := strings.Cut(r.RemoteAddr, ":")
	remoteIP := net.ParseIP(ipStr)
	return !c.VpnAllowedIP.Contains(remoteIP)
}

// Ask accounts.cyclopcam.org for information about this token.
// These tokens are generated by a user to prove that they are who they claim to be.
// These tokens have a short expiration time (eg 3 minutes).
func GetVerifiedIdentityFromToken(token string) (*VerifiedIdentity, error) {
	r, _ := http.NewRequest("GET", "https://accounts.cyclopcam.org/api/auth/checkIdentityToken?token="+token, nil)
	identity := &VerifiedIdentity{}
	err := www.FetchJSON(r, identity)
	if err != nil {
		return nil, err
	} else if identity.ID == "" {
		return nil, errors.New("Invalid identity token")
	}
	return identity, nil
}
