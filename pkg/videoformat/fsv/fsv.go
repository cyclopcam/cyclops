package fsv

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cyclopcam/cyclops/pkg/log"
	"github.com/cyclopcam/cyclops/pkg/videoformat/rf1"
)

// videoFile is a single logical video file, even if it's split into multiple physical files (eg rf1)
// A videoFile will typically have one or two tracks inside (video, or video + audio).
type videoFile struct {
	filename  string    // Example /var/lib/cyclops/archive/camera-0001/1708584695
	startTime time.Time // Start time of the video file
	endTime   time.Time // End time of the video file
	file      VideoFile // Interface to the on-disk format. This is an open handle to the video file, and needs to be Closed() when we're done with it.
}

// A small memory-footprint record that exists for every file in the archive
type videoFileIndex struct {
	filename  string // Only the logical filename, such as "1708584695"
	startTime int64  // Milliseconds UTC
}

// videoStream is a single logical video stream, usually split across many videoFiles
type videoStream struct {
	name   string      // Name of the stream, eg "camera-0001"
	format VideoFormat // Format of the video files

	// contentLock guards all access to all the members below
	contentLock sync.Mutex
	startTime   time.Time        // Start time of the stream (zero if unknown)
	endTime     time.Time        // End time of the stream (zero if unknown)
	current     *videoFile       // The file we are currently writing to
	files       []videoFileIndex // All files in the stream except for 'current'
}

// Information about a stream
type StreamInfo struct {
	Name      string
	StartTime time.Time
	EndTime   time.Time
}

// Track payload used when writing packets.
// Every write call has enough information to create a new track, if necessary.
// This allows us to have a stateless Write API.
type TrackPayload struct {
	TrackType   rf1.TrackType
	Codec       string // For audio/video tracks
	VideoWidth  int    // For video tracks
	VideoHeight int    // For video tracks
	NALUs       []rf1.NALU
}

// Archive is a collection of zero or more video streams,
// rooted at the same base directory. Every sub-directory from the base holds
// the videos of one stream. The stream name is the directory name.
// Archive is not safe for use from multiple threads.
type Archive struct {
	log                  log.Log
	baseDir              string
	formats              []VideoFormat
	maxVideoFileDuration time.Duration // We need to know this so that it is fast to find files close to a given time period.
	maxBytesPerRead      int           // Maximum number of bytes that we will return from a single Read()

	streamsLock sync.Mutex // Guards access to the streams map
	streams     map[string]*videoStream
}

// Open a directory of video files for reading and/or writing.
// The directory baseDir must exist, but it may be empty.
// When creating new streams, formats[0] is used, so the ordering
// of formats is important.
func Open(logger log.Log, baseDir string, formats []VideoFormat) (*Archive, error) {
	if len(formats) == 0 {
		return nil, fmt.Errorf("No video formats provided")
	}

	// This must be lower or equal to the max duration of the file formats that we support.
	// rf1 has a max duration of 1024 seconds, so that's why we choose 1000, because it's
	// less than 1024, with a bit of margin.
	// You can't arbitrarily change this constant after creating an archive, because then when
	// you scan for video files matching a given time period, you may skip a valid file.
	// The reason is because we round down to the previous period, and then use a filesystem glob
	// to find files.
	maxVideoFileDuration := 1000 * time.Second

	// This is an arbitrary limit intended to reduce the chance of an accidental out of memory situation
	maxBytesPerRead := 256 * 1024 * 1024

	baseDir = strings.TrimSuffix(baseDir, "/")
	// Scan top-level directories.
	// Each directory is a stream (eg camera-0001).
	archive := &Archive{
		log:                  logger,
		baseDir:              baseDir,
		formats:              formats,
		streams:              map[string]*videoStream{},
		maxVideoFileDuration: maxVideoFileDuration,
		maxBytesPerRead:      maxBytesPerRead,
	}
	err := filepath.WalkDir(baseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && path != baseDir {
			streamName := filepath.Base(path)
			archive.streams[streamName] = &videoStream{
				name: streamName,
			}
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Scan for all video files, so that we know the start and end time of each stream,
	// and the name of every file.
	if err := archive.scan(); err != nil {
		return nil, fmt.Errorf("Error scanning archive: %v", err)
	}

	return archive, nil
}

func (a *Archive) MaxVideoFileDuration() time.Duration {
	return a.maxVideoFileDuration
}

// Get a list of all streams in the archive, and some metadata about each stream.
func (a *Archive) ListStreams() []*StreamInfo {
	a.streamsLock.Lock()
	defer a.streamsLock.Unlock()
	streams := make([]*StreamInfo, 0, len(a.streams))
	for _, stream := range a.streams {
		streams = append(streams, &StreamInfo{
			Name:      stream.name,
			StartTime: stream.startTime,
			EndTime:   stream.endTime,
		})
	}
	return streams
}

// Scan all video files in the archive to figure out our start time and end time.
// We ignore gaps in the recording.
// In future, to find gaps, I plan on using the assumption that if contiguous files have
// start times that are less than X minutes apart, then there is no gap between them,
// and vice versa. X will be our max recording time per video file. For rf1, this
// has a hard limit of 1024 seconds, or just over 17 minutes.
// By using this assumption, we can find gaps by looking at the filenames alone,
// i.e. without having to read the files.
// NOTE: This function is not safe to call during any point besides initial Open()
// of the archive, because we assume here that we have exclusive access to the
// entire data structure - i.e. no thread safety.
func (a *Archive) scan() error {
	forgetStreams := []string{}
	for _, stream := range a.streams {
		// Scan all files in the stream
		streamDir := a.streamDir(stream.name)
		minStartTime := int64(1<<63 - 1)
		maxStartTime := int64(0)
		latestVideoFile := ""
		seenFile := map[string]bool{}
		var firstFormat VideoFormat
		err := filepath.WalkDir(streamDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if path == streamDir {
				// first iteration of walk
				return nil
			}
			if d.IsDir() {
				// This is unexpected, but we'll ignore it.
				// We expect to find a flat list of video files in the stream directory - no sub-directories.
				return filepath.SkipDir
			}
			// Check if this is a video file
			for _, format := range a.formats {
				if format.IsVideoFile(path) {
					onlyFilename := filepath.Base(path)
					// We need to shop the filename up here, because for rf1, look at this example:
					// path: /var/lib/cyclops/archive/camera-0001/1708584695_video.rf1i
					// onlyFilename: 1708584695_video.rf1i
					// Logically, we call this file "1708584695", because there could be more
					// tracks, such as 1708584695_audio.rf1i, and we don't want to count this video twice.
					// It's also nice to consistent in writing and reading video files. So that's why
					// we strip all the rf1-specific filename stuff away here.
					startTimeUnixMilli, _, found := strings.Cut(onlyFilename, "_")
					if found {
						if !seenFile[startTimeUnixMilli] {
							seenFile[startTimeUnixMilli] = true
							if firstFormat != nil && format != firstFormat {
								return fmt.Errorf("Multiple video formats found in stream %v", stream.name)
							} else if firstFormat == nil {
								firstFormat = format
							}
							t, err := strconv.ParseInt(startTimeUnixMilli, 10, 64)
							if err != nil {
								return fmt.Errorf("Invalid number in video file '%v'. Expected '{unixmilli}_...' video filename", onlyFilename)
							}
							if t > maxStartTime {
								latestVideoFile = path
							}
							minStartTime = min(minStartTime, t)
							maxStartTime = max(maxStartTime, t)
							stream.files = append(stream.files, videoFileIndex{
								filename:  startTimeUnixMilli,
								startTime: t,
							})
						}
					} else {
						return fmt.Errorf("Invalid video file '%v'. Expected {unixseconds}_... format", path)
					}
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
		if latestVideoFile != "" {
			// Stream has at least one video file
			stream.format = firstFormat
			stream.startTime = time.UnixMilli(minStartTime)
			if file, err := firstFormat.Open(latestVideoFile); err != nil {
				return fmt.Errorf("Error opening latest video file '%v' in stream %v: %w", latestVideoFile, stream.name, err)
			} else {
				// stream.endTime is the end time of the longest track in the latest video file (all tracks will usually have similar durations)
				stream.endTime = VideoFileMaxTrackEndTime(file)
				file.Close()
			}
			sort.Slice(stream.files, func(i, j int) bool {
				return stream.files[i].startTime < stream.files[j].startTime
			})
		} else {
			// Forget about empty streams, so that we can create them from scratch.
			// Imagine a process dies after creating the stream directory name, but it never actually
			// writes any video files to that stream. Now it's a defunct thing, because we don't know
			// its format. So that's why we just forget about it here, and recreate it if somebody
			// ever tries to write to that stream.
			forgetStreams = append(forgetStreams, stream.name)
		}
	}
	for _, streamName := range forgetStreams {
		delete(a.streams, streamName)
	}
	return nil
}

func (a *Archive) streamDir(streamName string) string {
	return filepath.Join(a.baseDir, streamName)
}

// Close the archive.
// If the system is shutting down, its probably simplest to NOT call Close() on the archive,
// because then you don't have to worry about upsetting any background readers or writers.
// You can just let them go away naturally, as they finish.
func (a *Archive) Close() {
	a.log.Infof("Archive closing")
	a.streamsLock.Lock()
	defer a.streamsLock.Unlock()
	for _, stream := range a.streams {
		stream.contentLock.Lock()
		if stream.current != nil {
			// Close the current video file
			if err := stream.current.file.Close(); err != nil {
				a.log.Errorf("Error closing video file %v: %v", stream.current.filename, err)
			}
			stream.current = nil
		}
		stream.contentLock.Unlock()
	}
	a.log.Infof("Archive closed")
}

func (a *Archive) getOrCreateStream(streamName string) (*videoStream, error) {
	a.streamsLock.Lock()
	defer a.streamsLock.Unlock()
	stream, ok := a.streams[streamName]
	if !ok {
		// Create the stream
		stream = &videoStream{
			name:   streamName,
			format: a.formats[0],
		}
		a.streams[streamName] = stream

		// Ensure the stream directory exists
		if err := os.Mkdir(a.streamDir(streamName), 0770); err != nil && !os.IsExist(err) {
			return nil, fmt.Errorf("Error creating stream directory '%v': %v", a.streamDir(streamName), err)
		}
	}
	return stream, nil
}

// Write a payload to the archive.
// payload keys are track names.
// The payload must always include the exact same set of tracks, even if some of
// them have no new content to write. We use the set of tracks and their properties (eg width, height)
// to figure out when we need to close a file and open a new one. For example, if the user
// decides to enable HD recording, then the track composition would change. Such as change
// requires a new video file.
func (a *Archive) Write(streamName string, payload map[string]TrackPayload) error {
	err := a.write(streamName, payload)
	if err != nil {
		a.log.Errorf("Error writing to stream %v: %v", streamName, err)
	}
	return err
}

func (a *Archive) write(streamName string, payload map[string]TrackPayload) error {
	for track, payload := range payload {
		if payload.TrackType != rf1.TrackTypeVideo {
			return fmt.Errorf("Only video tracks have been implemented. Track %v has type: %v", track, payload.TrackType)
		}
	}

	// Find the earliest packet time.
	// We'll use this if we need to create a new video file.
	hasPackets := false
	minPTSMicro := int64(1<<63 - 1)
	maxPTSMicro := int64(0)
	for _, packets := range payload {
		if len(packets.NALUs) != 0 {
			hasPackets = true
			minPTSMicro = min(minPTSMicro, packets.NALUs[0].PTS.UnixMicro())
			maxPTSMicro = max(maxPTSMicro, packets.NALUs[len(packets.NALUs)-1].PTS.UnixMicro())
		}
	}
	if !hasPackets {
		// If we don't have any packets to write, then we can't create a new video file.
		// Since there are zero packets, this function call is anyway a no-op,
		// so no harm in just returning immediately.
		return nil
	}
	minPTS := time.UnixMicro(minPTSMicro)
	maxPTS := time.UnixMicro(maxPTSMicro)

	stream, err := a.getOrCreateStream(streamName)
	if err != nil {
		return err
	}

	// Ensure that the tracks in the video file are the same set of tracks that
	// the caller is trying to write. If the caller has altered the track composition,
	// then we create a new file.

	// This is a big lock, but there's no simple way around this. We don't want to introduce
	// multi-threaded access into our VideoFile interface - that would be insane.
	// I'm assuming that the write phase here will usually complete quickly, so that we don't
	// end up starving readers. Unless something bad is happening (eg running out of disk space),
	// writes here should complete very quickly, because they're just a copying of memory into
	// the disk cache.
	stream.contentLock.Lock()
	defer stream.contentLock.Unlock()

	if stream.current != nil {
		mustCloseReason := "" // If not empty, then we close
		for trackName, packets := range payload {
			if !VideoFileHasVideoTrack(stream.current.file, trackName, packets.VideoWidth, packets.VideoHeight) {
				mustCloseReason = fmt.Sprintf("Track %v does not exist or has different dimensions", trackName)
				break
			}
			if !stream.current.file.HasCapacity(trackName, packets.NALUs) {
				mustCloseReason = fmt.Sprintf("Insufficient capacity in for track %v", trackName)
				break
			}
			if len(packets.NALUs) > 0 {
				endPTS := packets.NALUs[len(packets.NALUs)-1].PTS
				duration := endPTS.Sub(stream.current.startTime)
				if duration > a.maxVideoFileDuration {
					mustCloseReason = fmt.Sprintf("File has reached max duration %v", a.maxVideoFileDuration)
					break
				}
			}
		}

		if mustCloseReason != "" {
			a.log.Infof("Closing video file %v: %v", stream.current.filename, mustCloseReason)
			err := stream.current.file.Close()
			if err != nil {
				a.log.Errorf("Error closing video file %v: %v", stream.current.filename, err)
			}
			// Add to index
			stream.files = append(stream.files, videoFileIndex{
				filename:  filepath.Base(stream.current.filename),
				startTime: stream.current.startTime.UnixMilli(),
			})
			stream.current = nil
		}
	}

	if stream.current == nil {
		// Create a new video file
		//
		// Filename has resolution of one millisecond.
		// I can't see a scenario where you will start/stop recording within 1ms.
		//
		// At present, unix time is 1708584695, which is 10 digits. We'd like to preserve
		// lexicographic ordering. Do we need to use 11 digits? Unix time will only roll over
		// to 11 digits on 2286-11-20 17:46:40. The world is going to look very different 262
		// years from now. Probably not worth thinking about.
		videoFilename := filepath.Join(a.streamDir(streamName), fmt.Sprintf("%v", minPTSMicro/1000))
		a.log.Infof("Creating new video file %v", videoFilename)
		file, err := stream.format.Create(videoFilename)
		if err != nil {
			return fmt.Errorf("Error creating video file %v: %v", videoFilename, err)
		}
		for track, payload := range payload {
			if err := file.CreateVideoTrack(track, minPTS, payload.Codec, payload.VideoWidth, payload.VideoHeight); err != nil {
				file.Close()
				return fmt.Errorf("Error creating video track %v in %v: %v", track, videoFilename, err)
			}
		}

		stream.current = &videoFile{
			filename:  videoFilename,
			file:      file,
			startTime: minPTS,
			endTime:   minPTS, // We haven't written to the stream yet, so start = end. We'll update endTime further down in this function.
		}
	}

	if minPTS.Before(stream.current.endTime) {
		return fmt.Errorf("Video payload %v starts before the end of the current video file %v. This would cause non-contiguous frames.", minPTS, stream.current.endTime)
	}

	for track, packets := range payload {
		if err := stream.current.file.Write(track, packets.NALUs); err != nil {
			return fmt.Errorf("Error writing to video file %v: %v", stream.current.filename, err)
		}
	}

	stream.current.endTime = maxPTS

	if stream.startTime.IsZero() {
		stream.startTime = minPTS
	}
	stream.endTime = maxPTS

	return nil
}

// Read packets from the archive.
// The map that is returned contains the tracks that were requested.
// If no packets are found, we return an empty map and a nil error.
func (a *Archive) Read(streamName string, trackNames []string, startTime, endTime time.Time) (map[string][]rf1.NALU, error) {
	a.streamsLock.Lock()
	stream := a.streams[streamName]
	a.streamsLock.Unlock()
	if stream == nil {
		return nil, fmt.Errorf("Stream not found: %v", streamName)
	}
	// Do a binary search inside the stream.files to find the file that contains the requested time period.
	// We'll use the file's start time as the key for the binary search.

	// Concrete example to aid in the logic here:
	// 110_video
	// 123_video
	// 136_video
	// 170_video

	// We find the first video with a start time AFTER 'startTime', and then
	// we reverse back by one.

	tracks := map[string][]rf1.NALU{}
	totalBytes := 0

	// Read the tracks from vf, and append them to our result set
	readFromVideoFile := func(filename string, vf VideoFile) error {
		for _, trackName := range trackNames {
			packets, err := vf.Read(trackName, startTime, endTime)
			if err != nil {
				return fmt.Errorf("Error reading track %v from video file %v: %v", trackName, filename, err)
			}
			tracks[trackName] = append(tracks[trackName], packets...)
			for _, p := range packets {
				totalBytes += len(p.Payload)
			}
		}
		if totalBytes > a.maxBytesPerRead {
			return fmt.Errorf("Read limit exceeded: %v bytes", a.maxBytesPerRead)
		}
		return nil
	}

	// Minimize the amount of time that we need to hold stream.contentLock.
	// The crucial thing to note here is that we only need the lock for the
	// the "stream.files" slice and "stream.current". So we make our calculations
	// on those objects, and then we can release the lock. When we go to read
	// from the files, we'll open the video files independently, thereby
	// relying on OS/filesystem concurrency.
	stream.contentLock.Lock()
	startIdx := sort.Search(len(stream.files), func(i int) bool {
		return stream.files[i].startTime >= startTime.UnixMilli()
	}) - 1
	startIdx = max(startIdx, 0)
	endIdx := sort.Search(len(stream.files), func(i int) bool {
		return stream.files[i].startTime >= endTime.UnixMilli()
	})
	indexFiles := stream.files[startIdx:endIdx]
	var useCurrent *videoFile
	if stream.current != nil && DoTimeRangesOverlap(stream.current.startTime, stream.current.endTime, startTime, endTime) {
		useCurrent = stream.current
	}
	stream.contentLock.Unlock()

	// In this section, we have zero locks, so here during our most IO-heavy phase,
	// we have no concurrency problems. Multiple threads could be reading here
	// at the same time.
	for _, file := range indexFiles {
		if file.startTime > endTime.UnixMilli() {
			break
		}
		videoFilename := filepath.Join(a.streamDir(streamName), file.filename)
		videoFile, err := stream.format.Open(videoFilename)
		if err != nil {
			return nil, fmt.Errorf("Error opening video file %v: %v", videoFilename, err)
		}
		defer videoFile.Close()
		if err := readFromVideoFile(videoFilename, videoFile); err != nil {
			return nil, err
		}
	}

	// Here we need to take the contentLock again, before attempting to read from 'current'.
	// We need to manage two scenarios here:
	// 1. Current is still open
	// 2. Current has been closed
	// It is tempting to always reopen 'current', but our rf1 files aren't guaranteed to be in
	// a consistent state if they're still being written to (i.e. index could be written before
	// payload). Because of this, we always try to use our open handle for 'current'.
	if useCurrent != nil {
		stream.contentLock.Lock()
		defer stream.contentLock.Unlock()
		if useCurrent == stream.current {
			// Current is still the same open handle that we found at the start of the Read()
			if err := readFromVideoFile(stream.current.filename, stream.current.file); err != nil {
				return nil, err
			}
		} else {
			// Current got retired, so we need to open it from disk.
			videoFile, err := stream.format.Open(useCurrent.filename)
			if err != nil {
				return nil, fmt.Errorf("Error opening video file %v: %v", useCurrent.filename, err)
			}
			defer videoFile.Close()
			if err := readFromVideoFile(useCurrent.filename, videoFile); err != nil {
				return nil, err
			}
		}
	}

	return tracks, nil
}

func DoTimeRangesOverlap(start1, end1, start2, end2 time.Time) bool {
	return start1.Before(end2) && start2.Before(end1)
}
