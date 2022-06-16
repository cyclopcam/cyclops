package camera

import "strings"

func URLForHikVision(baseURL string, highRes bool) string {
	suffix := ""
	if highRes {
		suffix = "Streaming/Channels/101"
	} else {
		suffix = "Streaming/Channels/102"
	}
	if strings.HasSuffix(baseURL, "/") {
		return baseURL + suffix
	} else {
		return baseURL + "/" + suffix
	}
}
