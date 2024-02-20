package fsv

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cyclopcam/cyclops/pkg/videoformat/rf1"
)

// videoFile is a single logical video file, even if it's split into multiple physical files (eg rf1)
type videoFile struct {
	filename  string    // Base filename for rf1
	startTime time.Time // Start time of the video
	endTime   time.Time // End time of the video
}

// videoStream is a single logical video stream, usually split across many videoFiles
type videoStream struct {
	streamName string      // Name of the stream, eg "camera-0001"
	startTime  time.Time   // Start time of the stream (zero if unknown)
	endTime    time.Time   // End time of the stream (zero if unknown)
	files      []videoFile // Files in the stream
}

type Archive struct {
	baseDir string
	formats []VideoFormat
	streams map[string]*videoStream
}

// Open a directory of video files for reading and/or writing.
// The directory baseDir must exist, but it may be empty.
func Open(baseDir string, formats []VideoFormat) (*Archive, error) {
	if strings.HasSuffix(baseDir, "/") {
		baseDir = baseDir[:len(baseDir)-1]
	}
	// Scan top-level directories.
	// Each directory is a stream (eg camera-0001).
	archive := &Archive{
		baseDir: baseDir,
		formats: formats,
	}
	err := filepath.WalkDir(baseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			streamName := filepath.Base(path)
			archive.streams[streamName] = &videoStream{
				streamName: streamName,
			}
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Scan for all video files, so that we know the start and end time of each stream.
	if err := archive.scan(); err != nil {
		return nil, fmt.Errorf("Error scanning archive: %v", err)
	}

	return archive, nil
}

// scan all video files in the archive to figure out our start time and end time.
// We ignore gaps in the recording.
// In future, to find gaps, I plan on using the assumption that if contiguous files have
// start times that are less than X minutes apart, then there is no gap between them,
// and vice versa. X will be our max recording time per video file. For rf1, this
// has a hard limit of 1024 seconds, or just over 17 minutes.
// By using this assumption, we can find gaps by looking at the filenames alone,
// i.e. without having to read the files.
func (a *Archive) scan() error {
	for _, stream := range a.streams {
		// Scan all files in the stream
		streamDir := a.streamDir(stream.streamName)
		minStartTime := int64(1<<63 - 1)
		maxStartTime := int64(0)
		err := filepath.WalkDir(streamDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				// This is unexpected, but we'll ignore it.
				return filepath.SkipDir
			}
			// Check if this is a video file
			for _, format := range a.formats {
				if format.IsVideoFile(path) {
					startTimeUnixSecond, _, found := strings.Cut(path, "_")
					if found {
						t, err := strconv.ParseInt(startTimeUnixSecond, 10, 64)
						if err != nil {
							return fmt.Errorf("Invalid number in video file '%v'. Expected 12345_... format", path)
						}
						minStartTime = min(minStartTime, t)
						maxStartTime = max(maxStartTime, t)
					} else {
						return fmt.Errorf("Invalid video file '%v'. Expected 12345_... format", path)
					}
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
		if minStartTime < maxStartTime {
			stream.startTime = time.Unix(minStartTime, 0).UTC()
			stream.endTime = time.Unix(maxStartTime, 0).UTC()
		} else {
			// No video files found in this stream
			stream.startTime = time.Time{}
			stream.endTime = time.Time{}
		}
	}
	return nil
}

func (a *Archive) streamDir(streamName string) string {
	return filepath.Join(a.baseDir, streamName)
}

func (a *Archive) Write(streamName, trackName string, packets []rf1.NALU) {
}
