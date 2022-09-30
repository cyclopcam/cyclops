package vpn

import (
	"errors"

	"github.com/bmharper/cyclops/pkg/log"
	"github.com/bmharper/cyclops/proxy/kernel"
)

// package vpn manages our Wireguard setup

type VPN struct {
	Log       log.Log
	PublicKey []byte
	client    *kernel.Client
}

func NewVPN(log log.Log) *VPN {
	return &VPN{
		Log:    log,
		client: kernel.NewClient(),
	}
}

// Connect to our Wireguard interface process
func (v *VPN) ConnectKernelWG() error {
	return v.client.Connect("127.0.0.1")
}

// Start our Wireguard device, and save our public key
func (v *VPN) Start() error {
	getResp, err := v.client.GetDevice()
	if err == nil {
		v.storeDeviceDetails(getResp)
		return nil
	} else if !errors.Is(err, kernel.ErrWireguardDeviceNotExist) {
		return err
	}

	if err := v.client.BringDeviceUp(); err != nil {
		return err
	}

	getResp, err = v.client.GetDevice()
	if err == nil {
		v.storeDeviceDetails(getResp)
	}
	return err
}

func (v *VPN) RegisterWithProxy() error {
	return nil
}

func (v *VPN) storeDeviceDetails(resp *kernel.MsgGetDeviceResponse) {
	v.PublicKey = resp.PublicKey[:]
}
