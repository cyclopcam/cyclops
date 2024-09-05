package camera

import (
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format/rtph264"
	"github.com/bluenviron/gortsplib/v4/pkg/format/rtph265"
	"github.com/bluenviron/mediacommon/pkg/codecs/h264"
	"github.com/bmharper/ringbuffer"
	"github.com/cyclopcam/cyclops/pkg/gen"
	"github.com/cyclopcam/cyclops/pkg/videox"
	"github.com/cyclopcam/logs"
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
// UPDATE: While running on RPi5, 2 seems to be too small. We frequently
// get blocking on various stream sinks. Specifically, I'm seeing blocking
// of up to 4ms on these sinks: 'LD Decode', 'HD Ring'.
// So I'm raising the size of this buffer from 2 to 4.
// Blocking when sending to these sinks is very bad, because if you do it
// enough, you end up losing incoming camera packets.
// 4 is not enough. Trying 10.
const StreamSinkChanDefaultBufferSize = 10

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

type streamSink struct {
	sink StreamSinkChan
	name string // for debugging
}

type packetDecoder interface {
	Decode(pkt *rtp.Packet) ([][]byte, error)
}

// Stream is a bridge between the RTSP library (gortsplib) and one or more "sink" objects.
// The stream understands just enough about RTSP and video codecs to be able to receive
// information from gortsplib, transform them into our own internal data structures,
// and pass them onto the sinks.
// For each camera, we create one stream to handle the high res video, and another stream
// for the low res video.
type Stream struct {
	Log        logs.Log
	Client     *gortsplib.Client
	Ident      string // Just for logs. Simply CameraName.StreamName.
	CameraName string // Just for logs
	StreamName string // The stream name, such as "low" and "high"
	Codec      videox.Codec

	// These are read at the start of Listen(), and will be populated before Listen() returns
	//H264TrackID int                  // 0-based track index
	//H264Track   *gortsplib.TrackH264 // track object

	sinksLock sync.Mutex
	sinks     []streamSink
	closeWG   *sync.WaitGroup
	isClosed  bool

	infoLock sync.Mutex
	info     *StreamInfo // With Go 1.19 one could use atomic.Pointer[T] here

	livenessLock                 sync.Mutex
	livenessLastPacketReceivedAt time.Time

	// Used to infer real time from packet's relative timestamps
	refTimeWall         time.Time
	refTimeCameraOffset time.Duration

	recentFramesLock sync.Mutex
	recentFrames     ringbuffer.RingP[frameStat] // Some stats of recent frames (eg PTS, size)
	loggedStatsAt    time.Time

	// If true, the camera sends packets that are already Annex-B encoded.
	// This is independent of whether the packets have start codes or not. I only have
	// experience so far with Hikvision cameras, and they send packets that are already
	// Annex-B encoded, but without start codes.
	// If you don't know this information up front, then the only way to determine it is
	// by analyzing many packets, and building up heuristics. Annex-B "emulation prevention bytes"
	// don't occur on all packets, so there's no way to know this deterministically by
	// analyzing a single packet. In future, we might have to add some kind of automatic
	// detection mechanism.
	cameraSendsAnnexBEncoded bool
}

func NewStream(logger logs.Log, cameraName, streamName string, cameraSendsAnnexBEncoded bool) *Stream {
	return &Stream{
		Log:                      logs.NewPrefixLogger(logger, "Stream "+cameraName+"."+streamName),
		recentFrames:             ringbuffer.NewRingP[frameStat](128),
		CameraName:               cameraName,
		StreamName:               streamName,
		Ident:                    cameraName + "." + streamName,
		cameraSendsAnnexBEncoded: cameraSendsAnnexBEncoded,
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

	// find the H264/H265 track
	var formaH264 *format.H264
	var formaH265 *format.H265
	var forma format.Format
	var media *description.Media
	var rtpDecoder packetDecoder
	media = session.FindFormat(&formaH265)
	if media != nil {
		forma = formaH265
		s.Codec = videox.CodecH265
		rtpDecoder, err = formaH265.CreateDecoder()
		if err != nil {
			return fmt.Errorf("Failed to create H265 decoder: %w", err)
		}
	}
	if media == nil {
		media = session.FindFormat(&formaH264)
		if media != nil {
			forma = formaH264
			rtpDecoder, err = formaH264.CreateDecoder()
			s.Codec = videox.CodecH264
			if err != nil {
				return fmt.Errorf("Failed to create H264 decoder: %w", err)
			}
		}
	}
	if media == nil {
		return fmt.Errorf("H264/H265 track not found")
	}

	client.Setup(session.BaseURL, media, 0, 0)

	s.Log.Infof("Connected to %v, track %v, codec %v", camHost, media.ID, s.Codec)

	rawRecvID := atomic.Int64{}
	validRecvID := atomic.Int64{}
	nWarningsAboutNoPTS := 0

	// Two camera debugging flags, not used normally.
	// Used to figure out if a camera is sending packets that already have "emulation prevention bytes" added,
	// even though they don't have start codes.
	enableFindHiddenAnnexBPackets := false

	// If you enable this, then you'd also want to set "isPayloadAnnexBEncoded = false"
	enableForceAnnexBDecode := false

	client.OnPacketRTP(media, forma, func(pkt *rtp.Packet) {
		now := time.Now()
		myRawPacketID := rawRecvID.Add(1)

		s.livenessLock.Lock()
		s.livenessLastPacketReceivedAt = now
		s.livenessLock.Unlock()

		//if s.CameraName == "Driveway" && s.StreamName == "high" && s.info == nil {
		//	s.Log.Infof("Received packet %v (%v bytes)", myPacketID, len(pkt.Payload))
		//}

		pts, ok := client.PacketPTS(media, pkt)
		if !ok {
			if nWarningsAboutNoPTS == 0 {
				s.Log.Warnf("Ignoring %v packet without PTS", s.Codec)
			}
			nWarningsAboutNoPTS = min(nWarningsAboutNoPTS+1, 1000)
			return
		}

		nWarningsAboutNoPTS >>= 1

		// I don't seem to be getting NTP info from my Hikvision cameras
		//ntp, ntpOK := client.PacketNTP(media, pkt)

		nalus, err := rtpDecoder.Decode(pkt)
		if err != nil {
			if err != rtph264.ErrNonStartingPacketAndNoPrevious && err != rtph264.ErrMorePacketsNeeded &&
				err != rtph265.ErrNonStartingPacketAndNoPrevious && err != rtph265.ErrMorePacketsNeeded {
				s.Log.Errorf("Failed to decode %v packet: %v", s.Codec, err)
			}
			return
		}

		// These are debugging/camera flags, not usually enabled
		if enableFindHiddenAnnexBPackets {
			s.findHiddenAnnexBPackets(nalus)
		}
		if enableForceAnnexBDecode {
			s.forceAnnexBDecode(nalus)
		}

		myValidPacketID := validRecvID.Add(1)

		// Note that gortsplib also has client.PacketNTP(), which we could experiment with.
		// Perhaps we should measure NTP time from the camera, and if its close enough to our
		// perceived time, then use the camera's time.

		// establish reference time
		if s.refTimeWall.IsZero() && len(nalus) != 0 {
			s.refTimeWall = now
			s.refTimeCameraOffset = pts
		}

		// compute absolute PTS
		refTime := now
		if !s.refTimeWall.IsZero() {
			refTime = s.refTimeWall.Add(pts - s.refTimeCameraOffset)
			//if ntpOK {
			//	fmt.Printf("ntp: %v, refTime: %v\n", ntp, refTime)
			//}
		}

		// Populate width & height
		s.infoLock.Lock()
		if s.info == nil {
			if inf := s.extractSPSInfo(nalus); inf != nil {
				s.info = inf
				s.Log.Infof("Size: %v x %v (after %v packets)", inf.Width, inf.Height, myValidPacketID)
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
		// A typical iframe packet from a 320x240 camera is around 100 bytes!
		// A keyframe is between 10 and 20 KB.
		// NOTE: I gortsplib may have changed that memory re-use behaviour since
		// I wrote this. Should investigate again...
		cloned := videox.ClonePacket(nalus, pts, now, refTime, s.cameraSendsAnnexBEncoded)
		cloned.RawRecvID = myRawPacketID
		cloned.ValidRecvID = myValidPacketID

		// Frame size stats can be interesting
		//if s.Ident == "driveway.low" {
		//	fmt.Printf("Stream %v: Received packet %v. Size %v\n", s.Ident, cloned.RecvID, cloned.PayloadBytes())
		//}

		// Obtain the sinks lock, so that we can't send packets after a Close message has been sent.
		s.sinksLock.Lock()
		if !s.isClosed {
			for _, sink := range s.sinks {
				a := time.Now()
				s.sendSinkMsg(sink.sink, StreamMsgTypePacket, cloned)
				elapsed := time.Now().Sub(a)
				if elapsed > 5*time.Millisecond {
					// On my Rpi5, 5ms is a normal delay here. I suspect it's the NCNN threads hogging the CPU
					// On my Ryzen, times are always below 1ms.
					s.Log.Warnf("Slow stream sink '%v' (%v)", sink.name, elapsed)
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
		s.sendSinkMsg(sink.sink, StreamMsgTypeClose, nil)
	}
	s.sinksLock.Unlock()
}

func (s *Stream) extractSPSInfo(nalus [][]byte) *StreamInfo {
	for _, nalu := range nalus {
		if len(nalu) == 0 {
			continue
		}
		if s.Codec == videox.CodecH264 {
			if h264.NALUType(nalu[0]&31) == h264.NALUTypeSPS {
				width, height, err := videox.ParseH264SPS(nalu)
				if err != nil {
					s.Log.Errorf("Failed to decode SPS: %v", err)
				}
				return &StreamInfo{
					Width:  width,
					Height: height,
				}
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
func (s *Stream) ConnectSink(name string, sink StreamSinkChan) {
	s.sinksLock.Lock()
	defer s.sinksLock.Unlock()
	if s.isClosed {
		return
	}
	s.connectSinkNoLock(streamSink{name: name, sink: sink})
}

func (s *Stream) connectSinkNoLock(sink streamSink) {
	if s.sinkIndexNoLock(sink.sink) != -1 {
		panic("sink has already been connected to stream")
	}
	s.sinks = append(s.sinks, sink)
}

// Connect a standard sink object and run it.
//
// You don't need to call RemoveSink when using ConnectSinkAndRun.
// When RunStandardStream exits, it will call RemoveSink for you.
func (s *Stream) ConnectSinkAndRun(name string, sink StandardStreamSink) error {
	s.sinksLock.Lock()
	defer s.sinksLock.Unlock()
	if s.isClosed {
		return fmt.Errorf("Stream is already closed")
	} else {
		sinkChan, err := sink.OnConnect(s)
		if err != nil {
			return err
		}

		s.connectSinkNoLock(streamSink{name: name, sink: sinkChan})

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
		if s.sinks[i].sink == sink {
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
		s.Log.Infof("FPS: %.1f, Avg: %.0f, Keyframe: %.0f, Intra: %.0f, Rate: %.0f KB/s", stats.FPS, stats.FrameSize, stats.KeyFrameSize, stats.InterFrameSize, stats.FrameSize*stats.FPS/1024)
	}
}

func (s *Stream) findHiddenAnnexBPackets(nalus [][]byte) {
	for _, p := range nalus {
		//ds := videox.DecodeAnnexBSize(p)
		//if ds != len(p) {
		//	s.Log.Warnf("Annex-B packet found. Size: %v, Decoded size: %v. Prefix: %x", len(p), ds, p[:10])
		//}
		if i := videox.FirstLikelyAnnexBEncodedIndex(p); i != -1 {
			a := max(i-3, 0)
			b := min(i+6, len(p))
			s.Log.Warnf("Likely Annex-B packet. size: %v, byte %v: ..%x..", len(p), i, p[a:b])
		}
	}
}

func (s *Stream) forceAnnexBDecode(nalus [][]byte) {
	for i := range nalus {
		nalus[i] = videox.DecodeAnnexB(nalus[i])
	}
}
