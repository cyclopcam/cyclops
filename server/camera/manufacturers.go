package camera

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
)

type CameraModels string

const (
	// SYNC-CAMERA-MODELS
	CameraModelUnknown     CameraModels = ""
	CameraModelHikVision   CameraModels = "HikVision"
	CameraModelJustTesting CameraModels = "JustTesting"
)

// AllCameraModels is an array of all camera model names, excluding "Unknown"
var AllCameraModels []CameraModels

func URLForCamera(model, baseURL, lowResSuffix, highResSuffix string, highRes bool) (string, error) {
	suffix := ""
	if highRes {
		suffix = highResSuffix
		if suffix == "" {
			switch CameraModels(model) {
			case CameraModelHikVision:
				suffix = "Streaming/Channels/101"
			}
		}
	} else {
		suffix = lowResSuffix
		if suffix == "" {
			switch CameraModels(model) {
			case CameraModelHikVision:
				suffix = "Streaming/Channels/102"
			}
		}
	}
	if suffix == "" {
		return "", fmt.Errorf("Don't know how to find low and high res streams from %v (model '%v')", baseURL, model)
	}
	if strings.HasSuffix(baseURL, "/") {
		return baseURL + suffix, nil
	} else {
		return baseURL + "/" + suffix, nil
	}
}

// Attempt to identify the camera from the HTTP response it sends when asked for it's root page (eg http://192.168.10.5)
func IdentifyCameraFromHTTP(headers http.Header, body string) CameraModels {
	if headers.Get("Server") == "webserver" && strings.Contains(body, "去除edge下将数字处理成电话的错误") {
		return CameraModelHikVision
	}
	if headers.Get("Server") == "App-webs/" && strings.Contains(body, "//使其IE窗口最大化") {
		return CameraModelHikVision
	}
	return CameraModelUnknown
}

func init() {
	// SYNC-CAMERA-MODELS
	AllCameraModels = []CameraModels{
		CameraModelHikVision,
		CameraModelJustTesting,
	}
	sort.Slice(AllCameraModels, func(i, j int) bool {
		return AllCameraModels[i] < AllCameraModels[j]
	})
}
