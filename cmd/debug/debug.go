package main

import (
	"fmt"

	"github.com/bmharper/cyclops/server/scanner"
)

func main() {
	cams, err := scanner.ScanForLocalCameras()
	if err != nil {
		panic(err)
	}
	for _, c := range cams {
		fmt.Printf("Camera: %v\n", c)
	}
}
