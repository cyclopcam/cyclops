package proxy

import "time"

// BaseModel is our base class for a GORM model.
// The default GORM Model uses int, but we prefer int64
type BaseModel struct {
	ID int64 `gorm:"primaryKey"`
}

// A Cyclops server
type Server struct {
	BaseModel
	PublicKey []byte    // Wireguard public key of this server
	VpnIP     string    // IP address inside Wireguard VPN (eg 10.7.0.0)
	CreatedAt time.Time // Time when server record was created
}

// List of available VPN IP addresses
// Whenever we remove a peer, we add it's IP to this list.
// When we add a new peer, we take the first item from the list.
// If this list is empty when adding a new peer, then we populate it with
// a range of free addresses.
type IPFreePool struct {
	VpnIP string // IP address inside Wireguard VPN (eg 10.7.0.0)
}
