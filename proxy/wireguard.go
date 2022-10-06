package proxy

import (
	"errors"
	"fmt"
	"net"

	"github.com/bmharper/cyclops/pkg/log"
	"github.com/bmharper/cyclops/proxy/kernel"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"gorm.io/gorm"
)

// We only use /etc/wireguard/cyclops.conf for configuring the WireGuard interface.
// All peers are created and destroyed manually, by this code.
// In order to minimize downtime, we support restarting of
// this proxy server without recreating all the WireGuard peers.
// When we start, we check if the 'cyclops' wireguard device is
// alive. If it is alive, then we assume that all peers are also
// alive, and vice versa. In other words, we assume that the Wireguard
// state is 100% synchronized with our own state.
// When the server reboots, the wireguard device is not created automatically,
// so we start from a clean slate in this case.

type wireGuard struct {
	PublicKey  wgtypes.Key // Public Key of Wireguard device
	ListenPort int         // Real port that Wireguard device listens on
	VpnIP      string      // VPN IP of Wireguard device (eg 10.6.0.0)

	log    log.Log
	db     *gorm.DB
	client *kernel.Client
}

func newWireGuard(proxy *Proxy) (*wireGuard, error) {
	client := kernel.NewClient()
	if err := client.Connect(proxy.kernelwgHost); err != nil {
		return nil, err
	}

	return &wireGuard{
		log:    proxy.log,
		db:     proxy.db,
		client: client,
	}, nil
}

// Brings up the wireguard interface and all peers
func (w *wireGuard) boot() error {
	device, err := w.client.GetDevice()

	if err != nil && errors.Is(err, kernel.ErrWireguardDeviceNotExist) {
		w.log.Infof("Starting Wireguard")

		// Create the Wireguard device
		if err := w.client.BringDeviceUp(); err != nil {
			return err
		}

		// Get details
		device, err = w.client.GetDevice()
		if err != nil {
			return err
		}

		// Create wireguard peers and setup IP routes
		if err := w.createAllPeers(); err != nil {
			return err
		}
	} else if err != nil {
		return fmt.Errorf("Error reading Wireguard device: %w", err)
	} else {
		w.log.Infof("Wireguard is already running, no state change necessary")
	}

	// Unfortunately the wgctrl interface does not tell us the server's IP address,
	// so we just hardcode it. I don't understand why that data is excluded... it seems
	// like a natural thing to have there.
	w.PublicKey = device.PrivateKey.PublicKey()
	w.ListenPort = device.ListenPort
	w.VpnIP = ProxyAddr

	w.log.Infof("Wireguard public key is %v", w.PublicKey)
	w.log.Infof("Wireguard port is %v", w.ListenPort)
	w.log.Infof("Wireguard VPN IP is %v", w.VpnIP)

	return nil
}

// createAllPeers reads all peers out of the DB, and creates them in Wireguard
func (w *wireGuard) createAllPeers() error {
	servers := []Server{}
	if err := w.db.Find(&servers).Error; err != nil {
		return err
	}
	return w.createPeers(servers)
}

func (w *wireGuard) createPeers(servers []Server) error {
	w.log.Infof("Creating %v Wireguard peers", len(servers))

	if len(servers) == 0 {
		return nil
	}

	msg := kernel.MsgCreatePeersInMemory{}
	for _, server := range servers {
		peer := kernel.CreatePeerInMemory{}
		copy(peer.PublicKey[:], server.PublicKey)
		peer.AllowedIP.IP = net.ParseIP(server.VpnIP)
		if peer.AllowedIP.IP == nil {
			return fmt.Errorf("Server %v has invalid IP '%v'", server.ID, server.VpnIP)
		}
		peer.AllowedIP.Mask = net.IPv4Mask(255, 255, 255, 255)
		msg.Peers = append(msg.Peers, peer)
	}
	return w.client.CreatePeers(&msg)
}
