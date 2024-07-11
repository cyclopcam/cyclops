package main

import (
	"fmt"
	"os"

	"github.com/cyclopcam/cyclops/pkg/nnmodule"
)

func main() {
	m, err := nnmodule.Load("modules/hailo/bin/libcyhailo.so")
	if err != nil {
		fmt.Printf("nnmodule.Load failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Hailo module loaded\n")

	setup := nnmodule.ModelSetup{
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
