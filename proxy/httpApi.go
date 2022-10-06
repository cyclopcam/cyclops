package proxy

import (
	"encoding/base64"
	"net/http"
	"time"

	"github.com/bmharper/cyclops/pkg/dbh"
	"github.com/bmharper/cyclops/pkg/www"
	"github.com/bmharper/cyclops/proxy/proxymsg"
	"github.com/julienschmidt/httprouter"
	"gorm.io/gorm"
)

// Register a new server
// This API is idempotent
func (p *Proxy) httpRegister(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	// To keep this simple, we limit it to one at a time.
	p.addPeerLock.Lock()
	defer p.addPeerLock.Unlock()

	input := proxymsg.RegisterJSON{}
	www.ReadJSON(w, r, &input, 1024*1024)
	key, err := base64.StdEncoding.DecodeString(input.PublicKey)
	www.Check(err)
	if len(key) != PublicKeyLen {
		www.PanicBadRequestf("Public key must be %v bytes, base64 encoded (provided key is %v bytes)", PublicKeyLen, len(key))
	}

	serverIP := ""

	// Wrap all DB operations in a transaction, so that if the creation of the Wireguard peer
	// fails, then the DB operations also fail.
	// This way we can ensure that our DB state is always in sync with our Wireguard state,
	// and we don't need to do anything like keep a DB field "is_alive_in_wireguard".
	err = p.db.Transaction(func(tx *gorm.DB) error {
		existing := Server{}
		// Using ? doesn't work for this query...
		p.db.Where("public_key = " + dbh.PGByteArrayLiteral(key)).First(&existing)
		if existing.VpnIP != "" {
			serverIP = existing.VpnIP
			return nil
		}

		if time.Since(p.lastPeerAddedAt) < 5*time.Second {
			// Dumb rate limiting.. for my paranoia. Eventually we'll need a better way of vetting new people.. eg email address and captcha validation.
			time.Sleep(5 * time.Second)
		}

		p.log.Infof("Creating new peer %v", input.PublicKey)
		ip, err := p.findFreeIP(tx)
		if err != nil {
			return err
		}
		server := Server{
			PublicKey: key,
			VpnIP:     ip,
			CreatedAt: time.Now().UTC(),
		}
		if err := tx.Create(&server).Error; err != nil {
			return err
		}
		if err := p.wg.createPeers([]Server{server}); err != nil {
			return err
		}
		p.addPeerToCache(key, ip)
		p.lastPeerAddedAt = time.Now()
		serverIP = ip
		p.log.Infof("Created peer %v, with IP %v", input.PublicKey, serverIP)
		return nil
	})
	if err != nil {
		www.PanicBadRequestf("Creation of peer failed: %v", err)
	}
	resp := proxymsg.RegisterResponseJSON{
		ProxyPublicKey:  p.wg.PublicKey.String(),
		ProxyVpnIP:      p.wg.VpnIP,
		ProxyListenPort: p.wg.ListenPort,
		ServerVpnIP:     serverIP,
	}
	www.SendJSON(w, &resp)
}
