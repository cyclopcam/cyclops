package camera

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/use-go/onvif"
	onvifDevice "github.com/use-go/onvif/device"
	onvifMedia "github.com/use-go/onvif/media"
	sdkDevice "github.com/use-go/onvif/sdk/device"
	sdkMedia "github.com/use-go/onvif/sdk/media"
	xsdOnvif "github.com/use-go/onvif/xsd/onvif"
)

// Enable this to get verbose printf logs when using ONVIF
const onvifVerboseEnable = true

// Whatever we have discovered about the camera via ONVIF
type OnvifDeviceInfo struct {
	Brand         CameraBrands
	Model         string
	Firmware      string
	Serial        string
	MainStreamURL string
	SubStreamURL  string
}

func onvifVerbose(format string, v ...any) {
	if onvifVerboseEnable {
		fmt.Printf(format, v...)
	}
}

// Use ONVIF to discover whatever we need to know about the device
func OnvifGetDeviceInfo(host, username, password string) (*OnvifDeviceInfo, error) {
	// Connect to the camera
	dev, err := onvif.NewDevice(onvif.DeviceParams{
		Xaddr:    host,
		Username: username,
		Password: password,
	})
	if err != nil {
		onvifVerbose("Error connecting to device: %v\n", err)
		return nil, err
	}

	result := &OnvifDeviceInfo{}

	// The GetDeviceInfo() API is not implemented in the use-go/onvif library
	//deviceInfo := dev.GetDeviceInfo()

	devInfo, err := sdkDevice.Call_GetDeviceInformation(context.Background(), dev, onvifDevice.GetDeviceInformation{})
	if err != nil {
		onvifVerbose("Error fetching device info: %v\n", err)
		return nil, err
	} else {
		switch strings.ToUpper(devInfo.Manufacturer) {
		case "REOLINK":
			result.Brand = CameraBrandReolink
		case "HIKVISION":
			result.Brand = CameraBrandHikVision
		default:
			result.Brand = CameraBrandGenericONVIF
		}
		result.Model = devInfo.Model
		result.Firmware = devInfo.FirmwareVersion
		result.Serial = devInfo.SerialNumber
		if onvifVerboseEnable {
			onvifVerbose("Device info: %v\n", devInfo)
			onvifVerbose("Manufacturer: %v\n", devInfo.Manufacturer)
			onvifVerbose("Model: %v\n", devInfo.Model)
			onvifVerbose("Firmware Version: %v\n", devInfo.FirmwareVersion)
			onvifVerbose("Serial Number: %v\n", devInfo.SerialNumber)
			onvifVerbose("Hardware ID: %v\n", devInfo.HardwareId)
		}
	}

	if onvifVerboseEnable {
		for k, v := range dev.GetServices() {
			onvifVerbose("Service Key: %s, Value: %s\n", k, v)
		}
	}

	resp, err := sdkMedia.Call_GetProfiles(context.Background(), dev, onvifMedia.GetProfiles{})
	if err != nil {
		return nil, err
	}
	//check(err)

	onvifVerbose("\n")

	for _, profile := range resp.Profiles {
		name := strings.ToUpper(string(profile.Name))
		isMain := strings.Index(name, "MAIN") != -1
		isSub := strings.Index(name, "SUB") != -1
		if !isMain && !isSub {
			continue
		}
		onvifVerbose("Profile: %v, %v\n", profile.Name, profile.Token)
		streamRequest := onvifMedia.GetStreamUri{
			// This StreamSetup part is necessary for Reolink cameras.
			StreamSetup: xsdOnvif.StreamSetup{
				Stream: "RTP-Unicast",
				Transport: xsdOnvif.Transport{
					Protocol: "RTSP",
				},
			},
			ProfileToken: profile.Token,
		}
		r, err := sdkMedia.Call_GetStreamUri(context.Background(), dev, streamRequest)
		if err != nil {
			return nil, err
		}
		//check(err)
		onvifVerbose("Stream URI: %v\n", r)
		onvifVerbose("\n")
		streamUri := string(r.MediaUri.Uri)
		u, err := url.Parse(streamUri)
		if err != nil {
			return nil, err
		}
		path := u.Path
		if u.RawQuery != "" {
			path += "?" + u.RawQuery
		}
		if len(path) > 1 {
			// Remove leading slash
			path = path[1:]
		}
		if isMain {
			result.MainStreamURL = path
		} else if isSub {
			result.SubStreamURL = path
		}
	}

	return result, nil
}
