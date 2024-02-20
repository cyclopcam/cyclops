package configdb

import (
	"fmt"
	"os"
)

// Root system config
type ConfigJSON struct {
	Recording    RecordingJSON `json:"recording"`    // Default recording settings (can be overridden per camera)
	TempFilePath string        `json:"tempFilePath"` // Temporary file path
	ArcServer    string        `json:"arcServer"`    // Arc server URL
	ArcApiKey    string        `json:"arcApiKey"`    // Arc API key
}

// What causes us to record video
type RecordMode string

const (
	RecordModeAlways      RecordMode = "always"
	RecordModeOnMovement  RecordMode = "movement"
	RecordModeOnDetection RecordMode = "detection"
)

// Recording config
type RecordingJSON struct {
	Mode RecordMode `json:"mode,omitempty"`
	Path string     `json:"path,omitempty"`
}

// Returns an error if there is anything invalid about the config, or nil if everything is OK
func ValidateConfig(c *ConfigJSON) error {
	if err := ValidateRecordingConfig(true, &c.Recording); err != nil {
		return err
	}

	if err := os.MkdirAll(c.TempFilePath, 0770); err != nil {
		return fmt.Errorf("Invalid temporary file path '%v': %w", c.TempFilePath, err)
	}

	return nil
}

func ValidateRecordingConfig(isDefaults bool, c *RecordingJSON) error {
	if isDefaults && c.Mode == "" {
		return fmt.Errorf("Recording mode is required")
	}
	if c.Mode != "" && c.Mode != RecordModeAlways && c.Mode != RecordModeOnMovement && c.Mode != RecordModeOnDetection {
		return fmt.Errorf("Invalid recording mode '%v'. Valid modes are 'always', 'movement', and 'detection'", c.Mode)
	}
	if isDefaults && c.Path == "" {
		return fmt.Errorf("Recording path is required")
	}
	return nil
}
