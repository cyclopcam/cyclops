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

func (h *handler) handleCreateDevice() error {
	h.log.Infof("Creating Wireguard device %v", WireguardDeviceName)

	cmd := exec.Command("wg-quick", "up", WireguardDeviceName)
	err := cmd.Run()

	h.log.Infof("Device %v creation response: %v", WireguardDeviceName, err)
	return err
}

func (h *handler) handleIsDeviceAlive() error {
	_, err := h.wg.Device(WireguardDeviceName)
	if errors.Is(err, os.ErrNotExist) {
		return errors.New(kernel.ErrWireguardDeviceNotExist)
	}
	return err
}

func (h *handler) handleGetDevice() (any, error) {
	device, err := h.wg.Device(WireguardDeviceName)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, errors.New(kernel.ErrWireguardDeviceNotExist)
		}
		return nil, err
	}
	resp := kernel.MsgGetDeviceResponse{
		PublicKey:  device.PublicKey,
		ListenPort: device.ListenPort,
	}
	return &resp, nil
}

func (h *handler) handleGetPeers() (any, error) {
	device, err := h.wg.Device(WireguardDeviceName)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, errors.New(kernel.ErrWireguardDeviceNotExist)
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

func (h *handler) handleCreatePeers(request *kernel.MsgCreatePeers) error {
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

func (h *handler) handleMessage(msgType kernel.MsgType, msgLen int) error {
	if Debug {
		h.log.Infof("handleMessage %v, %v bytes", msgType, msgLen)
	}

	// Decode request, if any
	var request any
	switch msgType {
	case kernel.MsgTypeCreatePeers:
		request = &kernel.MsgCreatePeers{}
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
	case kernel.MsgTypeCreatePeers:
		err = h.handleCreatePeers(request.(*kernel.MsgCreatePeers))
	case kernel.MsgTypeCreateDevice:
		err = h.handleCreateDevice()
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

func main() {
	logger, err := log.NewLog()
	if err != nil {
		panic(err)
	}
	logger = log.NewPrefixLogger(logger, "kernelwg")
	//listenAddr := "127.0.0.1:666"
	listenAddr := net.UnixAddr{
		Net:  "unix",
		Name: kernel.UnixSocketName,
	}

	logger.Infof("Listening on %v", listenAddr)
	ln, err := net.ListenUnix("unix", &listenAddr)
	if err != nil {
		logger.Errorf("Error listening: %v", err)
		os.Exit(1)
	}

	ln.SetUnlinkOnClose(true)

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
