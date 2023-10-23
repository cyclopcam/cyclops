package videox

// Transcode a video to make it easy for a low powered mobile browser to seek to random video positions
func TranscodeSeekable(srcFilename, dstFilename string) error {
	args := []string{
		"-i",
		srcFilename,
		"-y",   // overwrite output file
		"-g",   // keyframe interval
		"3",    // keyframe every 3 frames
		"-crf", // constant rate factor
		"25",   // 0-51, 0 is lossless, 51 is worst quality
		dstFilename,
	}
	_, err := RunAppCombinedOutput("ffmpeg", args)
	if err != nil {
		return err
	}
	return nil
}

// Transcode the high quality video stream into a slightly lower quality stream,
// with keyframes every 10 frames, and with noise reduction. This is for use on
// our training platform, where people need to be able to seek randomly inside
// a video.
func TranscodeMediumQualitySeekable(srcFilename, dstFilename string) error {
	args := []string{
		"-i",
		srcFilename,
		"-vf",
		"removegrain=1,hqdn3d=1.5:1.5:10:6,scale=1400:-1",
		"-y",   // overwrite output file
		"-g",   // keyframe interval
		"10",   // keyframe every 3 frames
		"-crf", // constant rate factor
		"25",   // 0-51, 0 is lossless, 51 is worst quality
		dstFilename,
	}
	_, err := RunAppCombinedOutput("ffmpeg", args)
	if err != nil {
		return err
	}
	return nil
}
