package vpn

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"github.com/bmharper/cyclops/pkg/log"
	"github.com/bmharper/cyclops/pkg/requests"
	"github.com/bmharper/cyclops/proxy/kernel"
	"github.com/bmharper/cyclops/proxy/proxymsg"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// package vpn manages our Wireguard setup
// It will create /etc/wireguard/cyclops.conf if necessary, and populate it with
// all the relevant details. Before doing so, it must contact the proxy server,
// which will assign it a VPN IP address.

const ProxyHost = "proxy-cpt.cyclopcam.org"

// VPN is only safe to use by a single thread
type VPN struct {
	Log             log.Log
	PrivateKey      wgtypes.Key
	PublicKey       wgtypes.Key
	client          *kernel.Client
	connectionOK    atomic.Bool
	shutdownStarted chan bool
}

func NewVPN(log log.Log, privateKey, publicKey wgtypes.Key, shutdownStarted chan bool) *VPN {
	v := &VPN{
		Log:             log,
		PrivateKey:      privateKey,
		PublicKey:       publicKey,
		client:          kernel.NewClient(),
		shutdownStarted: shutdownStarted,
	}
	v.registerLoop()
	return v
}

// Connect to our Wireguard interface process
func (v *VPN) ConnectKernelWG() error {
	//v.testAutoReconnectToKernelWG()
	return v.client.Connect("127.0.0.1")
}

// Start our Wireguard device, and save our public key
func (v *VPN) Start() error {
	getResp, err := v.client.GetDevice()
	if err == nil {
		// Device was already up, so we're good to go, provided the key is correct
		return v.validateDeviceDetails(getResp)
	} else if !errors.Is(err, kernel.ErrWireguardDeviceNotExist) {
		return err
	}

	// Try bringing up device
	if err := v.client.BringDeviceUp(); err != nil {
		if !errors.Is(err, kernel.ErrWireguardDeviceNotExist) {
			return err
		}
		// The device does not exist, so we must create it
		if err := v.createDevice(); err != nil {
			return err
		}
		if err := v.client.BringDeviceUp(); err != nil {
			return err
		}
	}

	getResp, err = v.client.GetDevice()
	if err != nil {
		return err
	}
	return v.validateDeviceDetails(getResp)
}

func (v *VPN) createDevice() error {
	// step 1: Register our public key with the global proxy
	v.Log.Infof("Registering with %v", ProxyHost)
	req := proxymsg.RegisterJSON{
		PublicKey: v.PrivateKey.PublicKey().String(),
	}
	resp, err := requests.RequestJSON[proxymsg.RegisterResponseJSON]("POST", "https://"+ProxyHost+"/api/register", &req)
	if err != nil {
		return err
	}
	// Extract the proxy's Wireguard data, for later
	peer := kernel.MsgSetProxyPeerInConfigFile{}
	peer.PublicKey, err = wgtypes.ParseKey(resp.ProxyPublicKey)
	if err != nil {
		return err
	}
	peer.AllowedIP.IP = net.ParseIP(resp.ProxyVpnIP)
	if peer.AllowedIP.IP == nil {
		return fmt.Errorf("Proxy %v has invalid IP '%v'", ProxyHost, resp.ProxyVpnIP)
	}
	// We only accept traffic from the proxy server, and not from any of the other peers.
	// One *could* allow peers to communicate with each other via the proxy, but I don't see the utility,
	// and that seems like a bad idea for security.
	peer.AllowedIP.Mask = net.IPv4Mask(255, 255, 255, 255)

	peer.Endpoint = fmt.Sprintf("%v:%v", ProxyHost, resp.ProxyListenPort)

	// step 2: Create our Wireguard device.
	// We needed to know our VPN IP address before we could do this.
	createMsg := &kernel.MsgCreateDeviceInConfigFile{
		PrivateKey: v.PrivateKey,
		Address:    resp.ServerVpnIP,
	}
	v.Log.Infof("Creating local Wireguard config file")
	if err := v.client.CreateDeviceInConfigFile(createMsg); err != nil {
		return err
	}

	// step 3: Add the proxy as a peer
	v.Log.Infof("Adding proxy peer to local Wireguard config file")
	if err := v.client.SetProxyPeerInConfigFile(&peer); err != nil {
		return err
	}

	return nil
}

func (v *VPN) validateDeviceDetails(resp *kernel.MsgGetDeviceResponse) error {
	if subtle.ConstantTimeCompare(resp.PrivateKey[:], v.PrivateKey[:]) == 0 {
		return fmt.Errorf("Wireguard device has a different key. Delete /etc/wireguard/cyclops.conf, so that it can be recreated.")
	}
	v.connectionOK.Store(true)
	return nil
}

// Keep pinging server so that it knows we're alive.
// In future we should probably remove this, and just rely on the Wireguard pings to
// maintain liveness.
// TODO: this needs to run up front, in case our IP changes
func (v *VPN) registerLoop() {
	nextRegisterAt := time.Now().Add(time.Second)

	minSleep := 5 * time.Second
	maxSleep := 10 * time.Minute
	sleep := minSleep

	go func() {
		for {
		inner:
			select {
			case <-time.After(sleep):
				break inner
			case <-v.shutdownStarted:
				return
			}

			if v.connectionOK.Load() && time.Now().After(nextRegisterAt) {
				req := proxymsg.RegisterJSON{
					PublicKey: v.PrivateKey.PublicKey().String(),
				}
				_, err := requests.RequestJSON[proxymsg.RegisterResponseJSON]("POST", "https://"+ProxyHost+"/api/register", &req)
				if err != nil {
					v.Log.Warnf("Failed to re-register with proxy: %v", err)
					sleep = sleep * 2
					if sleep > maxSleep {
						sleep = maxSleep
					}
				} else {
					v.Log.Infof("Re-register with proxy OK")
					nextRegisterAt = time.Now().Add(12 * time.Hour)
					sleep = minSleep
				}
			}
		}
	}()
}

func (v *VPN) testAutoReconnectToKernelWG() {
	go func() {
		for {
			time.Sleep(3 * time.Second)
			v.Log.Infof("IsDeviceAlive: %v", v.client.IsDeviceAlive())
		}
	}()
}
