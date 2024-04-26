package camera

import (
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bluenviron/gortsplib/v4/pkg/format/rtph264"
	"github.com/bluenviron/mediacommon/pkg/codecs/h264"
	"github.com/bmharper/ringbuffer"
	"github.com/cyclopcam/cyclops/pkg/gen"
	"github.com/cyclopcam/cyclops/pkg/log"
	"github.com/cyclopcam/cyclops/pkg/videox"
	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/gortsplib/v4/pkg/liberrors"
)

// A stream sink is fundamentally just a channel
type StreamSinkChan chan StreamMsg

// StandardStreamSink allows you to run the stream with RunStandardStream()
// This is really just a convenience wrapper around StreamSinkChan.
type StandardStreamSink interface {
	// OnConnect is called by Stream.ConnectSinkAndRun().
	// You must return a channel to which all stream messages will be sent.
	OnConnect(stream *Stream) (StreamSinkChan, error)
	OnPacketRTP(packet *videox.VideoPacket) // Called by RunStandardStream(), when it receives a StreamMsgTypePacket
	Close()                                 // Called by RunStandardStream(), when it receives a StreamMsgTypeClose
}

type StreamMsgType int

const (
	StreamMsgTypePacket StreamMsgType = iota // New camera packet
	StreamMsgTypeClose                       // Close yourself. There will be no further packets.
)

// StreamMsg is sent on a channel from the stream to a sink
type StreamMsg struct {
	Type   StreamMsgType
	Stream *Stream
	Packet *videox.VideoPacket
}

// There isn't much rhyme or reason behind this number
const StreamSinkChanDefaultBufferSize = 2

type StreamInfo struct {
	Width  int
	Height int
}

// Average observed stats from a sample of recent frames
type StreamStats struct {
	FPS            float64 // frames per second
	FrameSize      float64 // average frame size in bytes
	KeyFrameSize   float64 // average key-frame size in bytes
	InterFrameSize float64 // average non-key-frame size in bytes
}

func (s *StreamStats) FPSRounded() int {
	return int(math.Round(s.FPS))
}

type frameStat struct {
	isIDR bool
	pts   time.Duration
	size  int
}

// Stream is a bridge between the RTSP library (gortsplib) and one or more "sink" objects.
// The stream understands just enough about RTSP and video codecs to be able to receive
// information from gortsplib, transform them into our own internal data structures,
// and pass them onto the sinks.
// For each camera, we create one stream to handle the high res video, and another stream
// for the low res video.
type Stream struct {
	Log        log.Log
	Client     *gortsplib.Client
	Ident      string // Just for logs. Simply CameraName.StreamName.
	CameraName string // Just for logs
	StreamName string // The stream name, such as "low" and "high"

	// These are read at the start of Listen(), and will be populated before Listen() returns
	//H264TrackID int                  // 0-based track index
	//H264Track   *gortsplib.TrackH264 // track object

	sinksLock sync.Mutex
	sinks     []StreamSinkChan
	closeWG   *sync.WaitGroup
	isClosed  bool

	infoLock sync.Mutex
	info     *StreamInfo // With Go 1.19 one could use atomic.Pointer[T] here

	livenessLock                 sync.Mutex
	livenessLastPacketReceivedAt time.Time

	// Used to infer real time from packet's relative timestamps
	refTimeWall     time.Time
	refTimeDuration time.Duration

	recentFramesLock sync.Mutex
	recentFrames     ringbuffer.RingP[frameStat] // Some stats of recent frames (eg PTS, size)
	loggedStatsAt    time.Time
}

func NewStream(logger log.Log, cameraName, streamName string) *Stream {
	return &Stream{
		Log:          log.NewPrefixLogger(logger, "Stream "+cameraName+"."+streamName),
		recentFrames: ringbuffer.NewRingP[frameStat](128),
		CameraName:   cameraName,
		StreamName:   streamName,
		Ident:        cameraName + "." + streamName,
	}
}

func (s *Stream) Listen(address string) error {
	if s.Client != nil {
		s.Client.Close()
		s.Client = nil
	}
	client := &gortsplib.Client{}

	// parse URL
	u, err := base.ParseURL(address)
	if err != nil {
		return fmt.Errorf("Invalid stream URL: %w", err)
	}
	camHost := u.Host + u.Path

	// connect to the server
	s.Log.Infof("Connecting to %v", camHost)
	err = client.Start(u.Scheme, u.Host)
	if err != nil {
		return fmt.Errorf("Failed to start stream: %w", err)
	}
	// From this point on, we're responsible for calling client.Close()
	s.Client = client

	// find published tracks
	session, _, err := client.Describe(u)
	if err != nil {
		if e, ok := err.(liberrors.ErrClientBadStatusCode); ok {
			if e.Code == 401 {
				//s.Log.Infof("Connection failed. Details: %v", e.Message)
				return fmt.Errorf("Invalid username or password")
			}
		}
		return err
	}

	// find the H264 track
	var forma *format.H264
	media := session.FindFormat(&forma)
	if media == nil {
		return fmt.Errorf("H264 track not found")
	}
	//h264TrackID, h264track := func() (int, *gortsplib.TrackH264) {
	//	for i, track := range tracks {
	//		if h264track, ok := track.(*gortsplib.TrackH264); ok {
	//			return i, h264track
	//		}
	//	}
	//	return -1, nil
	//}()
	//if h264TrackID < 0 {
	//	return fmt.Errorf("H264 track not found")
	//}

	rtpDecoder, err := forma.CreateDecoder()
	if err != nil {
		return fmt.Errorf("Failed to create H264 decoder: %w", err)
	}

	client.Setup(session.BaseURL, media, 0, 0)

	// From old gortplib version
	//s.H264TrackID = h264TrackID
	//s.H264Track = h264track

	s.Log.Infof("Connected to %v, track %v", camHost, media.ID)

	recvID := atomic.Int64{}
	nWarningsAboutNoPTS := 0

	client.OnPacketRTP(media, forma, func(pkt *rtp.Packet) {
		now := time.Now()
		myPacketID := recvID.Add(1)

		s.livenessLock.Lock()
		s.livenessLastPacketReceivedAt = now
		s.livenessLock.Unlock()

		//if s.CameraName == "Driveway" && s.StreamName == "high" && s.info == nil {
		//	s.Log.Infof("Received packet %v (%v bytes)", myPacketID, len(pkt.Payload))
		//}

		pts, ok := client.PacketPTS(media, pkt)
		if !ok {
			if nWarningsAboutNoPTS == 0 {
				s.Log.Warnf("Ignoring H264 packet without PTS")
			}
			nWarningsAboutNoPTS = min(nWarningsAboutNoPTS+1, 1000)
			return
		}

		nWarningsAboutNoPTS >>= 1

		// I don't seem to be getting NTP info from my Hikvision cameras
		//ntp, ntpOK := client.PacketNTP(media, pkt)

		nalus, err := rtpDecoder.Decode(pkt)
		if err != nil {
			if err != rtph264.ErrNonStartingPacketAndNoPrevious && err != rtph264.ErrMorePacketsNeeded {
				s.Log.Errorf("Failed to decode H264 packet: %v", err)
			}
			return
		}

		// Note that gortsplib also has client.PacketNTP(), which we could experiment with.
		// Perhaps we should measure NTP time from the camera, and if its close enough to our
		// perceived time, then use the camera's time.

		// establish reference time
		if s.refTimeWall.IsZero() && len(nalus) != 0 {
			s.refTimeWall = now
			s.refTimeDuration = pts
		}

		// compute absolute PTS
		refTime := time.Time{}
		if !s.refTimeWall.IsZero() {
			refTime = s.refTimeWall.Add(pts - s.refTimeDuration)
			//if ntpOK {
			//	fmt.Printf("ntp: %v, refTime: %v\n", ntp, refTime)
			//}
		}

		// Populate width & height.
		s.infoLock.Lock()
		if s.info == nil {
			if inf := s.extractSPSInfo(nalus); inf != nil {
				s.info = inf
				s.Log.Infof("Size: %v x %v (after %v packets)", inf.Width, inf.Height, myPacketID)
			}
			//if myPacketID == 100 && s.info == nil {
			//	s.Log.Warnf("Failed to extract SPS info after 100 packets")
			//}
		}
		s.infoLock.Unlock()

		s.addFrameToStats(nalus, pts)

		// Before we return, we must clone the packet. This is because we send
		// the packet via channels, to all of our stream sinks. These sinks
		// are running on unspecified threads, so we have no idea how long
		// it will take before this data gets processed. gortsplib re-uses
		// the packet buffers, so if our sinks take too long to process this
		// packet, then we've got a race condition.
		// The only safe solution is to copy the packet entirely, before returning
		// from this function.
		// We have to ask the question: Is it possible to avoid this memory copy?
		// And the answer is: only if gortsplib gave us control over that.
		// The good news is that usually we're not recording the high resolution
		// streams, so this penalty is not too severe.
		// A typical iframe packet from a 320x240 camera is around 100 bytes!
		// A keyframe is between 10 and 20 KB.
		cloned := videox.ClonePacket(nalus, pts, now, refTime)
		cloned.RecvID = myPacketID

		// Frame size stats can be interesting
		//if s.Ident == "driveway.low" {
		//	fmt.Printf("Stream %v: Received packet %v. Size %v\n", s.Ident, cloned.RecvID, cloned.PayloadBytes())
		//}

		// Obtain the sinks lock, so that we can't send packets after a Close message has been sent.
		s.sinksLock.Lock()
		if !s.isClosed {
			for _, sink := range s.sinks {
				a := time.Now()
				s.sendSinkMsg(sink, StreamMsgTypePacket, cloned)
				elapsed := time.Now().Sub(a)
				if elapsed > time.Millisecond {
					s.Log.Warnf("Slow stream sink (%v)", elapsed)
				}
			}
		}
		s.sinksLock.Unlock()
	})

	// start reading tracks
	//err = client.SetupAndPlay(tracks, baseURL)
	_, err = client.Play(nil)
	if err != nil {
		return fmt.Errorf("Stream Play failed: %w", err)
	}

	s.Log.Infof("Connection to %v success", camHost)

	return nil
}

// Close the stream.
// If wg is not nil, then you must call wg.Done() once all of your sinks have closed themselves.
func (s *Stream) Close(wg *sync.WaitGroup) {
	s.Log.Infof("Closing stream")

	if s.Client != nil {
		s.Client.Close()
	}

	// Obtain the sinks lock, so that we can't send packets after a Close message has been sent.
	s.sinksLock.Lock()
	s.isClosed = true
	if wg != nil {
		// Every time a sink removes itself, we'll remove it from the wait group
		s.closeWG = wg
		s.Log.Debugf("Adding %v sinks to Stream waitgroup", len(s.sinks))
		wg.Add(len(s.sinks))
	}
	for _, sink := range s.sinks {
		s.sendSinkMsg(sink, StreamMsgTypeClose, nil)
	}
	s.sinksLock.Unlock()
}

func (s *Stream) extractSPSInfo(nalus [][]byte) *StreamInfo {
	for _, nalu := range nalus {
		if len(nalu) == 0 {
			continue
		}
		if h264.NALUType(nalu[0]&31) == h264.NALUTypeSPS {
			width, height, err := videox.ParseSPS(nalu)
			if err != nil {
				s.Log.Errorf("Failed to decode SPS: %v", err)
			}
			return &StreamInfo{
				Width:  width,
				Height: height,
			}
		}
	}
	return nil
}

// Return the stream info, or nil if we have not yet encountered the necessary NALUs
func (s *Stream) Info() *StreamInfo {
	s.infoLock.Lock()
	defer s.infoLock.Unlock()
	return s.info
}

// Estimate the frame rate
func (s *Stream) RecentFrameStats() StreamStats {
	s.recentFramesLock.Lock()
	defer s.recentFramesLock.Unlock()

	return s.statsNoMutexLock()
}

// Return the wall time of the most recently received packet
func (s *Stream) LastPacketReceivedAt() time.Time {
	s.livenessLock.Lock()
	defer s.livenessLock.Unlock()
	return s.livenessLastPacketReceivedAt
}

func (s *Stream) statsNoMutexLock() StreamStats {
	stats := StreamStats{}
	if s.recentFrames.Len() < 2 {
		return stats
	}
	count := s.recentFrames.Len()
	oldest := s.recentFrames.Peek(0)
	latest := s.recentFrames.Peek(count - 1)
	elapsed := latest.pts.Seconds() - oldest.pts.Seconds()
	stats.FPS = float64(count-1) / elapsed
	nIDR := 0
	nInter := 0
	for i := 0; i < count; i++ {
		frame := s.recentFrames.Peek(i)
		frameSize := float64(frame.size)
		stats.FrameSize += frameSize
		if frame.isIDR {
			stats.KeyFrameSize += frameSize
			nIDR++
		} else {
			stats.InterFrameSize += frameSize
			nInter++
		}
	}
	stats.FrameSize /= float64(count)
	stats.KeyFrameSize /= float64(nIDR)
	stats.InterFrameSize /= float64(nInter)
	return stats
}

// Connect a sink.
//
// Every call to ConnectSink must be accompanies by a call to RemoveSink.
// The usual time to do this is when receiving StreamMsgTypeClose.
//
// This function will panic if you attempt to add the same sink twice.
func (s *Stream) ConnectSink(sink StreamSinkChan) {
	s.sinksLock.Lock()
	defer s.sinksLock.Unlock()
	if s.isClosed {
		return
	}
	s.connectSinkNoLock(sink)
}

func (s *Stream) connectSinkNoLock(sink StreamSinkChan) {
	if s.sinkIndexNoLock(sink) != -1 {
		panic("sink has already been connected to stream")
	}
	s.sinks = append(s.sinks, sink)
}

// Connect a standard sink object and run it.
//
// You don't need to call RemoveSink when using ConnectSinkAndRun.
// When RunStandardStream exits, it will call RemoveSink for you.
func (s *Stream) ConnectSinkAndRun(sink StandardStreamSink) error {
	s.sinksLock.Lock()
	defer s.sinksLock.Unlock()
	if s.isClosed {
		return fmt.Errorf("Stream is already closed")
	} else {
		sinkChan, err := sink.OnConnect(s)
		if err != nil {
			return err
		}

		s.connectSinkNoLock(sinkChan)

		go RunStandardStream(s, sink, sinkChan)

		return nil
	}
}

// Remove a sink
func (s *Stream) RemoveSink(sink StreamSinkChan) {
	s.sinksLock.Lock()
	defer s.sinksLock.Unlock()
	idx := s.sinkIndexNoLock(sink)
	if idx != -1 {
		s.sinks = gen.DeleteFromSliceOrdered(s.sinks, idx)
		if s.closeWG != nil {
			s.closeWG.Done()
		}
	}
}

func (s *Stream) sendSinkMsg(sink StreamSinkChan, msgType StreamMsgType, packet *videox.VideoPacket) {
	sink <- StreamMsg{
		Type:   msgType,
		Stream: s,
		Packet: packet,
	}
}

// NOTE: This function does not take sinksLock, but assumes you have already done so
func (s *Stream) sinkIndexNoLock(sink StreamSinkChan) int {
	for i := 0; i < len(s.sinks); i++ {
		if s.sinks[i] == sink {
			return i
		}
	}
	return -1
}

func (s *Stream) addFrameToStats(nalus [][]byte, pts time.Duration) {
	s.recentFramesLock.Lock()
	defer s.recentFramesLock.Unlock()

	for _, nalu := range nalus {
		nn := videox.WrapRawNALU(nalu)
		nnType := nn.Type()
		if videox.IsVisualPacket(nnType) {
			s.recentFrames.Add(frameStat{
				isIDR: nnType == h264.NALUTypeIDR,
				pts:   pts,
				size:  len(nalu),
			})
		}
	}

	if s.loggedStatsAt.IsZero() && s.recentFrames.Len() >= s.recentFrames.Capacity() {
		s.loggedStatsAt = time.Now()
		stats := s.statsNoMutexLock()
		s.Log.Infof("FPS: %.1f, Avg: %.0f, IDR: %.0f, Non-IDR: %.0f", stats.FPS, stats.FrameSize, stats.KeyFrameSize, stats.InterFrameSize)
	}
}
