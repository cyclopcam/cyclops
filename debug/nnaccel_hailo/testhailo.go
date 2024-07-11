package main

import (
	"fmt"
	"os"

	"github.com/cyclopcam/cyclops/pkg/nnaccel"
)

// To build C++ and run:
// cd nnaccel/hailo && ./build && cd ../.. && go run debug/nnaccel_hailo/testhailo.go

func main() {
	m, err := nnaccel.Load("hailo")
	if err != nil {
		fmt.Printf("nnaccel.Load failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Hailo module loaded\n")

	setup := nnaccel.ModelSetup{
		BatchSize: 1,
	}
	model, err := m.LoadModel("models/hailo/8L/yolov8s.hef", &setup)
	if err != nil {
		fmt.Printf("m.LoadModel failed: %v\n", err)
		os.Exit(1)
	}
	model.Close()

	fmt.Printf("Done\n")
}
