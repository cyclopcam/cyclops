package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/bmharper/cimg/v2"
	"github.com/cyclopcam/cyclops/pkg/gen"
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

type streamInfoJSON struct {
	FPS            int     `json:"fps"`
	Width          int     `json:"width"`
	Height         int     `json:"height"`
	FrameSize      float64 `json:"frameSize"`
	KeyFrameSize   float64 `json:"keyFrameSize"`
	InterFrameSize float64 `json:"interFrameSize"`
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
		FPS:            stats.FPSRounded(),
		FrameSize:      stats.FrameSize,
		KeyFrameSize:   stats.KeyFrameSize,
		InterFrameSize: stats.InterFrameSize,
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
	img, detections, analysis, err := s.monitor.LatestFrame(cam.ID())
	if err == nil {
		encodedImg, err = cimg.Compress(img, cimg.MakeCompressParams(cimg.Sampling420, 85, 0))
		www.Check(err)
		// We must send Content-Type before X-Detections or X-Analysis... not sure if that's browser or Go HTTP infra, but it's a thing.
		w.Header().Set("Content-Type", contentType)
		if detections != nil {
			jsDet, err := json.Marshal(detections)
			www.Check(err)
			w.Header().Set("X-Detections", string(jsDet))
		}
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
	seekToPreviousKeyframe := www.QueryValue(r, "seekMode") == "previousKeyframe" // If toggled, then return the first keyframe before 'time'
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
	// Regardless of what the user wants, we always need to read back to a prior keyframe, so that we can initialize the decoder.
	// So fsv.ReadFlagSeekBackToKeyFrame is mandatory here.
	packets, err := s.videoDB.Archive.Read(cam.RecordingStreamName(res), []string{streamName}, startTime, startTime, fsv.ReadFlagSeekBackToKeyFrame)
	if err != nil {
		www.PanicServerErrorf("Failed to read video: %v", err)
	}
	if packets[streamName] == nil || len(packets[streamName].NALS) == 0 {
		www.PanicBadRequestf("No video available at that time")
	}
	packet := &videox.VideoPacket{
		WallPTS: packets[streamName].NALS[0].PTS,
	}
	// While scanning the NALs, we find the frame with the closest time to the requested one.
	// This is important for caching, so that we can correctly identify frames that are requested twice,
	// even if the user doesn't know that he's asking for the same frame (eg two requests 15 ms apart, when frames are actually 100ms apart).
	bestMatchDeltaT := time.Duration(1<<63 - 1)
	bestMatchPTS := time.Time{}
	outPackets := []*videox.VideoPacket{}
	packetIntervals := []time.Duration{}
	for _, p := range packets[streamName].NALS {
		if p.PTS != packet.WallPTS {
			packetIntervals = append(packetIntervals, p.PTS.Sub(packet.WallPTS))
			outPackets = append(outPackets, packet)
			packet = &videox.VideoPacket{
				WallPTS: p.PTS,
			}
		}
		n := videox.NALU{
			PayloadIsAnnexB: p.Flags&fsv.NALUFlagAnnexB != 0,
			Payload:         p.Payload,
		}
		packet.H264NALUs = append(packet.H264NALUs, n)
		deltaT := gen.Abs(p.PTS.Sub(startTime))
		if deltaT < bestMatchDeltaT {
			bestMatchDeltaT = deltaT
			bestMatchPTS = p.PTS
		}
	}
	outPackets = append(outPackets, packet)
	//fmt.Printf("%v packets. Packet 0: %v\n", len(outPackets), outPackets[0].WallPTS)
	findImageAt := bestMatchPTS
	if seekToPreviousKeyframe {
		// Zero time means "return first image that decodes"
		findImageAt = time.Time{}
	}
	codec, err := videox.ParseCodec(packets[streamName].Codec)
	www.Check(err)
	img, imgTime, err := videox.DecodeClosestImageInPacketList(codec, outPackets, findImageAt, s.seekFrameCache, videoCacheKey)
	if err != nil {
		www.PanicServerErrorf("Failed to decode video: %v", err)
	}
	encodedImg, err := cimg.Compress(img, cimg.MakeCompressParams(cimg.Sampling420, compressQuality, 0))
	www.Check(err)
	www.CacheSeconds(w, 3600) // could cache forever - but need to test different things like LD/HD and quality
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("X-Cyclops-Frame-Time", strconv.FormatInt(imgTime.UnixMicro(), 10))
	if len(packetIntervals) != 0 {
		w.Header().Set("X-Cyclops-FPS", strconv.FormatFloat(camera.EstimateFPS(packetIntervals), 'f', 2, 64))
	}
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
