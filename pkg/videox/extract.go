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
		"-v",
		"error",
		"-show_entries",
		"format=duration",
		"-of",
		"default=noprint_wrappers=1:nokey=1",
		srcFilename,
	}
	out, err := RunAppCombinedOutput("ffprobe", args)
	if err != nil {
		return 0, err
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
// If outputWidth is zero, then we use the same width as the input video
func ExtractFrame(srcFilename string, atSecond float64, outputWidth int) ([]byte, error) {
	tmpFilename := rando.TempFilename(".jpg")
	defer os.Remove(tmpFilename)
	args := []string{
		"-ss",
		fmt.Sprintf("%.3f", atSecond),
		"-i",
		srcFilename,
	}
	if outputWidth > 0 {
		args = append(args,
			"-vf",
			fmt.Sprintf("scale=%v:-1", outputWidth),
		)
	}
	args = append(args,
		"-frames:v",
		"1",
		"-q:v",
		"8",
		tmpFilename,
	)
	_, err := RunAppCombinedOutput("ffmpeg", args)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(tmpFilename)
}

// app_name is an executable, such as "ffmpeg" or "ffprobe"
// args must not include the executable name as the first parameter
// Returns the string output from exec.Cmd's "CombinedOutput" method.
func RunAppCombinedOutput(app_name string, args []string) ([]byte, error) {
	app_path, err := exec.LookPath(app_name)
	if err != nil {
		return nil, fmt.Errorf("Unable to find '%v' in your path (%w)", app_name, err)
	}
	args_with_app := append([]string{app_name}, args...)
	cmd := &exec.Cmd{
		Path: app_path,
		Args: args_with_app,
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		outStr := ""
		if out != nil {
			outStr = string(out)
		}
		return nil, fmt.Errorf("%v execution failed: %w (%v)", app_name, err, outStr)
	}
	return out, nil
}
