package configdb

import (
	"fmt"
	"os"
	"time"

	"github.com/cyclopcam/cyclops/pkg/kibi"
	"github.com/cyclopcam/cyclops/server/util"
	"github.com/cyclopcam/dbh"
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
	Mode              RecordMode `json:"mode,omitempty"`
	Path              string     `json:"path,omitempty"`              // Root directory of fsv archive
	MaxStorageSize    string     `json:"maxStorageSize,omitempty"`    // Maximum storage with optional "gb", "mb", "tb" suffix. If no suffix, then bytes.
	RecordBeforeEvent int        `json:"recordBeforeEvent,omitempty"` // Record this many seconds before an event
	RecordAfterEvent  int        `json:"recordAfterEvent,omitempty"`  // Record this many seconds after an event
}

func (r *RecordingJSON) RecordBeforeEventDuration() time.Duration {
	if r.RecordBeforeEvent <= 0 {
		return 15 * time.Second
	}
	return time.Duration(r.RecordBeforeEvent) * time.Second
}

func (r *RecordingJSON) RecordAfterEventDuration() time.Duration {
	if r.RecordAfterEvent <= 0 {
		return 15 * time.Second
	}
	return time.Duration(r.RecordAfterEvent) * time.Second
}

// Holding off on this for now. I'd rather have it in code, until it's obvious that it
// belongs on config.
// AI config
//type AIConfigJSON struct {
//	// Used to map from an NN class to another class, which could be a more abstract class.
//	// eg {"car": "vehicle", "truck": "vehicle"}
//	// We should really be training our own NN that does this kind of mapping, but this
//	// is a reasonable solution until we get there.
//	RemapClasses map[string]string `json:"remapClasses,omitempty"`
//}

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
		if _, err := kibi.ParseBytes(c.MaxStorageSize); err != nil {
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
	needsRestart := RestartNeeded(&c.config, &cfg)
	c.config = cfg
	systemConfig := SystemConfig{
		Key:   "main",
		Value: dbh.MakeJSONField(cfg),
	}
	c.DB.Save(&systemConfig)

	c.Log.Infof("Config updated. Restart needed: %v", needsRestart)

	// TODO: apply hot config to all the sub-systems that don't poll the
	// config periodically.

	return needsRestart, nil
}
