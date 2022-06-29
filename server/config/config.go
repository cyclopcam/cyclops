package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type Camera struct {
	Model            string `json:"model"`            // HikVision
	Name             string `json:"name"`             // Friendly name
	URL              string `json:"url"`              // RTSP url such as rtsp://user:password@192.168.1.33:554
	HighResURLSuffix string `json:"highResURLSuffix"` // eg Streaming/Channels/101 for HikVision. Can leave blank if Model is a known type.
	LowResURLSuffix  string `json:"lowResURLSuffix"`  // eg Streaming/Channels/102 for HikVision. Can leave blank if Model is a known type.
}

type Config struct {
	Cameras        []Camera `json:"cameras"`        // The cameras
	StoragePath    string   `json:"storagePath"`    // Path to video footage storage
	TempPath       string   `json:"tempPath"`       // Path for temporary files
	CameraBufferMB int      `json:"cameraBufferMB"` // Size in MB of each camera's high resolution ring buffer
}

func LoadConfig(filename string) (*Config, error) {
	if filename == "" {
		filename = "cyclops.json"
	}
	raw, err := os.ReadFile("cyclops.json")
	if err != nil {
		return nil, fmt.Errorf("Error loading %v: %w", filename, err)
	}
	cfg := &Config{}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("Error loading as JSON %v: %w", filename, err)
	}
	return cfg, nil
}
