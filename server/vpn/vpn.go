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
	privateKey      wgtypes.Key
	publicKey       wgtypes.Key
	client          *kernel.Client
	connectionOK    atomic.Bool
	deviceIP        string // Our IP in the VPN
	shutdownStarted chan bool
}

func NewVPN(log log.Log, privateKey, publicKey wgtypes.Key, shutdownStarted chan bool) *VPN {
	v := &VPN{
		Log:             log,
		privateKey:      privateKey,
		publicKey:       publicKey,
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
		return v.validateAndSaveDeviceDetails(getResp)
	} else if !errors.Is(err, kernel.ErrWireguardDeviceNotExist) {
		return err
	}

	// Try bringing up device
	if err := v.client.BringDeviceUp(); err != nil {
		if !errors.Is(err, kernel.ErrWireguardDeviceNotExist) {
			return err
		}
		// The device does not exist, so we must create it
		if err := v.registerAndCreateDevice(); err != nil {
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
	return v.validateAndSaveDeviceDetails(getResp)
}

func (v *VPN) registerAndCreateDevice() error {
	// step 1: Register our public key with the global proxy
	v.Log.Infof("Registering with %v", ProxyHost)
	req := proxymsg.RegisterJSON{
		PublicKey: v.privateKey.PublicKey().String(),
	}
	resp, err := requests.RequestJSON[proxymsg.RegisterResponseJSON]("POST", "https://"+ProxyHost+"/api/register", &req)
	if err != nil {
		return err
	}
	// step 2: Now that we know our IP in the VPN, we can create our Wireguard device.
	return v.createDevice(resp)
}

func (v *VPN) createDevice(resp *proxymsg.RegisterResponseJSON) error {
	// Extract the proxy's Wireguard data, for later
	var err error
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

	// We needed to know our VPN IP address before we could do this.
	createMsg := &kernel.MsgCreateDeviceInConfigFile{
		PrivateKey: v.privateKey,
		Address:    resp.ServerVpnIP,
	}
	v.Log.Infof("Creating local Wireguard config file")
	if err := v.client.CreateDeviceInConfigFile(createMsg); err != nil {
		return err
	}

	// Add the proxy as a peer
	v.Log.Infof("Adding proxy peer to local Wireguard config file")
	if err := v.client.SetProxyPeerInConfigFile(&peer); err != nil {
		return err
	}

	return nil
}

func (v *VPN) validateAndSaveDeviceDetails(resp *kernel.MsgGetDeviceResponse) error {
	if subtle.ConstantTimeCompare(resp.PrivateKey[:], v.privateKey[:]) == 0 {
		return fmt.Errorf("Wireguard device has a different key. Delete /etc/wireguard/cyclops.conf, so that it can be recreated.")
	}
	v.deviceIP = resp.Address
	v.connectionOK.Store(true)
	v.Log.Infof("VPN IP is %v", v.deviceIP)
	return nil
}

// Keep pinging server so that it knows we're alive.
// Also, if we've been dormant for a long time, then the proxy may have culled us,
// and we may not receive a new VPN IP, so that's also why this system is essential.
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
					PublicKey: v.privateKey.PublicKey().String(),
				}
				response, err := requests.RequestJSON[proxymsg.RegisterResponseJSON]("POST", "https://"+ProxyHost+"/api/register", &req)
				if err != nil {
					v.Log.Warnf("Failed to re-register with proxy: %v", err)
					sleep = sleep * 2
					if sleep > maxSleep {
						sleep = maxSleep
					}
				} else {
					if response.ServerVpnIP != v.deviceIP {
						v.Log.Infof("VPN IP has changed. Recreating Wireguard device")
						if err := v.recreateDevice(response); err != nil {
							v.Log.Errorf("Recreating of Wireguard device failed: %v", err)
							nextRegisterAt = time.Now().Add(5 * time.Minute)
						} else {
							v.Log.Errorf("New VPN IP is %v", v.deviceIP)
							nextRegisterAt = time.Now().Add(12 * time.Hour)
						}
					} else {
						v.Log.Infof("Re-register with proxy OK")
						nextRegisterAt = time.Now().Add(12 * time.Hour)
					}
					sleep = minSleep
				}
			}
		}
	}()
}

func (v *VPN) recreateDevice(register *proxymsg.RegisterResponseJSON) error {
	err := v.client.TakeDeviceDown()
	if err != nil && !errors.Is(err, kernel.ErrWireguardDeviceNotExist) {
		return err
	}

	if err := v.createDevice(register); err != nil {
		return err
	}

	if err := v.client.BringDeviceUp(); err != nil {
		return err
	}

	getResp, err := v.client.GetDevice()
	if err != nil {
		return err
	}
	return v.validateAndSaveDeviceDetails(getResp)
}

func (v *VPN) testAutoReconnectToKernelWG() {
	go func() {
		for {
			time.Sleep(3 * time.Second)
			v.Log.Infof("IsDeviceAlive: %v", v.client.IsDeviceAlive())
		}
	}()
}
