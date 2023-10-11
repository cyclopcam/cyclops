package videox

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/cyclopcam/cyclops/pkg/rando"
)

// Extract the duration of a video file
func ExtractVideoDuration(srcFilename string) (time.Duration, error) {
	args := []string{
		"ffprobe",
		"-v",
		"error",
		"-show_entries",
		"format=duration",
		"-of",
		"default=noprint_wrappers=1:nokey=1",
		srcFilename,
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		return 0, fmt.Errorf("Unable to find ffprobe in your path (%w)", err)
	}
	cmd := &exec.Cmd{
		Path: ffprobe,
		Args: args,
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		outStr := ""
		if out != nil {
			outStr = string(out)
		}
		return 0, fmt.Errorf("ffprobe execution failed: %w (%v)", err, outStr)
	}
	// I don't know why, but the full output on my machine is two lines:
	//   Warning: using insecure memory!
	//   6.399000
	// So we make allowance for that here.
	outStr := string(out)
	for _, line := range strings.Split(outStr, "\n") {
		if seconds, err := strconv.ParseFloat(line, 64); err == nil {
			return time.Duration(seconds * float64(time.Second)), nil
		}
	}
	return 0, fmt.Errorf("Unable to parse ffprobe output: %v", outStr)
}

// Extract a single frame from a video file and return the JPEG bytes
func ExtractFrame(srcFilename string, atSecond float64) ([]byte, error) {
	tmpFilename := rando.TempFilename(".jpg")
	defer os.Remove(tmpFilename)
	args := []string{
		"ffmpeg",
		"-ss",
		fmt.Sprintf("%.3f", atSecond),
		"-i",
		srcFilename,
		"-frames:v",
		"1",
		"-q:v",
		"8",
		tmpFilename,
	}
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, fmt.Errorf("Unable to find ffmpeg in your path (%w)", err)
	}
	cmd := &exec.Cmd{
		Path: ffmpeg,
		Args: args,
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		outStr := ""
		if out != nil {
			outStr = string(out)
		}
		return nil, fmt.Errorf("ffmpeg execution failed: %w (%v)", err, outStr)
	}
	return os.ReadFile(tmpFilename)
}
