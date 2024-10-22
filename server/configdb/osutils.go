package configdb

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/cyclopcam/cyclops/pkg/shell"
)

var digitRegex = regexp.MustCompile(`\d+`)

// Measure the space available at the given path, or the first parent directory that exists.
// Returns the amount of space available in bytes.
func MeasureDiscSpaceAvailable(path string) (int64, error) {
	availB := int64(0)

	// Keep walking up the directory tree until we can find the free space.
	// This is useful because often the path specified won't exist yet, but
	// it will be rooted in a valid directory, somewhere higher up.
	availPath := path
	for {
		// On linux
		// df -B1 --output=avail /path
		// Example output:
		//        Avail
		// 815667085312
		res, err := shell.Run("df", "-B1", "--output=avail", availPath)
		if err != nil {
			if strings.Contains(err.Error(), "No such file or directory") {
				prev := availPath
				availPath = filepath.Dir(availPath)
				if availPath == "\\" || availPath == "/" || availPath == "." || availPath == prev {
					return 0, fmt.Errorf("Invalid path: %v", availPath)
				} else {
					continue
				}
			}
			return 0, fmt.Errorf("Failed to read space available: %v", err)
		}
		availStr := digitRegex.FindString(res)
		availB, _ = strconv.ParseInt(availStr, 10, 64)
		return availB, nil
	}
}
