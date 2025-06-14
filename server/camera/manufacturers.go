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
	CameraBrandTPLink       CameraBrands = "TP-Link"
	CameraBrandGenericRTSP  CameraBrands = "Generic RTSP"  // Used as a response from the port scanner to indicate that we can connect on RTSP, but we don't know anything else yet
	CameraBrandGenericONVIF CameraBrands = "Generic ONVIF" // Used as a response from OnvifGetDeviceInfo() to indicate a camera that supports ONVIF, but which we don't recognize
)

// AllCameraBrands is an array of all camera model names, excluding "Unknown"
var AllCameraBrands []CameraBrands

type CameraRTSPInfo struct {
	LowResURL               string
	HighResURL              string
	PacketsAreAnnexBEncoded bool // I suspect this is always true
}

func cameraBrandDefaults(brand CameraBrands, info *CameraRTSPInfo) {
	// This is an unvalidated assumption.
	// Read the comment above Stream.cameraSendsAnnexBEncoded for more context,
	// and the justification for why we make this true by default.
	info.PacketsAreAnnexBEncoded = true

	switch brand {
	case CameraBrandHikVision:
		info.HighResURL = "Streaming/Channels/101"
		info.LowResURL = "Streaming/Channels/102"
	case CameraBrandReolink:
		info.HighResURL = "/"
		info.LowResURL = "h264Preview_01_sub"
		info.PacketsAreAnnexBEncoded = true // untested
	case CameraBrandTPLink:
		info.HighResURL = "stream1"
		info.LowResURL = "stream2"
	}
}

func GetCameraRTSP(brand CameraBrands, host, username, password string, port int, lowResSuffix, highResSuffix string) (*CameraRTSPInfo, error) {
	baseURL := "rtsp://" + username + ":" + password + "@" + host
	if port == 0 {
		baseURL += ":554"
	} else {
		baseURL += fmt.Sprintf(":%v", port)
	}

	out := &CameraRTSPInfo{}
	cameraBrandDefaults(brand, out)
	// At this stage out.LowResURL and out.HighResURL and just the path (eg "Streaming/Channels/101")
	if lowResSuffix != "" {
		out.LowResURL = lowResSuffix
	}
	if highResSuffix != "" {
		out.HighResURL = highResSuffix
	}
	if out.LowResURL == "" {
		return nil, fmt.Errorf("Can't find low resolution stream for Camera Model '%v'", brand)
	}
	if out.HighResURL == "" {
		return nil, fmt.Errorf("Can't find high resolution stream for Camera Model '%v'", brand)
	}
	out.LowResURL = baseURL + "/" + strings.TrimPrefix(out.LowResURL, "/")
	out.HighResURL = baseURL + "/" + strings.TrimPrefix(out.HighResURL, "/")
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
		CameraBrandTPLink,
		CameraBrandGenericRTSP,
		CameraBrandGenericONVIF,
	}
	sort.Slice(AllCameraBrands, func(i, j int) bool {
		return AllCameraBrands[i] < AllCameraBrands[j]
	})
}
