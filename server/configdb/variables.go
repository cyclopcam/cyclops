package configdb

// VariableKey is global configuration variables that can be set on the system
type VariableKey string

const (
	VarPermanentStoragePath   VariableKey = "PermanentStoragePath"
	VarRecentEventStoragePath VariableKey = "RecentEventStoragePath"
	VarTempFilePath           VariableKey = "TempFilePath"
)

// If true, then the system must be restarted after setting this variable
func VariableSetNeedsRestart(v VariableKey) bool {
	return true
}
