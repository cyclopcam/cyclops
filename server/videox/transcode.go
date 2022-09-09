package videox

import (
	"fmt"
	"os/exec"
)

// Transcode a video to make it easy for a low powered mobile browser to seek to random video positions
func TranscodeSeekable(srcFilename, dstFilename string) error {
	args := []string{
		"ffmpeg",
		"-i",
		srcFilename,
		"-y",
		"-g",
		"3",
		"-crf",
		"25",
		dstFilename,
	}
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		return fmt.Errorf("Unable to find ffmpeg in your path (%w)", err)
	}
	//fmt.Printf("\n%v %v\n", ffmpeg, strings.Join(args, " "))
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
		return fmt.Errorf("ffmpeg execution failed: %w (%v)", err, outStr)
	}
	return nil
}