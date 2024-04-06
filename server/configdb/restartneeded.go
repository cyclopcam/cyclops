package configdb

func RestartNeeded(c1, c2 *ConfigJSON) bool {
	if c1.Recording.Path != c2.Recording.Path {
		return true
	}
	if c1.TempFilePath != c2.TempFilePath {
		return true
	}
	if c1.ArcServer != c2.ArcServer {
		return true
	}
	if c1.ArcApiKey != c2.ArcApiKey {
		return true
	}
	return false
}
