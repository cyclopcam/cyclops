package main

import (
	"fmt"

	"github.com/cyclopcam/cyclops/server/scanner"
)

func main() {
	cams, err := scanner.ScanForLocalCameras(nil)
	if err != nil {
		panic(err)
	}
	for _, c := range cams {
		fmt.Printf("Camera: %v\n", c)
	}
}
