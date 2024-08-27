package defs

import (
	"fmt"
)

// defs contains some definitions that are shared by all systems

type Resolution string // Resolution

const (
	ResHD Resolution = "HD" // High Definition (exact resolution unspecified - whatever the camera's main stream is set to)
	ResLD Resolution = "LD" // Low Definition (exact resolution unspecified - whatever the camera's sub stream is set to)
)

var AllResolutions = []Resolution{ResLD, ResHD}

func ParseResolution(res string) (Resolution, error) {
	switch res {
	case "ld":
		fallthrough
	case "LD":
		return ResLD, nil
	case "hd":
		fallthrough
	case "HD":
		return ResHD, nil
	}
	return "", fmt.Errorf("Unknown resolution '%v'. Valid values are 'LD' and 'HD'", res)
}
