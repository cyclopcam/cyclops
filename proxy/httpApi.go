package proxy

import (
	"encoding/base64"
	"net/http"
	"time"

	"github.com/cyclopcam/cyclops/pkg/www"
	"github.com/cyclopcam/cyclops/proxy/proxymsg"
	"github.com/cyclopcam/dbh"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"gorm.io/gorm"
)

// Register a new server
// This API is idempotent, and it is intended to be used by clients once a day to refresh
// their liveliness.
func (p *Proxy) httpRegister(w http.ResponseWriter, r *http.Request) {
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
		// Using ? doesn't work for this query, because of the BLOB search, so we hardcode the entire query string
		wherePubkey := "public_key = " + dbh.PGByteArrayLiteral(key)
		p.db.Where(wherePubkey).First(&existing)
		if existing.VpnIP != "" {
			// Update last_seen_at. We should really use Wireguard data too, because this is unauthenticated.
			serverIP = existing.VpnIP
			if err := p.db.Model(&existing).Update("last_register_at", "now()").Error; err != nil {
				p.log.Warnf("Failed to update last_register_at: %v", err)
			}
			return nil
		}

		p.log.Infof("Creating new peer %v", input.PublicKey)
		ip, err := p.findFreeIP(tx)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		server := Server{
			PublicKey:      key,
			VpnIP:          ip,
			CreatedAt:      now,
			LastRegisterAt: now,
			// LastTrafficAt is null here, because the caller has not authenticated over Wireguard
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

func (p *Proxy) httpRemove(w http.ResponseWriter, r *http.Request) {
	req := proxymsg.RemoveJSON{}
	www.ReadJSON(w, r, &req, 1024*1024)
	key, err := wgtypes.ParseKey(req.PublicKey)
	www.Check(err)
	p.log.Infof("Forcibly removing peer %v", key)

	www.Check(p.removePeer(key))
	www.SendOK(w)
}

func (p *Proxy) removePeer(publicKey wgtypes.Key) error {
	p.log.Infof("Removing peer %v", publicKey)

	err := p.db.Transaction(func(tx *gorm.DB) error {
		server := Server{}
		wherePubkey := "public_key = " + dbh.PGByteArrayLiteral(publicKey[:])
		p.db.Where(wherePubkey).First(&server)
		if server.ID == 0 {
			// does not exist
			return nil
		}
		if err := p.wg.removePeer(server); err != nil {
			return err
		}
		freeIP := IPFreePool{VpnIP: server.VpnIP}
		if err := p.db.Create(&freeIP).Error; err != nil {
			return err
		}
		if err := p.db.Delete(&server).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	p.removePeerFromCache(publicKey[:])
	return nil
}
