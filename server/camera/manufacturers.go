package camera

import (
	"fmt"
	"strings"
)

const ModelHikVision = "HikVision"

func URLForCamera(model, baseURL, lowResSuffix, highResSuffix string, highRes bool) (string, error) {
	suffix := ""
	if highRes {
		suffix = highResSuffix
		if suffix == "" {
			switch model {
			case ModelHikVision:
				suffix = "Streaming/Channels/101"
			}
		}
	} else {
		suffix = lowResSuffix
		if suffix == "" {
			switch model {
			case ModelHikVision:
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
