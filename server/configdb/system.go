package configdb

import (
	"fmt"
	"os"

	"github.com/cyclopcam/cyclops/pkg/dbh"
	"github.com/cyclopcam/cyclops/pkg/kibi"
	"github.com/cyclopcam/cyclops/server/util"
)

// Root system config
// SYNC-SYSTEM-CONFIG-JSON
type ConfigJSON struct {
	Recording    RecordingJSON `json:"recording"`    // Recording settings. We aim to make some settings overridable per-camera, such as recording mode.
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
// SYNC-SYSTEM-RECORDING-CONFIG-JSON
type RecordingJSON struct {
	Mode           RecordMode `json:"mode,omitempty"`
	Path           string     `json:"path,omitempty"`           // Root directory of fsv archive
	MaxStorageSize string     `json:"maxStorageSize,omitempty"` // Maximum storage with optional "gb", "mb", "tb" suffix. If no suffix, then bytes.
}

// Returns an error if there is anything invalid about the config, or nil if everything is OK
func ValidateConfig(c *ConfigJSON) error {
	if err := ValidateRecordingConfig(true, &c.Recording); err != nil {
		return err
	}

	if _, err := util.FindAnyTempFileDirectory(c.TempFilePath); err != nil {
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
	if c.Path != "" {
		if err := os.MkdirAll(c.Path, 0770); err != nil {
			return fmt.Errorf("Invalid recording path '%v': %w", c.Path, err)
		}
	}
	if c.MaxStorageSize != "" {
		if _, err := kibi.Parse(c.MaxStorageSize); err != nil {
			return fmt.Errorf("Invalid max storage size '%v': %w", c.MaxStorageSize, err)
		}
	}
	return nil
}

func (c *ConfigDB) GetConfig() ConfigJSON {
	c.configLock.Lock()
	defer c.configLock.Unlock()
	return c.config
}

// Return true if the system needs to be restarted for the config changes to take effect
func (c *ConfigDB) SetConfig(cfg ConfigJSON) (bool, error) {
	if err := ValidateConfig(&cfg); err != nil {
		return false, err
	}
	c.configLock.Lock()
	defer c.configLock.Unlock()
	needsRestart := false
	if c.config.Recording.Path != cfg.Recording.Path {
		needsRestart = true
	}
	c.config = cfg
	systemConfig := SystemConfig{
		Key:   "main",
		Value: dbh.MakeJSONField(cfg),
	}
	c.DB.Save(&systemConfig)

	c.Log.Infof("Config updated. Restart needed: %v", needsRestart)

	// TODO: apply hot config to all the sub-systems
	return needsRestart, nil
}
