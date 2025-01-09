package camera

import (
	"context"
	"fmt"
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
type DeviceInfo struct {
	Model         CameraModels
	MainStreamURL string
	SubStreamURL  string
}

func onvifVerbose(format string, v ...any) {
	if onvifVerboseEnable {
		fmt.Printf(format, v...)
	}
}

// Use ONVIF to discover whatever we need to know about the device
func OnvifGetDeviceInfo(host, username, password string) (*DeviceInfo, error) {
	// Connect to the camera
	//deviceEndpoint := fmt.Sprintf("%v", host)
	dev, err := onvif.NewDevice(onvif.DeviceParams{
		//Xaddr:    deviceEndpoint,
		Xaddr:    host,
		Username: username,
		Password: password,
	})
	if err != nil {
		onvifVerbose("Error connecting to device: %v\n", err)
		return nil, err
	}

	result := &DeviceInfo{}

	// The GetDeviceInfo() API is not implemented in the use-go/onvif library
	//deviceInfo := dev.GetDeviceInfo()

	devInfo, err := sdkDevice.Call_GetDeviceInformation(context.Background(), dev, onvifDevice.GetDeviceInformation{})
	if err != nil {
		onvifVerbose("Error fetching device info: %v\n", err)
		return nil, err
	} else {
		switch strings.ToUpper(devInfo.Manufacturer) {
		case "REOLINK":
			result.Model = CameraModelReolink
		case "HIKVISION":
			result.Model = CameraModelHikVision
		default:
			result.Model = CameraModelGenericONVIF
		}
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
		if isMain {
			result.MainStreamURL = string(r.MediaUri.Uri)
		} else if isSub {
			result.SubStreamURL = string(r.MediaUri.Uri)
		}
	}

	return result, nil
}
