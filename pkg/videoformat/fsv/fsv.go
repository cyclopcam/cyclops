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

	"github.com/cyclopcam/cyclops/pkg/kibi"
	"github.com/cyclopcam/cyclops/pkg/log"
	"github.com/cyclopcam/cyclops/pkg/perfstats"
	"github.com/cyclopcam/cyclops/pkg/videoformat/rf1"
)

// videoFile is a single logical video file, even if it's split into multiple physical files (eg rf1)
// A videoFile will typically have one or two tracks inside (video, or video + audio).
type videoFile struct {
	filename  string    // Example /var/lib/cyclops/archive/camera-0001/1712815946731
	startTime time.Time // Start time of the video file
	endTime   time.Time // End time of the video file
	file      VideoFile // Interface to the on-disk format. This is an open handle to the video file, and needs to be Closed() when we're done with it.
	tracks    []string  // Names of all the tracks that we've written in this file
}

// A small-memory-footprint record that exists for every file in the archive
type videoFileIndex struct {
	filename  string // Only the logical filename, such as "1712815946731"
	startTime int64  // Milliseconds UTC, should be equal to the filename (we might consider getting rid of "filename")
	size      int64  // Size of the file in bytes. For rf1 files, this is the sum of all rf1 files (all tracks: index files and packet files)

	// Names of the tracks (necessary for rf1, so we can delete all tracks/files of the video without scanning the filesystem).
	// Note: This array is likely shared with many (or all) other videoFileIndex objects in the same stream.
	// We do this to save memory. So be careful if you're manipulating the 'tracks' array, or the strings inside.
	tracks []string
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
	// Recently written packets, with their payloads set to nil.
	// Used to splice overlapping writes.
	// Key is track name.
	recentWrite map[string][]rf1.NALU
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

func MakeVideoPayload(codec string, width, height int, nalus []rf1.NALU) TrackPayload {
	return TrackPayload{
		Codec:       codec,
		VideoWidth:  width,
		VideoHeight: height,
		NALUs:       nalus,
	}
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
	sweepStop            chan bool     // Tell the sweeper to stop
	sweeperStopped       chan bool     // Sweeper closes this once it has stopped
	recentWriteMaxQueue  int           // Max number of NALU headers we'll store in videoStream.recentWrite

	settingsLock sync.Mutex // Guards access to settings
	settings     ArchiveSettings

	streamsLock sync.Mutex // Guards access to the streams map. Access inside a stream needs stream.contentLock.
	streams     map[string]*videoStream

	firstWrite        time.Time                  // Time when we wrote our first byte
	bytesWrittenStat  perfstats.Int64Accumulator // All the bytes that we've written
	writeTimeStat     perfstats.TimeAccumulator  // How long each write took
	lastStatWriteTime time.Time                  // Last time we wrote stats to log
	numStatWrites     int64                      // Number of times we've written stats to log
}

// Archive Settings
type ArchiveSettings struct {
	MaxBytesPerRead int           // Maximum number of bytes that we will return from a single Read()
	MaxArchiveSize  int64         // Maximum size of all files in the archive. We will eat into old files when we need to recycle space. Zero = no limit.
	SweepInterval   time.Duration // How often we check if we need to recycle space
}

func DefaultArchiveSettings() ArchiveSettings {
	return ArchiveSettings{
		MaxBytesPerRead: 256 * 1024 * 1024, // 256MB
		MaxArchiveSize:  0,                 // No limit
		SweepInterval:   time.Minute,
	}
}

// Open a directory of video files for reading and/or writing.
// The directory baseDir must exist, but it may be empty.
// When creating new streams, formats[0] is used, so the ordering
// of formats is important.
func Open(logger log.Log, baseDir string, formats []VideoFormat, settings ArchiveSettings) (*Archive, error) {
	if len(formats) == 0 {
		return nil, fmt.Errorf("No video formats provided")
	}

	// This must be lower or equal to the max duration of the file formats that we support.
	// rf1 has a max duration of 1024 seconds, so that's why we choose 1000, because it's
	// less than 1024, with a bit of margin.
	// You can't arbitrarily change this constant after creating an archive, because then when
	// you scan for video files matching a given time period, you may skip a valid file.
	// The reason is because we round down to the previous period, and then use a filesystem glob
	// to find files. When we do this filesystem scan, the filename tells us the start time
	// of the video, but we don't know the duration from the filename. But if we know that the
	// duration cannot be greater than maxVideoFileDuration, then we can figure it out, if the
	// videos are contiguous. But if you change maxVideoFileDuration, then that knowledge is
	// no longer valid.
	maxVideoFileDuration := 1000 * time.Second

	baseDir = strings.TrimSuffix(baseDir, "/")
	// Scan top-level directories.
	// Each directory is a stream (eg camera-0001).
	archive := &Archive{
		log:                  log.NewPrefixLogger(logger, "Archive:"),
		baseDir:              baseDir,
		formats:              formats,
		streams:              map[string]*videoStream{},
		maxVideoFileDuration: maxVideoFileDuration,
		recentWriteMaxQueue:  1000, // at 30 fps, 1000/30 = 33 seconds of recent writes. My plan is to include 15 seconds of history at 10 FPS, so 33 at 30 FPS is a plenty big buffer.
		settings:             settings,
		lastStatWriteTime:    time.Now(),
	}
	err := filepath.WalkDir(baseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && path != baseDir {
			streamName := filepath.Base(path)
			archive.streams[streamName] = &videoStream{
				name:        streamName,
				recentWrite: map[string][]rf1.NALU{},
			}
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Scan for all video files, so that we know the start and end time of each stream,
	// and the name of every file. Also - their sizes.
	if err := archive.scan(); err != nil {
		return nil, fmt.Errorf("Error scanning archive: %v", err)
	}

	archive.startSweeper()

	return archive, nil
}

func (a *Archive) Settings() ArchiveSettings {
	a.settingsLock.Lock()
	defer a.settingsLock.Unlock()
	return a.settings
}

func (a *Archive) SetSettings(settings ArchiveSettings) {
	a.settingsLock.Lock()
	defer a.settingsLock.Unlock()
	a.settings = settings
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
	scanStart := time.Now()
	defer func() {
		a.log.Infof("Archive scan took %v", time.Now().Sub(scanStart))
	}()

	for _, stream := range a.streams {
		if err := a.scanStream(stream); err != nil {
			return err
		}
	}

	// Forget about empty streams, so that we can create them from scratch.
	// Imagine a process dies after creating the stream directory name, but it never actually
	// writes any video files to that stream. Now it's a defunct thing, because we don't know
	// its format. So that's why we just forget about it here, and recreate it if somebody
	// ever tries to write to that stream.
	forgetStreams := []string{}
	for _, stream := range a.streams {
		if stream.startTime.IsZero() {
			forgetStreams = append(forgetStreams, stream.name)
		}
	}
	for _, streamName := range forgetStreams {
		delete(a.streams, streamName)
	}

	return nil
}

func (a *Archive) scanStream(stream *videoStream) error {
	// Scan all files in the stream
	streamDir := a.streamDir(stream.name)
	foundTime := map[string]*videoFileIndex{} // Total size of all .rf1i/.rf1p files (can be multiple tracks)
	foundVideo := map[string]bool{}           // Have we found the .rf1i file?
	streamFormat := a.formats[0]              // Just assume rf1
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
		onlyFilename := filepath.Base(path)
		// We need to chop the filename up here, because for rf1, look at this example:
		// path: /var/lib/cyclops/archive/cam-1-HD/1708584695_video.rf1i
		// onlyFilename: 1708584695_video.rf1i
		// Logically, we call this file "1712815946731", because there could be more
		// tracks, such as 1708584695_audio.rf1i, and we don't want to count this video twice.
		// It's also nice to be consistent in writing and reading video files. So that's why
		// we strip all the rf1-specific filename stuff away here.
		tMilli := int64(0)
		startTimeUnixMilli, remainder, splitOK := strings.Cut(onlyFilename, "_")
		if splitOK {
			tMilli, _ = strconv.ParseInt(startTimeUnixMilli, 10, 64)
		}
		if tMilli == 0 {
			// Ignore files that don't start with "{unixmilli}_"
			return nil
		}
		trackName, ext, _ := strings.Cut(remainder, ".")
		if ext != "rf1i" && ext != "rf1p" {
			// Ignore unrecognized filename
			return nil
		}
		//if err != nil {
		//	return fmt.Errorf("Invalid number in video file '%v'. Expected '{unixmilli}_...' video filename", onlyFilename)
		//}
		// Ignore stat error
		st, _ := os.Stat(path)
		entry := foundTime[startTimeUnixMilli]
		if entry == nil {
			foundTime[startTimeUnixMilli] = &videoFileIndex{
				filename:  startTimeUnixMilli,
				startTime: tMilli,
				size:      0,
			}
			entry = foundTime[startTimeUnixMilli]
		}
		// Sum of all files with the same timestamp
		entry.size += st.Size()

		if ext == "rf1i" {
			foundVideo[startTimeUnixMilli] = true
			entry.tracks = append(entry.tracks, trackName)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if len(foundVideo) != 0 {
		// Stream has at least one video file
		for filename := range foundVideo {
			stream.files = append(stream.files, *foundTime[filename])
		}
		sort.Slice(stream.files, func(i, j int) bool {
			return stream.files[i].startTime < stream.files[j].startTime
		})
		latestVideoFile := filepath.Join(streamDir, stream.files[len(stream.files)-1].filename)

		stream.format = streamFormat
		stream.startTime = time.UnixMilli(stream.files[0].startTime)
		if file, err := streamFormat.Open(latestVideoFile); err != nil {
			return fmt.Errorf("Error opening latest video file '%v' in stream %v: %w", latestVideoFile, stream.name, err)
		} else {
			// stream.endTime is the end time of the longest track in the latest video file (all tracks will usually have similar durations)
			stream.endTime = VideoFileMaxTrackEndTime(file)
			file.Close()
		}
	}

	// Remove memory duplication for all of the track name strings.
	// These are basically all the same, so it's pointless to store N arrays, each containing M strings,
	// and have all of those things be unique memory. For a given stream, odds are very high that
	// every single file in the stream has the exact same track list.
	// I hope this doesn't come back to bite us if we forget that these
	// track lists share the same memory!
	sig2list := map[string][]string{} // Map from concatenated track names into list of tracks
	for i := range stream.files {
		sig := strings.Join(stream.files[i].tracks, "|")
		if existing, ok := sig2list[sig]; ok {
			stream.files[i].tracks = existing
		} else {
			sig2list[sig] = stream.files[i].tracks
		}
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
	a.stopSweeper()
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
			name:        streamName,
			format:      a.formats[0],
			recentWrite: map[string][]rf1.NALU{},
		}
		a.streams[streamName] = stream

		// Ensure the stream directory exists
		if err := os.Mkdir(a.streamDir(streamName), 0770); err != nil && !os.IsExist(err) {
			return nil, fmt.Errorf("Error creating stream directory '%v': %v", a.streamDir(streamName), err)
		}
	}
	return stream, nil
}

func (a *Archive) deleteEmptyStreamHaveLock(streamName string) {
	dir := a.streamDir(streamName)
	if err := os.RemoveAll(dir); err != nil {
		a.log.Warnf("Failed to remove empty stream directory %v: %v", dir, err)
	}
	delete(a.streams, streamName)
}

// Return the total size of all files in the archive
func (a *Archive) TotalSize() int64 {
	ss := a.StreamSizes()
	total := int64(0)
	for _, size := range ss {
		total += size
	}
	return total
}

// Return the size of each stream
func (a *Archive) StreamSizes() map[string]int64 {
	streamSize := map[string]int64{}

	// Make a copy of 'a.streams', so that we don't need to hold a.streamsLock for long.
	a.streamsLock.Lock()
	streams := make([]*videoStream, 0, len(a.streams))
	for _, stream := range a.streams {
		streams = append(streams, stream)
	}
	a.streamsLock.Unlock()

	for _, stream := range streams {
		stream.contentLock.Lock()

		size := int64(0)
		for _, file := range stream.files {
			size += file.size
		}
		if stream.current != nil {
			currentSize, _ := stream.current.file.Size()
			size += currentSize
		}
		streamSize[stream.name] = size

		stream.contentLock.Unlock()
	}

	return streamSize
}

func DoTimeRangesOverlap(start1, end1, start2, end2 time.Time) bool {
	return start1.Before(end2) && start2.Before(end1)
}

func totalPayloadBytes(p []rf1.NALU) int64 {
	total := int64(0)
	for _, nalu := range p {
		total += int64(len(nalu.Payload))
	}
	return total
}

func (a *Archive) AutoStatsToLog() {
	interval := 60 * time.Second
	if a.numStatWrites > 3 {
		interval = 15 * time.Minute
	}
	now := time.Now()
	if now.Sub(a.lastStatWriteTime) < interval {
		return
	}
	elapsed := now.Sub(a.firstWrite)
	if elapsed.Seconds() < 5 {
		return
	}
	a.log.Infof("Bytes per second: %v (%v samples)", kibi.FormatBytesHighPrecision(a.bytesWrittenStat.Total/int64(elapsed.Seconds())), a.bytesWrittenStat.Samples)
	a.log.Infof("Average time per write: %v (%v samples)", a.writeTimeStat.Average(), a.writeTimeStat.Samples)
	a.lastStatWriteTime = time.Now()
	a.numStatWrites++
	if a.numStatWrites%10 == 0 {
		a.log.Infof("Resetting stats")
		a.firstWrite = time.Now()
		a.bytesWrittenStat.Reset()
		a.writeTimeStat.Reset()
	}
}
