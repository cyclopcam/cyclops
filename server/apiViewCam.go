package server

import (
	"encoding/json"
	"math"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/bmharper/cimg/v2"
	"github.com/cyclopcam/cyclops/pkg/videoformat/fsv"
	"github.com/cyclopcam/cyclops/pkg/videox"
	"github.com/cyclopcam/cyclops/server/camera"
	"github.com/cyclopcam/cyclops/server/configdb"
	"github.com/cyclopcam/cyclops/server/defs"
	"github.com/cyclopcam/cyclops/server/streamer"
	"github.com/cyclopcam/www"
	"github.com/julienschmidt/httprouter"
)

func parseResolutionOrPanic(res string) defs.Resolution {
	r, err := defs.ParseResolution(res)
	if err != nil {
		www.PanicBadRequestf("%v", err)
	}
	return r
}

func (s *Server) getCameraFromIDOrPanic(idStr string) *camera.Camera {
	id, _ := strconv.ParseInt(idStr, 10, 64)
	cam := s.LiveCameras.CameraFromID(id)
	if cam == nil {
		www.PanicBadRequestf("Invalid camera ID '%v'", idStr)
	}
	return cam
}

// SYNC-STREAM-INFO-JSON
type streamInfoJSON struct {
	FPS              int     `json:"fps"`
	Width            int     `json:"width"`
	Height           int     `json:"height"`
	FrameSize        float64 `json:"frameSize"`
	KeyFrameSize     float64 `json:"keyFrameSize"`
	InterFrameSize   float64 `json:"interFrameSize"`
	KeyframeInterval int     `json:"keyframeInterval"`
}

// See CameraInfo in www
// camInfoJSON holds information about a running camera. This is distinct from
// it's configuration, which is stored in model.Camera
type camInfoJSON struct {
	ID   int64          `json:"id"`
	Name string         `json:"name"`
	LD   streamInfoJSON `json:"ld"`
	HD   streamInfoJSON `json:"hd"`
}

func toStreamInfoJSON(s *camera.Stream) streamInfoJSON {
	stats := s.RecentFrameStats()
	r := streamInfoJSON{
		FPS:              stats.FPSRounded(),
		FrameSize:        math.Round(stats.FrameSize),
		KeyFrameSize:     math.Round(stats.KeyframeSize),
		InterFrameSize:   math.Round(stats.InterframeSize),
		KeyframeInterval: stats.KeyframeInterval,
	}
	inf := s.Info()
	if inf != nil {
		r.Width = inf.Width
		r.Height = inf.Height
	}
	return r
}

func liveToCamInfoJSON(c *camera.Camera) *camInfoJSON {
	r := &camInfoJSON{
		ID:   c.ID(),
		Name: c.Name(),
		LD:   toStreamInfoJSON(c.LowStream),
		HD:   toStreamInfoJSON(c.HighStream),
	}
	return r
}

func cfgToCamInfoJSON(c *configdb.Camera) *camInfoJSON {
	r := &camInfoJSON{
		ID:   c.ID,
		Name: c.Name,
	}
	return r
}

func (s *Server) httpCamGetInfo(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	cam := s.getCameraFromIDOrPanic(params.ByName("cameraID"))
	www.SendJSON(w, liveToCamInfoJSON(cam))
}

// Fetch a low res JPG of the camera's last image.
// Example: curl -o img.jpg localhost:8080/camera/latestImage/0
func (s *Server) httpCamGetLatestImage(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	cam := s.getCameraFromIDOrPanic(params.ByName("cameraID"))

	www.CacheNever(w)

	contentType := "image/jpeg"
	var encodedImg []byte

	// First try to get latest frame that has had NN detections run on it
	//img, detections, analysis, err := s.monitor.LatestFrame(cam.ID())
	img, _, analysis, err := s.monitor.LatestFrame(cam.ID())
	if err == nil {
		encodedImg, err = cimg.Compress(img, cimg.MakeCompressParams(cimg.Sampling420, 85, 0))
		www.Check(err)
		// We must send Content-Type before X-Detections or X-Analysis... not sure if that's browser or Go HTTP infra, but it's a thing.
		w.Header().Set("Content-Type", contentType)
		// Detections are superfluous, because they're already in the analysis
		//if detections != nil {
		//	jsDet, err := json.Marshal(detections)
		//	www.Check(err)
		//	w.Header().Set("X-Detections", string(jsDet))
		//}
		if analysis != nil {
			jsAna, err := json.Marshal(analysis)
			www.Check(err)
			w.Header().Set("X-Analysis", string(jsAna))
		}
	} else {
		// Fall back to latest frame without NN detections
		s.Log.Infof("httpCamGetLatestImage fallback on camera %v (%v)", cam.ID(), err)
		encodedImg = cam.LatestImage(contentType)
		if encodedImg == nil {
			www.PanicBadRequestf("No image available yet")
		}
		w.Header().Set("Content-Type", contentType)
	}

	w.Write(encodedImg)
}

// Fetch a high res MP4 of the camera's recent footage
// default duration is 5 seconds
// Example: curl -o recent.mp4 localhost:8080/camera/recentVideo/0?duration=15s
func (s *Server) httpCamGetRecentVideo(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	cam := s.getCameraFromIDOrPanic(params.ByName("cameraID"))
	duration, _ := time.ParseDuration(r.URL.Query().Get("duration"))
	if duration <= 0 {
		duration = 5 * time.Second
	}

	www.CacheNever(w)

	contentType := "video/mp4"
	fn := s.TempFiles.GetOnceOff()
	raw, err := cam.ExtractHighRes(camera.ExtractMethodShallowClone, duration)
	www.Check(err)
	www.Check(raw.SaveToMP4(fn))

	www.SendTempFile(w, r, fn, contentType)
}

func (s *Server) httpCamStreamVideo(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	cam := s.getCameraFromIDOrPanic(params.ByName("cameraID"))
	res := parseResolutionOrPanic(params.ByName("resolution"))
	stream := cam.GetStream(res)

	// send backlog for small stream, so user can play immediately.
	// could do the same for high stream too...
	var backlog *camera.VideoRingBuffer
	if res == defs.ResLD {
		backlog = cam.LowDumper
	}

	s.Log.Infof("httpCamStreamVideo websocket upgrading")

	conn, err := s.wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		s.Log.Errorf("httpCamStreamVideo websocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	s.Log.Infof("httpCamStreamVideo starting")

	newDetections := s.monitor.AddWatcher(cam.ID())

	streamer.RunVideoWebSocketStreamer(cam.Name(), s.Log, conn, stream, backlog, newDetections)

	s.monitor.RemoveWatcher(cam.ID(), newDetections)

	s.Log.Infof("httpCamStreamVideo done")
}

func (s *Server) httpCamGetImage(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	cam := s.getCameraFromIDOrPanic(params.ByName("cameraID"))
	res := parseResolutionOrPanic(params.ByName("resolution"))
	timeMS, _ := strconv.ParseInt(params.ByName("time"), 10, 64)

	const modePreviousKeyframe = "previousKeyframe"
	const modeNearestKeyframe = "nearestKeyframe"

	// Default seek mode (when unspecified) is 'nearest frame'
	seekMode := www.QueryValue(r, "seekMode")
	if seekMode != "" && seekMode != modePreviousKeyframe && seekMode != modeNearestKeyframe {
		www.PanicBadRequestf("Invalid seekMode. Must be either blank, 'previousKeyframe' or 'nearestKeyframe'")
	}

	compressQuality := www.QueryInt(r, "quality")
	if compressQuality < 0 || compressQuality > 100 {
		compressQuality = 75
	}
	if s.videoDB == nil {
		www.PanicServerErrorf("VideoDB not initialized")
	}
	startTime := time.UnixMilli(timeMS)
	streamName := "video"
	videoCacheKey := cam.RecordingStreamName(res)
	streamStats := cam.GetStream(res).RecentFrameStats()

	// Even if the user is asking for the nearest frame, we still want to read a little bit into the future, otherwise we might
	// miss that exact frame. 200ms seems like a reasonable maximum frame interval (5 FPS).
	endTime := startTime.Add(200 * time.Millisecond)

	if seekMode == modeNearestKeyframe {
		// The nearest keyframe could be relatively far into the future. The +50ms is just for padding/rounding
		endTime = startTime.Add(streamStats.KeyframeIntervalDuration() + 50*time.Millisecond)
	}

	// Regardless of what the user wants, we always need to read back to a prior keyframe, so that we can initialize the decoder.
	// So fsv.ReadFlagSeekBackToKeyFrame is mandatory here.
	readResult, err := s.videoDB.Archive.Read(cam.RecordingStreamName(res), []string{streamName}, startTime, endTime, fsv.ReadFlagSeekBackToKeyFrame)
	if err != nil {
		www.PanicServerErrorf("Failed to read video: %v", err)
	}
	if readResult[streamName] == nil || len(readResult[streamName].NALS) == 0 {
		www.PanicBadRequestf("No video available at that time")
	}
	pbuffer, err := videox.ExtractFsvPackets(readResult[streamName].Codec, readResult[streamName].NALS)
	www.Check(err)
	if !pbuffer.HasIDR() {
		www.PanicBadRequestf("No keyframes found")
	}

	// While scanning the NALs, we find the frame with the closest time to the requested one.
	// This is important for caching, so that we can correctly identify frames that are requested twice,
	// even if the user doesn't know that he's asking for the same frame (eg two requests 15 ms apart, when frames are actually 100ms apart).
	// In the ideal case, the caller knows the precise time of each frame, but I'm still not sure how
	// I'm going to structure all of this, so I want to be efficient even if the caller doesn't know
	// the precise frame times. The the super-ideal case the user can decode all codecs, but I'm also
	// not confident that I can rely on that yet.

	// Regardless of the seek mode, make sure we find the exact frame that we want to decode, and
	// pass the precise time of that frame into DecodeClosestImageInPacketList(). This allows DecodeClosestImageInPacketList()
	// to correctly identify frames for caching.
	closestPacketIdx := 0
	if seekMode == modePreviousKeyframe {
		closestPacketIdx = pbuffer.FindFirstIDR()
	} else {
		closestPacketIdx = pbuffer.FindClosestPacketWallPTS(startTime, seekMode == modeNearestKeyframe)
	}
	findImageAt := pbuffer.Packets[closestPacketIdx].WallPTS
	if seekMode == modePreviousKeyframe || seekMode == modeNearestKeyframe {
		// Don't waste time decoding frames prior to the keyframe we're actually interested in
		pbuffer.Packets = pbuffer.Packets[closestPacketIdx:]
	}

	codec, err := videox.ParseCodec(readResult[streamName].Codec)
	www.Check(err)
	img, imgTime, err := videox.DecodeClosestImageInPacketList(codec, pbuffer.Packets, findImageAt, s.seekFrameCache, videoCacheKey)
	if err != nil {
		www.PanicServerErrorf("Failed to decode video: %v", err)
	}
	encodedImg, err := cimg.Compress(img, cimg.MakeCompressParams(cimg.Sampling420, compressQuality, 0))
	www.Check(err)

	// Read events, so that the front-end can show boxes around detected objects.
	events, err := s.videoDB.ReadEvents(cam.LongLivedName(), imgTime.Add(-time.Second), imgTime.Add(time.Second))
	if err != nil {
		s.Log.Errorf("Failed to read events at %v: %v", imgTime, err)
	} else {
		// Create a monitor.Analysis data structure, because we're already using this to transmit detection information
		// during live view playback, so might as well use the same thing.
		analysis := s.copyEventsToMonitorAnalysis(cam.ID(), events, imgTime)
		jsAna, err := json.Marshal(analysis)
		www.Check(err)
		w.Header().Set("X-Analysis", string(jsAna))
	}

	www.CacheSeconds(w, 3600) // could probably cache forever
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("X-Cyclops-Frame-Time", strconv.FormatInt(imgTime.UnixMilli(), 10))
	// Getting rid of this FPS estimate, because we already know the camera FPS
	//if len(packetIntervals) != 0 {
	//	w.Header().Set("X-Cyclops-FPS", strconv.FormatFloat(camera.EstimateFPS(packetIntervals), 'f', 2, 64))
	//}
	w.Write(encodedImg)
}

// Get the video frames available in the given time window.
// This is used by the seek bar to know precisely which frames to ask for.
// Knowing the exact frames is vital for an efficient cache.
// Granularity is specified in milliseconds-per-pixel. It is used to determine the granularity of the frames
// that we return. If not specified, then we assume your query window is 1000 pixels wide.
// NOTE: While writing this, I realized we could do MUCH better, by compressing the frame times at
// the DB level.
// ALSO NOTE: This API has not yet been used, and probably never will
func (s *Server) httpCamGetFrames(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	type frameJSON struct {
		PTS float64 `json:"pts"` // Frame presentation time in unix milliseconds
	}
	type getFramesJSON struct {
		Frames []frameJSON `json:"frames"`
	}
	cam := s.getCameraFromIDOrPanic(params.ByName("cameraID"))
	res := parseResolutionOrPanic(params.ByName("resolution"))
	startTimeMS, _ := strconv.ParseInt(params.ByName("startTime"), 10, 64)
	endTimeMS, _ := strconv.ParseInt(params.ByName("endTime"), 10, 64)
	granularity := www.QueryFloat64(r, "granularity") / 1000.0 // convert milliseconds to seconds
	if granularity == 0 {
		// Assume 300 pixels wide
		// The 1000.0 is to convert from milliseconds to seconds
		granularity = float64(endTimeMS-startTimeMS) / (300.0 * 1000.0)
	}
	if endTimeMS <= startTimeMS {
		www.PanicBadRequestf("Invalid time range")
	}
	if endTimeMS-startTimeMS > 5*3600*1000 {
		www.PanicBadRequestf("Invalid time range")
	}

	if s.videoDB == nil {
		www.PanicServerErrorf("VideoDB not initialized")
	}
	startTime := time.UnixMilli(startTimeMS)
	endTime := time.UnixMilli(endTimeMS)
	streamName := "video"
	// Always start reading at a keyframe, so that we have a consistent time base when we skip frames
	packets, err := s.videoDB.Archive.Read(cam.RecordingStreamName(res), []string{streamName}, startTime, endTime, fsv.ReadFlagHeadersOnly|fsv.ReadFlagSeekBackToKeyFrame)
	if err != nil {
		www.PanicServerErrorf("Failed to read video: %v", err)
	}
	resp := getFramesJSON{}
	readResult := packets[streamName]
	if readResult == nil || len(readResult.NALS) == 0 {
		www.SendJSON(w, resp)
		return
	}
	keyframeThreshold := granularity * 3
	// Figure out the keyframe interval.
	// If keyframes occur more frequently than keyframeThreshold, then we're zoomed out enough that we only return keyframes.
	lastKeyframePTS := time.Time{}
	keyframeNSample := 0
	keyframeDeltaSum := time.Duration(0)
	for i := range readResult.NALS {
		if readResult.NALS[i].IsKeyFrame() {
			delta := readResult.NALS[i].PTS.Sub(lastKeyframePTS)
			// assume keyframes are no more than 10 seconds apart
			if delta < 11*time.Second {
				keyframeNSample++
				keyframeDeltaSum += delta
			}
			lastKeyframePTS = readResult.NALS[i].PTS
		}
	}
	keyframeGranularity := 1.0
	if keyframeNSample != 0 {
		keyframeGranularity = keyframeDeltaSum.Seconds() / float64(keyframeNSample)
	}
	onlyKeyframes := false
	if keyframeGranularity <= keyframeThreshold {
		// only return keyframes
		s.Log.Infof("Only returning keyframes. %.1f <= %.1f (granularity %.1fms)", keyframeGranularity*1000, keyframeThreshold*1000, granularity*1000)
		onlyKeyframes = true
	}
	lastPTS := time.Time{}
	timeBetweenFrames := time.Duration(granularity*1000) * time.Millisecond
	for _, n := range readResult.NALS {
		useFrame := n.IsKeyFrame() || (!onlyKeyframes && n.PTS.Sub(lastPTS) >= timeBetweenFrames)
		if useFrame {
			lastPTS = n.PTS
		}
	}
	s.Log.Infof("Result has %v/%v frames", len(resp.Frames), len(readResult.NALS))
	www.SendJSON(w, resp)
}

// This is for developers, for gathering unit tests data.
// We save mp4 files of the camera's footage, to the local disc.
func (s *Server) httpCamDebugSaveClip(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	cam := s.getCameraFromIDOrPanic(params.ByName("cameraID"))
	startTimeMS, _ := strconv.ParseInt(params.ByName("startTime"), 10, 64)
	endTimeMS, _ := strconv.ParseInt(params.ByName("endTime"), 10, 64)
	duration := time.Duration(endTimeMS-startTimeMS) * time.Millisecond
	if duration <= 0 {
		www.PanicBadRequestf("Invalid time range")
	}
	resolutions := []defs.Resolution{defs.ResLD, defs.ResHD}
	for _, res := range resolutions {
		result, err := s.videoDB.Archive.Read(cam.RecordingStreamName(res), []string{"video"}, time.UnixMilli(startTimeMS), time.UnixMilli(endTimeMS), fsv.ReadFlagSeekBackToKeyFrame)
		www.Check(err)
		pbuffer, err := videox.ExtractFsvPackets(result["video"].Codec, result["video"].NALS)
		www.Check(err)
		fn := filepath.Join(s.configDB.GetConfig().Recording.Path, "clip-"+string(res)+".mp4")
		www.Check(pbuffer.SaveToMP4(fn))
	}
	www.SendOK(w)
}

// Example usage: curl -u USERNAME:PASSWORD localhost:8080/api/camera/debug/stats
func (s *Server) httpCamDebugStats(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	result := map[string]camera.StreamStats{}
	for _, cam := range s.LiveCameras.Cameras() {
		result[cam.Name()+"-HD"] = cam.HighStream.RecentFrameStats()
		result[cam.Name()+"-LD"] = cam.LowStream.RecentFrameStats()
	}
	www.SendJSON(w, result)
}

// Example usage: curl -u USERNAME:PASSWORD localhost:8080/api/camera/debug/frameTimes/1/HD
func (s *Server) httpCamDebugFrameTimes(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	cam := s.getCameraFromIDOrPanic(params.ByName("cameraID"))
	res := parseResolutionOrPanic(params.ByName("resolution"))
	stream := cam.GetStream(res)
	times := stream.RecentFrameTimes()
	// Turn absolute times into intervals
	for i := len(times) - 1; i > 0; i-- {
		times[i] = times[i] - times[i-1]
		times[i] = math.Round(times[i]*1000) / 1000
	}

	times = times[1:]
	www.SendJSON(w, times)
}
