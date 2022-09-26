package kernel

import (
	"net"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// Our message format is (little endian):
// [0:4] uint32 Message Bytes (including 8 byte header)
// [4:8] uint32 MsgType
// [8:...] Payload
// Minimum message size is 8 bytes.
// Maximum message size is 1024 * 1024 * 1024.

// We make clones of the Wireguard wgtypes structs, because we use 'gob' package to encode,
// and gob does not allow nil pointers. In addition, we want to reduce the amount of IPC
// to a minimum, and also strip things like private keys.

const MaxMsgSize = 1024 * 1024 * 1024

// const UnixSocketName = "/var/opt/kernelwg"
const UnixSocketName = "@cyclops-wg"

// Well known error messages
const ErrWireguardDeviceNotExist = "Wireguard device does not exist"

type MsgType int

const (
	MsgTypeNone MsgType = iota
	MsgTypeError
	MsgTypeIsDeviceAlive
	MsgTypeGetDevice
	MsgTypeGetDeviceResponse
	MsgTypeGetPeers
	MsgTypeGetPeersResponse
	MsgTypeCreateDevice
	MsgTypeCreatePeers
)

type MsgError struct {
	Error string
}

type MsgGetDeviceResponse struct {
	PublicKey  wgtypes.Key
	ListenPort int
}

type MsgGetPeersResponse struct {
	Peers []Peer
}

type MsgCreatePeers struct {
	Peers []CreatePeer
}

// Device is a cut-down clone of wgtypes.Device
type Device struct {
	Name       string
	ListenPort int
	Peers      []Peer
}

// Peer is a cut-down clone of wgtypes.Peer
type Peer struct {
	PublicKey                   wgtypes.Key
	PersistentKeepaliveInterval time.Duration
	LastHandshakeTime           time.Time
	ReceiveBytes                int64
	TransmitBytes               int64
	AllowedIPs                  []net.IPNet
}

type CreatePeer struct {
	PublicKey wgtypes.Key
	AllowedIP net.IPNet
}
