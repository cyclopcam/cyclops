package camera

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
)

type CameraBrands string

const (
	// SYNC-CAMERA-BRANDS
	CameraBrandUnknown      CameraBrands = ""
	CameraBrandHikVision    CameraBrands = "HikVision"
	CameraBrandReolink      CameraBrands = "Reolink"
	CameraBrandGenericRTSP  CameraBrands = "Generic RTSP"  // Used as a response from the port scanner to indicate that we can connect on RTSP, but we don't know anything else yet
	CameraBrandGenericONVIF CameraBrands = "Generic ONVIF" // Used as a response from OnvifGetDeviceInfo() to indicate a camera that supports ONVIF, but which we don't recognize
)

// AllCameraBrands is an array of all camera model names, excluding "Unknown"
var AllCameraBrands []CameraBrands

type CameraModelOutputParameters struct {
	LowResURL               string
	HighResURL              string
	PacketsAreAnnexBEncoded bool
}

// Should switch to onvif!
func GetCameraModelParameters(model, baseURL, lowResSuffix, highResSuffix string) (*CameraModelOutputParameters, error) {
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	out := &CameraModelOutputParameters{}
	switch CameraBrands(model) {
	case CameraBrandHikVision:
		out.HighResURL = baseURL + "Streaming/Channels/101"
		out.LowResURL = baseURL + "Streaming/Channels/102"
		out.PacketsAreAnnexBEncoded = true
	default:
		if lowResSuffix != "" && highResSuffix != "" {
			out.HighResURL = baseURL + highResSuffix
			out.LowResURL = baseURL + lowResSuffix
			// This is an unvalidated assumption.
			// Read the comment above Stream.cameraSendsAnnexBEncoded for more context,
			// and the justification for why we make this true by default.
			out.PacketsAreAnnexBEncoded = true
		} else {
			return nil, fmt.Errorf("Don't know how to find low and high resolution streams for Camera Model '%v' (connection details: %v)", model, baseURL)
		}
	}
	return out, nil
}

// Attempt to identify the camera from the HTTP response it sends when asked for it's root page (eg http://192.168.10.5)
func IdentifyCameraFromHTTP(headers http.Header, body string) CameraBrands {
	if headers.Get("Server") == "webserver" && strings.Contains(body, "去除edge下将数字处理成电话的错误") {
		return CameraBrandHikVision
	}
	if headers.Get("Server") == "App-webs/" && strings.Contains(body, "//使其IE窗口最大化") {
		return CameraBrandHikVision
	}
	if strings.Contains(body, "Reolink") {
		return CameraBrandReolink
	}
	return CameraBrandUnknown
}

func init() {
	// SYNC-CAMERA-BRANDS
	AllCameraBrands = []CameraBrands{
		CameraBrandHikVision,
		CameraBrandReolink,
		CameraBrandGenericRTSP,
		CameraBrandGenericONVIF,
	}
	sort.Slice(AllCameraBrands, func(i, j int) bool {
		return AllCameraBrands[i] < AllCameraBrands[j]
	})
}
