package camera

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/bmharper/cyclops/server/gen"
	"github.com/bmharper/cyclops/server/log"
	"github.com/bmharper/cyclops/server/videox"
	"github.com/bmharper/ringbuffer"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/h264"
	"github.com/aler9/gortsplib/pkg/liberrors"
	"github.com/aler9/gortsplib/pkg/url"
)

// A stream sink is fundamentally just a channel
type StreamSinkChan chan StreamMsg

// StandardStreamSink allows you to run the stream with RunStandardStream()
// This is really just a convenience wrapper around StreamSinkChan.
type StandardStreamSink interface {
	// OnConnect is called by Stream.ConnectSinkAndRun().
	// You must return a channel to which all stream messages will be sent.
	OnConnect(stream *Stream) (StreamSinkChan, error)
	OnPacketRTP(packet *videox.DecodedPacket) // Called by RunStandardStream(), when it receives a StreamMsgTypePacket
	Close()                                   // Called by RunStandardStream(), when it receives a StreamMsgTypeClose
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
	Packet *videox.DecodedPacket
}

// There isn't much rhyme or reason behind this number
const StreamSinkChanDefaultBufferSize = 2

type StreamInfo struct {
	Width  int
	Height int
}

type Stream struct {
	Log        log.Log
	Client     *gortsplib.Client
	Ident      string // Just for logs. Simply CameraName.StreamName.
	CameraName string // Just for logs
	StreamName string // Just for logs

	// These are read at the start of Listen(), and will be populated before Listen() returns
	H264TrackID int                  // 0-based track index
	H264Track   *gortsplib.TrackH264 // track object

	sinksLock sync.Mutex
	sinks     []StreamSinkChan
	closeWG   *sync.WaitGroup
	isClosed  bool

	infoLock sync.Mutex
	info     *StreamInfo // With Go 1.19 one could use atomic.Pointer[T] here

	recentFramesLock sync.Mutex
	recentFrames     ringbuffer.RingP[time.Duration]
	loggedFPS        bool
}

func NewStream(logger log.Log, cameraName, streamName string) *Stream {
	return &Stream{
		Log:          log.NewPrefixLogger(logger, "Stream "+cameraName+"."+streamName),
		recentFrames: ringbuffer.NewRingP[time.Duration](64),
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
	u, err := url.Parse(address)
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
	tracks, baseURL, _, err := client.Describe(u)
	if err != nil {
		if e, ok := err.(liberrors.ErrClientBadStatusCode); ok {
			if e.Code == 401 {
				return fmt.Errorf("Invalid username or password")
			}
		}
		return err
	}

	// find the H264 track
	h264TrackID, h264track := func() (int, *gortsplib.TrackH264) {
		for i, track := range tracks {
			if h264track, ok := track.(*gortsplib.TrackH264); ok {
				return i, h264track
			}
		}
		return -1, nil
	}()
	if h264TrackID < 0 {
		return fmt.Errorf("H264 track not found")
	}
	s.H264TrackID = h264TrackID
	s.H264Track = h264track
	s.Log.Infof("Connected to %v, track %v", camHost, h264TrackID)

	client.OnPacketRTP = func(ctx *gortsplib.ClientOnPacketRTPCtx) {
		if ctx.TrackID != h264TrackID || len(ctx.H264NALUs) == 0 {
			return
		}

		now := time.Now()

		// Populate width & height.
		s.infoLock.Lock()
		if s.info == nil {
			if inf := s.extractSPSInfo(ctx.H264NALUs); inf != nil {
				s.info = inf
				s.Log.Infof("Size: %v x %v", inf.Width, inf.Height)
			}
		}
		s.infoLock.Unlock()

		s.countFrames(ctx)

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
		cloned := videox.ClonePacket(ctx, now)

		// Obtain the sinks lock, so that we can't send packets after a Close message has been sent.
		s.sinksLock.Lock()
		if !s.isClosed {
			for _, sink := range s.sinks {
				s.sendSinkMsg(sink, StreamMsgTypePacket, cloned)
			}
		}
		s.sinksLock.Unlock()
	}

	// start reading tracks
	err = client.SetupAndPlay(tracks, baseURL)
	if err != nil {
		return fmt.Errorf("Stream SetupAndPlay failed: %w", err)
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
		s.Log.Debugf("Adding %v to Stream waitgroup", len(s.sinks))
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
func (s *Stream) FPSFloat() float64 {
	s.recentFramesLock.Lock()
	defer s.recentFramesLock.Unlock()

	return s.fpsNoMutexLock()
}

// Estimate the frame rate
func (s *Stream) FPS() int {
	return int(math.Round(s.FPSFloat()))
}

func (s *Stream) fpsNoMutexLock() float64 {
	if s.recentFrames.Len() < 2 {
		return 10
	}
	count := s.recentFrames.Len()
	oldest := s.recentFrames.Peek(0)
	latest := s.recentFrames.Peek(count - 1)
	elapsed := latest.Seconds() - oldest.Seconds()
	return float64(count-1) / elapsed
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

func (s *Stream) sendSinkMsg(sink StreamSinkChan, msgType StreamMsgType, packet *videox.DecodedPacket) {
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

func (s *Stream) countFrames(ctx *gortsplib.ClientOnPacketRTPCtx) {
	s.recentFramesLock.Lock()
	defer s.recentFramesLock.Unlock()

	for _, nalu := range ctx.H264NALUs {
		if len(nalu) > 0 {
			t := h264.NALUType(nalu[0] & 31)
			if videox.IsVisualPacket(t) {
				s.recentFrames.Add(ctx.H264PTS)
			}
		}
	}

	if !s.loggedFPS && s.recentFrames.Len() >= s.recentFrames.Capacity() {
		s.loggedFPS = true
		s.Log.Infof("FPS: %.3f", s.fpsNoMutexLock())
	}
}
