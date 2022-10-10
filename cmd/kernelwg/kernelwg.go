package main

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"

	"github.com/bmharper/cyclops/pkg/log"
	"github.com/bmharper/cyclops/proxy/kernel"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// This is the name of our wireguard device, which is the same on the proxy server and on a camera server.
const WireguardDeviceName = "cyclops"
const Debug = false

type handler struct {
	log            log.Log
	conn           net.Conn
	wg             *wgctrl.Client
	requestBuffer  bytes.Buffer
	responseBuffer bytes.Buffer
	decoder        *gob.Decoder
	encoder        *gob.Encoder
}

func (h *handler) handleBringDeviceUp() error {
	h.log.Infof("Bring up Wireguard device %v", WireguardDeviceName)

	// Check first if the config file exists, so that we can return a definitive "does not exist" error.
	if _, err := os.Stat(configFilename()); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return kernel.ErrWireguardDeviceNotExist
		}
	}

	cmd := exec.Command("wg-quick", "up", WireguardDeviceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		h.log.Infof("Device %v activation failed: %v. Output: %v", WireguardDeviceName, err, string(output))
		return fmt.Errorf("%w: %v", err, string(output))
	}
	h.log.Infof("Device %v activation OK", WireguardDeviceName)
	return nil
}

func (h *handler) handleTakeDeviceDown() error {
	h.log.Infof("Taking down Wireguard device %v", WireguardDeviceName)

	// Check first if the config file exists, so that we can return a definitive "does not exist" error.
	if _, err := os.Stat(configFilename()); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return kernel.ErrWireguardDeviceNotExist
		}
	}

	cmd := exec.Command("wg-quick", "down", WireguardDeviceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		h.log.Infof("Device %v takedown failed: %v. Output: %v", WireguardDeviceName, err, string(output))
		return fmt.Errorf("%w: %v", err, string(output))
	}
	h.log.Infof("Device %v is down", WireguardDeviceName)
	return nil
}

func (h *handler) handleIsDeviceAlive() error {
	_, err := h.wg.Device(WireguardDeviceName)
	if errors.Is(err, os.ErrNotExist) {
		return kernel.ErrWireguardDeviceNotExist
	}
	return err
}

func (h *handler) handleGetDevice() (any, error) {
	device, err := h.wg.Device(WireguardDeviceName)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, kernel.ErrWireguardDeviceNotExist
		}
		return nil, err
	}
	// This is a hack here, because we're mixing kernel-provided Wireguard state with the config file.
	// These two may be out of sync, which is why this is bad.
	// The reason I'm doing this, is because I need to know our IP address in the VPN, and I can't
	// see a cleaner way of doing this, than by looking at the Wireguard config file.
	// An alternative would be to read the output of "ip -4 address", but I've already got logic in here
	// for parsing Wireguard config files, so I'm using that.
	cfg, err := loadConfigFile(configFilename())
	address := ""
	if err == nil {
		iface := cfg.findSectionByTitle("Interface")
		if iface != nil {
			a := iface.get("Address")
			if a != nil {
				address = *a
			}
		}
	}

	resp := kernel.MsgGetDeviceResponse{
		PrivateKey: device.PrivateKey,
		ListenPort: device.ListenPort,
		Address:    address,
	}
	return &resp, nil
}

func (h *handler) handleGetPeers() (any, error) {
	device, err := h.wg.Device(WireguardDeviceName)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, kernel.ErrWireguardDeviceNotExist
		}
		return nil, err
	}
	resp := kernel.MsgGetPeersResponse{}
	for _, pi := range device.Peers {
		resp.Peers = append(resp.Peers, kernel.Peer{
			PublicKey:                   pi.PublicKey,
			PersistentKeepaliveInterval: pi.PersistentKeepaliveInterval,
			LastHandshakeTime:           pi.LastHandshakeTime,
			ReceiveBytes:                pi.ReceiveBytes,
			TransmitBytes:               pi.TransmitBytes,
			AllowedIPs:                  pi.AllowedIPs,
		})
	}
	return &resp, nil
}

// Create peers in memory. They are not saved to the config file.
// This is used by the proxy for bringing peers online.
func (h *handler) handleCreatePeersInMemory(request *kernel.MsgCreatePeersInMemory) error {
	h.log.Infof("Creating %v peers", len(request.Peers))
	cfg := wgtypes.Config{
		ReplacePeers: false, // If this is false, then we append peers, which is what we want
	}
	for _, p := range request.Peers {
		cfg.Peers = append(cfg.Peers, wgtypes.PeerConfig{
			PublicKey:  p.PublicKey,
			AllowedIPs: []net.IPNet{p.AllowedIP},
		})
	}
	if err := h.wg.ConfigureDevice(WireguardDeviceName, cfg); err != nil {
		return err
	}

	// Create IP routes
	for _, p := range request.Peers {
		// ip -4 route add 10.100.1.1/32 dev cyclops
		cmd := exec.Command("ip", "-4", "route", "add", p.AllowedIP.String(), "dev", WireguardDeviceName)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("Error creating IP route to %v: %w", p.AllowedIP.String(), err)
		}
	}

	return nil
}

func (h *handler) handleRemovePeerInMemory(request *kernel.MsgRemovePeerInMemory) error {
	h.log.Infof("Removing peer %v", request.PublicKey)
	cfg := wgtypes.Config{}
	cfg.Peers = append(cfg.Peers, wgtypes.PeerConfig{
		PublicKey: request.PublicKey,
		Remove:    true,
	})
	if err := h.wg.ConfigureDevice(WireguardDeviceName, cfg); err != nil {
		return err
	}

	// Delete IP route
	// ip -4 route delete 10.101.1.2/32 dev cyclops
	cmd := exec.Command("ip", "-4", "route", "delete", request.AllowedIP.String(), "dev", WireguardDeviceName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Error deleting IP route to %v: %w", request.AllowedIP.String(), err)
	}

	return nil
}

func configFilename() string {
	return fmt.Sprintf("/etc/wireguard/%v.conf", WireguardDeviceName)
}

// This is used on a Cyclops server when it is setting up it's Wireguard interface to
// the proxy server. The purpose of this function is to create the initial /etc/wireguard/cyclops.conf
// file, and/or set the [Interface] section at the top of that file.
func (h *handler) handleCreateDeviceInConfigFile(request *kernel.MsgCreateDeviceInConfigFile) error {
	h.log.Infof("Creating %v", configFilename())
	cfg, err := loadConfigFile(configFilename())
	if errors.Is(err, os.ErrNotExist) {
		cfg = &configFile{}
	} else if err != nil {
		return err
	}

	iface := cfg.findSectionByTitle("Interface")
	if iface == nil {
		iface = cfg.addSection("Interface")
	}

	iface.set("PrivateKey", request.PrivateKey.String())
	iface.set("Address", request.Address)

	return cfg.writeFile(configFilename())
}

// This is used on a Cyclops server when it is setting up it's Wireguard interface to
// the proxy server. The purpose of this function is to add the [Peer] section to
// /etc/wireguard/cyclops.conf that points to our proxy server.
func (h *handler) handleSetProxyPeerInConfigFile(request *kernel.MsgSetProxyPeerInConfigFile) error {
	h.log.Infof("Setting proxy peer in cyclops.conf")
	cfg, err := loadConfigFile(configFilename())
	if err != nil {
		return err
	}

	peer := cfg.findSectionByKeyValue("Peer", "PublicKey", request.PublicKey.String())
	if peer == nil {
		peer = cfg.addSection("Peer")
	}

	peer.set("PublicKey", request.PublicKey.String())
	peer.set("Endpoint", request.Endpoint)
	peer.set("AllowedIPs", request.AllowedIP.String())
	peer.set("PersistentKeepalive", "25")

	return cfg.writeFile(configFilename())
}

func (h *handler) handleMessage(msgType kernel.MsgType, msgLen int) error {
	if Debug {
		h.log.Infof("handleMessage %v, %v bytes", msgType, msgLen)
	}

	// Decode request, if any
	var request any
	switch msgType {
	case kernel.MsgTypeCreatePeersInMemory:
		request = &kernel.MsgCreatePeersInMemory{}
	case kernel.MsgTypeRemovePeerInMemory:
		request = &kernel.MsgRemovePeerInMemory{}
	case kernel.MsgTypeCreateDeviceInConfigFile:
		request = &kernel.MsgCreateDeviceInConfigFile{}
	case kernel.MsgTypeSetProxyPeerInConfigFile:
		request = &kernel.MsgSetProxyPeerInConfigFile{}
	}
	if request != nil {
		err := h.decoder.Decode(request)
		if err != nil {
			h.log.Errorf("Error decoding request: %v", err)
			return fmt.Errorf("Error decoding request: %w", err)
		}
	}

	respType := kernel.MsgTypeNone
	var resp any
	var err error
	switch msgType {
	case kernel.MsgTypeGetPeers:
		respType = kernel.MsgTypeGetPeersResponse
		resp, err = h.handleGetPeers()
	case kernel.MsgTypeGetDevice:
		respType = kernel.MsgTypeGetDeviceResponse
		resp, err = h.handleGetDevice()
	case kernel.MsgTypeCreatePeersInMemory:
		err = h.handleCreatePeersInMemory(request.(*kernel.MsgCreatePeersInMemory))
	case kernel.MsgTypeRemovePeerInMemory:
		err = h.handleRemovePeerInMemory(request.(*kernel.MsgRemovePeerInMemory))
	case kernel.MsgTypeCreateDeviceInConfigFile:
		err = h.handleCreateDeviceInConfigFile(request.(*kernel.MsgCreateDeviceInConfigFile))
	case kernel.MsgTypeSetProxyPeerInConfigFile:
		err = h.handleSetProxyPeerInConfigFile(request.(*kernel.MsgSetProxyPeerInConfigFile))
	case kernel.MsgTypeBringDeviceUp:
		err = h.handleBringDeviceUp()
	case kernel.MsgTypeIsDeviceAlive:
		err = h.handleIsDeviceAlive()
	default:
		err = fmt.Errorf("Invalid request message %v", int(msgType))
	}
	if err != nil {
		// Send error response
		respType = kernel.MsgTypeError
		resp = &kernel.MsgError{Error: err.Error()}
		err = nil
	}

	headerPlaceholder := [8]byte{}

	h.responseBuffer.Reset()
	h.responseBuffer.Write(headerPlaceholder[:])
	if resp != nil {
		if respType == kernel.MsgTypeNone {
			panic("Response type not populated")
		}
		if err := h.encoder.Encode(resp); err != nil {
			return fmt.Errorf("Response encoding failed: %v", err)
		}
	}
	if h.responseBuffer.Len() > kernel.MaxMsgSize {
		// Send an error response
		h.log.Errorf("Response too large (%v bytes)", h.responseBuffer.Len())
		h.responseBuffer.Reset()
		h.responseBuffer.Write(headerPlaceholder[:])
		respType = kernel.MsgTypeError
		if err := h.encoder.Encode(&kernel.MsgError{Error: "Response too large"}); err != nil {
			// This is not expected
			return fmt.Errorf("Double fault: %v", err)
		}
	}
	header := h.responseBuffer.Bytes()
	binary.LittleEndian.PutUint32(header[0:4], uint32(h.responseBuffer.Len()))
	binary.LittleEndian.PutUint32(header[4:8], uint32(respType))
	_, err = io.Copy(h.conn, &h.responseBuffer)
	if err != nil {
		return fmt.Errorf("Response sending failed: %v", err)
	}
	return nil
}

func handleConnection(conn net.Conn, log log.Log) {
	wg, err := wgctrl.New()
	if err != nil {
		log.Errorf("Error creating wgctrl: %v", err)
		return
	}
	defer wg.Close()

	h := &handler{
		conn: conn,
		log:  log,
		wg:   wg,
	}
	h.encoder = gob.NewEncoder(&h.responseBuffer)
	h.decoder = gob.NewDecoder(&h.requestBuffer)
	buf := [4096]byte{}
	for {
		n, err := conn.Read(buf[:])
		if err != nil {
			log.Errorf("conn.Read failed: %v", err)
			return
		}
		if Debug {
			log.Infof("Read %v bytes", n)
		}
		h.requestBuffer.Write(buf[:n])
		if h.requestBuffer.Len() >= 8 {
			// This little chunk of code will run over and over until len(raw) == expectedRawLen
			req := h.requestBuffer.Bytes()
			msgLen := int(binary.LittleEndian.Uint32(req[:4]))
			if msgLen > kernel.MaxMsgSize {
				log.Errorf("Request payload is too large (%v bytes)", msgLen)
				return
			}
			msgType := kernel.MsgType(binary.LittleEndian.Uint32(req[4:8]))
			if h.requestBuffer.Len() > msgLen {
				log.Errorf("Request is larger than specified (%v > %v)", h.requestBuffer.Len(), msgLen)
				return
			}
			if h.requestBuffer.Len() == msgLen {
				// consume our header, so that the GOB decoder can see only it's data
				dump := [8]byte{}
				h.requestBuffer.Read(dump[:])

				err = h.handleMessage(msgType, msgLen)
				if err != nil {
					return
				}
				h.requestBuffer.Reset()
			}
		}
	}
}

func verifyPermissions(logger log.Log) error {
	wg, err := wgctrl.New()
	if err != nil {
		return fmt.Errorf("Error creating wgctrl: %w", err)
	}
	defer wg.Close()

	// Sanity check
	//device, err := wg.Device("cyclops")
	//if err != nil {
	//	return fmt.Errorf("Error scanning Wireguard device: %v", err)
	//}
	//logger.Infof("Wireguard device public key: %v", device.PublicKey)

	devices, err := wg.Devices()
	if err != nil {
		return fmt.Errorf("Error scanning Wireguard devices: %v", err)
	}
	logger.Infof("Found %v active wireguard devices", len(devices))
	for _, d := range devices {
		logger.Infof("Wireguard device %v public key: %v", d.Name, d.PublicKey)
	}

	return nil
}

func main() {
	logger, err := log.NewLog()
	if err != nil {
		panic(err)
	}
	logger = log.NewPrefixLogger(logger, "kernelwg")

	if err := verifyPermissions(logger); err != nil {
		logger.Criticalf("%v", err)
		panic(err)
	}
	logger.Infof("Wireguard communication successful")

	listenAddr := "127.0.0.1:666"
	//listenAddr := net.UnixAddr{
	//	Net:  "unix",
	//	Name: kernel.UnixSocketName,
	//}

	logger.Infof("Listening on %v", listenAddr)
	//ln, err := net.ListenUnix("unix", &listenAddr)
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		logger.Errorf("Error listening: %v", err)
		os.Exit(1)
	}

	//ln.SetUnlinkOnClose(true)

	// Only connect to a single socket at a time
	for {
		conn, err := ln.Accept()
		if err != nil {
			logger.Errorf("Error accepting connection: %v", err)
		}
		logger.Infof("Accept connection from %v", conn.RemoteAddr().String())
		// Note that we do not do "go handleConnection", because our design is to be used by a single
		// client, in half-duplex mode (i.e. synchronous request/response).
		handleConnection(conn, logger)
	}
}
