package configdb

import (
	"fmt"
	"os"
	"path/filepath"
)

// VariableKey is global configuration variable that can be set on the system
type VariableKey string

// VariableDef defines a system variable.
// This is used to drive the configuration UI.
type VariableDef struct {
	Key         VariableKey `json:"key"`
	Title       string      `json:"title"`       // eg "Permanent Storage Path"
	Explanation string      `json:"explanation"` // eg "Recordings that you want to keep forever are stored on Permanent Storage..."
	Required    bool        `json:"required"`    // True if the variable must be set before the system can run
	UIGroup     string      `json:"uiGroup"`     // Used to group UI elements together
	Type        string      `json:"type"`        // One of ["path", "text"]
}

var AllVariablesByKey map[VariableKey]*VariableDef
var AllVariables []VariableDef

const (
	// SYNC-ALL-VARIABLES
	VarPermanentStoragePath   VariableKey = "PermanentStoragePath"
	VarRecentEventStoragePath VariableKey = "RecentEventStoragePath"
	VarTempFilePath           VariableKey = "TempFilePath"
	// The following 3 are here because I haven't figured out the authentication strategy yet.
	// How does a person authenticate from their home cyclops server to an Arc server?
	// It feels like it should be some kind of OAuth thing.
	VarArcServer   VariableKey = "ArcServer"
	VarArcUsername VariableKey = "ArcUsername"
	VarArcPassword VariableKey = "ArcPassword"
)

// If true, then the system must be restarted after setting this variable
func VariableSetNeedsRestart(v VariableKey) bool {
	return true
}

func ValidateVariable(v VariableKey, value string) error {
	switch v {
	case VarPermanentStoragePath:
		fallthrough
	case VarRecentEventStoragePath:
		fallthrough
	case VarTempFilePath:
		if len(value) < 1 {
			return fmt.Errorf("Invalid directory name: '%v'", value)
		}
		if !filepath.IsAbs(value) {
			return fmt.Errorf("The path '%v' does not start with a slash. You must use an absolute path such as /home/ubuntu/storage", value)
		}
		st, err := os.Stat(value)
		if err == nil && !st.IsDir() {
			// Not sure if this will cause false positives with some kind of symlinks.
			// As far as I can tell, it's fine on linux for a symbolic link.
			return fmt.Errorf("A file named '%v' already exists. The path must be a directory (it can be empty or non existent).", value)
		}
	}
	return nil
}

// Guess a default variable value, or return an empty string if we can't make a good guess
func GuessDefaultVariableValue(v VariableKey) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	switch v {
	case VarPermanentStoragePath:
		return filepath.Join(home, "cyclops", "permanent")
	case VarRecentEventStoragePath:
		return filepath.Join(home, "cyclops", "recent")
	case VarTempFilePath:
		return filepath.Join(home, "cyclops", "temp")
	}
	return ""
}

// Guess reasonable defaults for mandatory system variables that are not set
func (c *ConfigDB) GuessDefaultVariables() error {
	// Read variables in DB
	indb := []Variable{}
	indbByKey := map[string]*Variable{}
	if err := c.DB.Find(&indb).Error; err != nil {
		return err
	}
	for i := range indb {
		indbByKey[indb[i].Key] = &indb[i]
	}

	db, err := c.DB.DB()
	if err != nil {
		return err
	}

	for _, v := range AllVariables {
		existing := indbByKey[string(v.Key)]
		if existing == nil || existing.Value == "" {
			guessed := GuessDefaultVariableValue(v.Key)
			if guessed != "" {
				c.Log.Infof("Setting variable %v to guessed value %v", v.Key, guessed)
				_, err = db.Exec("INSERT INTO variable (key, value) VALUES ($1, $2) ON CONFLICT(key) DO UPDATE SET value = EXCLUDED.value", v.Key, guessed)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func init() {
	AllVariables = []VariableDef{
		{
			Key:   VarPermanentStoragePath,
			Title: "Permanent Storage Path",
			Explanation: "Video clips that are used to train the system are stored in permanent storage. " +
				"This folder should be backed up.",
			Required: true,
			Type:     "path",
			UIGroup:  "01.01",
		},
		{
			Key:   VarRecentEventStoragePath,
			Title: "Recent Events Storage Path",
			Explanation: "These are recent video clips which may be of interest. Old clips are automatically " +
				"erased when we run out of space. If you boot from an SD card (eg Raspberry Pi), then this path should be located " +
				"on removable storage (like a USB thumb drive), to avoid wearing out the flash memory on the boot device.",
			Required: true,
			Type:     "path",
			UIGroup:  "01.02",
		},
		{
			Key:   VarTempFilePath,
			Title: "Temporary File Path",
			Explanation: "This is temporary space used while encoding videos. This space is seldom actually " +
				"written to, and merely serves as a fallback when RAM is low.",
			Required: true,
			Type:     "path",
			UIGroup:  "01.03",
		},
		{
			Key:         VarArcServer,
			Title:       "Arc Server (for custom training)",
			Explanation: "This server is used for gather training videos, so that we can create better neural networks.",
			Required:    false,
			Type:        "text",
			UIGroup:     "02.01",
		},
		{
			Key:         VarArcUsername,
			Title:       "Arc Server username",
			Explanation: "Username for logging into the Arc server.",
			Required:    false,
			Type:        "text",
			UIGroup:     "02.02",
		},
		{
			Key:         VarArcPassword,
			Title:       "Arc Server password",
			Explanation: "Password for logging into the Arc server.",
			Required:    false,
			Type:        "password",
			UIGroup:     "02.03",
		},
	}
	AllVariablesByKey = map[VariableKey]*VariableDef{}

	for i := range AllVariables {
		AllVariablesByKey[AllVariables[i].Key] = &AllVariables[i]
	}
}
