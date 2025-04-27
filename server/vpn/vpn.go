package vpn

import (
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"github.com/cyclopcam/cyclops/pkg/requests"
	"github.com/cyclopcam/logs"
	"github.com/cyclopcam/proxyapi"
	"github.com/cyclopcam/safewg/wguser"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// package vpn manages our Wireguard client setup
// It will create /etc/wireguard/cyclops.conf if necessary, and populate it with
// all the relevant details. Before doing so, it must contact the proxy server,
// which will assign it a VPN IP address.

// At some point, if we have multiple geo-relevant proxies, then we'd choose the closest
// server instead of just hard-coding to a single one.
const ProxyHost = "proxy-cpt.cyclopcam.org"

// Our proxied host name is hex(publicKey[:ShortPublicKeyLen]).p.cyclopcam.org
func ProxiedHostName(publicKey wgtypes.Key) string {
	return hex.EncodeToString(publicKey[:ShortPublicKeyLen]) + ".p.cyclopcam.org"
}

// Our VPN supports either IPv4 or IPv6
type IPNetwork string

const (
	IPv4 IPNetwork = "IPv4" // IPv4 - Must match the string required for the 'register' API of the proxy service
	IPv6 IPNetwork = "IPv6" // IPv6 - Must match the string required for the 'register' API of the proxy service
)

// Used for host names, encoded as hex. First 10 bytes of public key.
// SYNC-SHORT-PUBLIC-KEY-LEN
const ShortPublicKeyLen = 10

// Manage connection to our VPN/proxy server
type VPN struct {
	Log           logs.Log
	AllowedIP     net.IPNet // Network of the VPN (actually just the proxy server's addresses, eg 10.6.0.0/32 or fdce:c10b:5ca1:1::1/128)
	ipNetwork     IPNetwork // IPv4 or IPv6
	deviceName    string    // "cyclops"
	privateKey    wgtypes.Key
	publicKey     wgtypes.Key
	client        *wguser.Client
	connectionOK  atomic.Bool
	ownDeviceIP   string // Our IP in the VPN, such as 10.7.0.99 or fdce:c10b:5ca1:2::99
	hasRegistered atomic.Bool
}

func NewVPN(log logs.Log, privateKey wgtypes.Key, wgkernelClientSecret string, forceIPv4 bool) *VPN {
	ipVersion := IPv6
	if forceIPv4 {
		ipVersion = IPv4
	}
	v := &VPN{
		Log:        log,
		ipNetwork:  ipVersion,
		deviceName: "cyclops",
		privateKey: privateKey,
		publicKey:  privateKey.PublicKey(),
		client:     wguser.NewClient(wgkernelClientSecret),
	}
	return v
}

// Connect to our Wireguard interface process
func (v *VPN) ConnectKernelWG() error {
	//v.testAutoReconnectToKernelWG()
	return v.client.Connect()
}

func (v *VPN) DisconnectKernelWG() {
	v.client.Close()
}

// Start our Wireguard device, and save our public key
func (v *VPN) Start() error {
	getResp, err := v.client.GetDevice(v.deviceName)
	if err == nil {
		// Device was already up, so we're good to go, provided the key is correct
		return v.validateAndSaveDeviceDetails(getResp)
	} else if !errors.Is(err, wguser.ErrWireguardDeviceNotExist) {
		return fmt.Errorf("client.GetDevice (#1) failed: %w", err)
	}

	// Temporarily raise the timeout.
	// Cyclops sometimes fails to start on reboot, and I'm wondering if my timeout is just too short.
	// So this is a test. The default timeout is 10 seconds.
	// Hmm.. so in a test I just ran, BringDeviceUp takes 5 seconds.
	// The only time my test device ever reboots is when all the power goes down. In these events, the wifi/fiber/ethernet
	// goes down too. So maybe the slow startup is due to the network being down. Doesn't make sense though - why would
	// bringing up the wireguard device be dependent on the internet being up?
	timeout := v.client.GetMaxReadDuration()
	defer v.client.SetMaxReadDuration(timeout)
	v.client.SetMaxReadDuration(30 * time.Second)

	// Try bringing up device
	if err := v.client.BringDeviceUp(v.deviceName); err != nil {
		if !errors.Is(err, wguser.ErrWireguardDeviceNotExist) {
			return fmt.Errorf("client.BringDeviceUp (#1) failed: %w", err)
		}
		// The device does not exist, so we must create it
		if err := v.registerAndCreateDevice(); err != nil {
			return fmt.Errorf("v.registerAndCreateDevice failed: %w", err)
		}
		if err := v.client.BringDeviceUp(v.deviceName); err != nil {
			return fmt.Errorf("client.BringDeviceUp (#2) failed: %w", err)
		}
	}

	getResp, err = v.client.GetDevice(v.deviceName)
	if err != nil {
		return fmt.Errorf("client.GetDevice (#2) failed: %w", err)
	}
	return v.validateAndSaveDeviceDetails(getResp)
}

func (v *VPN) registerAndCreateDevice() error {
	// step 1: Register our public key with the global proxy
	v.Log.Infof("Registering with %v", ProxyHost)
	req := proxyapi.RegisterJSON{
		PublicKey: v.privateKey.PublicKey().String(),
		Network:   string(v.ipNetwork),
	}
	resp, err := requests.RequestJSON[proxyapi.RegisterResponseJSON]("POST", "https://"+ProxyHost+"/api/register", &req)
	if err != nil {
		return err
	}
	v.hasRegistered.Store(true)
	// step 2: Now that we know our IP in the VPN, we can create our Wireguard device.
	return v.createDevice(resp)
}

func (v *VPN) createDevice(resp *proxyapi.RegisterResponseJSON) error {
	// Extract the proxy's Wireguard data, for later
	var err error
	peer := wguser.MsgSetProxyPeerInConfigFile{
		DeviceName: v.deviceName,
	}
	peer.PublicKey, err = wgtypes.ParseKey(resp.ProxyPublicKey)
	if err != nil {
		return err
	}
	var allowedIP net.IPNet
	allowedIP.IP = net.ParseIP(resp.ProxyVpnIP)
	if allowedIP.IP == nil {
		return fmt.Errorf("Proxy %v has invalid IP '%v'", ProxyHost, resp.ProxyVpnIP)
	}
	// We only accept traffic from the proxy server, and not from any of the other peers.
	// One *could* allow peers to communicate with each other via the proxy, but I don't see the utility,
	// and that seems like a bad idea for security.
	if allowedIP.IP.To4() != nil {
		allowedIP.Mask = net.IPv4Mask(255, 255, 255, 255)
	} else {
		allowedIP.Mask = net.CIDRMask(128, 128)
	}
	peer.AllowedIPs = []net.IPNet{allowedIP}

	peer.Endpoint = fmt.Sprintf("%v:%v", ProxyHost, resp.ProxyListenPort)

	// We needed to know our VPN IP address before we could do this.
	createMsg := &wguser.MsgCreateDeviceInConfigFile{
		DeviceName: v.deviceName,
		PrivateKey: v.privateKey,
		Addresses:  []string{resp.ServerVpnIP},
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

func (v *VPN) validateAndSaveDeviceDetails(resp *wguser.MsgGetDeviceResponse) error {
	if subtle.ConstantTimeCompare(resp.PrivateKey[:], v.privateKey[:]) == 0 {
		color := "\033[0;32m"
		reset := " \033[0m"
		v.Log.Errorf("%vWireguard key in Cyclops config database differs from /etc/wireguard/cyclops.conf:%v", color, reset)
		v.Log.Infof("%vThere are two ways to fix this:%v", color, reset)
		v.Log.Infof("%v1. Force a new key to be created, by deleting your cyclops wireguard interface:%v", color, reset)
		v.Log.Infof("%v a. sudo wg-quick down cyclops%v", color, reset)
		v.Log.Infof("%v b. sudo rm /etc/wireguard/cyclops.conf%v", color, reset)
		v.Log.Infof("%v c. Try starting cyclops again%v", color, reset)
		v.Log.Infof("%v2. Reuse your existing wireguard key:%v", color, reset)
		v.Log.Infof("%v a. sudo cat /etc/wireguard/cyclops.conf%v", color, reset)
		v.Log.Infof("%v b. Use the private key displayed in the console, and run cyclops once with --privatekey <key>%v", color, reset)
		v.Log.Infof("%v c. Start cyclops regularly again%v", color, reset)
		return fmt.Errorf("Wireguard device has a different key. Follow instructions in the logs.")
	}
	peers, err := v.client.GetPeers(v.deviceName)
	if err != nil {
		return fmt.Errorf("client.GetPeers failed: %w", err)
	}
	if len(peers.Peers) != 1 {
		return fmt.Errorf("Expected 1 peer, but got %v", len(peers.Peers))
	}
	if len(peers.Peers[0].AllowedIPs) != 1 {
		return fmt.Errorf("Expected 1 AllowedIPs on peer, but got %v", len(peers.Peers[0].AllowedIPs))
	}
	if len(resp.Addresses) != 1 {
		return fmt.Errorf("Expected 1 address, but got %v", len(resp.Addresses))
	}
	v.ownDeviceIP = resp.Addresses[0]
	v.AllowedIP = peers.Peers[0].AllowedIPs[0]
	v.connectionOK.Store(true)
	v.Log.Infof("VPN own IP is %v, proxy AllowedIPs is %v", v.ownDeviceIP, v.AllowedIP)
	return nil
}

// Keep pinging server so that it knows we're alive.
// Also, if we've been dormant for a long time, then the proxy may have culled us,
// and we may not receive a new VPN IP, so that's also why this system is essential.
func (v *VPN) RunRegisterLoop(exit chan bool) {
	registerInterval := 4 * time.Hour

	nextRegisterAt := time.Now().Add(time.Second)
	if v.hasRegistered.Load() {
		// This code path gets hit on first time startup, where we make first contact
		// with the proxy. It's confusing to see two registrations in the logs,
		// so that's really the only reason why this code path exists.
		nextRegisterAt = time.Now().Add(registerInterval)
	}

	minSleep := 5 * time.Second
	maxSleep := 60 * time.Minute
	sleep := minSleep

	go func() {
		for {
		inner:
			select {
			case <-time.After(sleep):
				break inner
			case <-exit:
				return
			}

			if v.connectionOK.Load() && time.Now().After(nextRegisterAt) {
				req := proxyapi.RegisterJSON{
					PublicKey: v.privateKey.PublicKey().String(),
					Network:   string(v.ipNetwork),
				}
				response, err := requests.RequestJSON[proxyapi.RegisterResponseJSON]("POST", "https://"+ProxyHost+"/api/register", &req)
				if err != nil {
					v.Log.Warnf("Failed to re-register with proxy: %v", err)
					sleep = min(sleep*2, maxSleep)
				} else {
					v.hasRegistered.Store(true)
					if response.ServerVpnIP != v.ownDeviceIP {
						v.Log.Infof("VPN IP has changed. Recreating Wireguard device")
						if err := v.recreateDevice(response); err != nil {
							v.Log.Errorf("Recreating of Wireguard device failed: %v", err)
							nextRegisterAt = time.Now().Add(time.Minute)
						} else {
							v.Log.Errorf("New VPN IP is %v", v.ownDeviceIP)
							nextRegisterAt = time.Now().Add(registerInterval)
						}
					} else {
						v.Log.Infof("Re-register with proxy OK")
						nextRegisterAt = time.Now().Add(registerInterval)
					}
					sleep = minSleep
				}
			}
		}
	}()
}

func (v *VPN) recreateDevice(register *proxyapi.RegisterResponseJSON) error {
	err := v.client.TakeDeviceDown(v.deviceName)
	if err != nil && !errors.Is(err, wguser.ErrWireguardDeviceNotExist) {
		return err
	}

	if err := v.createDevice(register); err != nil {
		return err
	}

	if err := v.client.BringDeviceUp(v.deviceName); err != nil {
		return err
	}

	getResp, err := v.client.GetDevice(v.deviceName)
	if err != nil {
		return err
	}
	return v.validateAndSaveDeviceDetails(getResp)
}

func (v *VPN) testAutoReconnectToKernelWG() {
	go func() {
		for {
			time.Sleep(3 * time.Second)
			v.Log.Infof("IsDeviceAlive: %v", v.client.IsDeviceAlive(v.deviceName))
		}
	}()
}
