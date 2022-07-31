package camera

import (
	"errors"
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

// StreamSink receives packets from the stream on its channel
// There can be multiple StreamSinks connected to a Stream
type StreamSink interface {
	OnConnect(stream *Stream) (StreamSinkChan, error) // Called by Stream.ConnectSink()
}

// StandardStreamSink allows you to run the stream with RunStandardStream()
type StandardStreamSink interface {
	StreamSink
	OnPacketRTP(ctx *gortsplib.ClientOnPacketRTPCtx)
	Close()
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
	Packet *gortsplib.ClientOnPacketRTPCtx
}

// Once a sink is connected to a stream, all messages to the sink are sent via this channel
type StreamSinkChan chan StreamMsg

// There isn't much rhyme or reason behind this number
const StreamSinkChanDefaultBufferSize = 2

// Internal sink data structure of Stream
type sinkObj struct {
	sink StreamSink
	ch   StreamSinkChan
}

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
	sinks     []sinkObj

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
		// Copy the sinks out, to be safe during stream Close()
		s.sinksLock.Lock()
		sinks := gen.CopySlice(s.sinks)
		s.sinksLock.Unlock()

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

		//s.Log.Infof("Packet %v", ctx.H264PTS)
		for _, sink := range sinks {
			//sink.OnPacketRTP(ctx)
			s.sendSinkMsg(sink.ch, StreamMsgTypePacket, ctx)
		}
	}

	// start reading tracks
	err = client.SetupAndPlay(tracks, baseURL)
	if err != nil {
		return fmt.Errorf("Stream SetupAndPlay failed: %w", err)
	}

	s.Log.Infof("Connection to %v success", camHost)

	return nil
}

func (s *Stream) Close() {
	s.Log.Infof("Closing stream")

	if s.Client != nil {
		s.Client.Close()
	}

	s.sinksLock.Lock()
	sinks := gen.CopySlice(s.sinks)
	s.sinks = []sinkObj{}
	s.sinksLock.Unlock()

	//s.Log.Infof("Closing stream - sending StreamMsgTypeClose")
	for _, sink := range sinks {
		s.sendSinkMsg(sink.ch, StreamMsgTypeClose, nil)
	}
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

// Connect a sink
// If runStandardHandler is true, then we cast sink to StandardStreamSink, and
// start a new goroutine that runs RunStandardStream() on this stream.
// If runStandardHandler is false, then you must run a message loop like
// RunStandardStream yourself.
func (s *Stream) ConnectSink(sink StreamSink, runStandardHandler bool) error {
	sinkChan, err := sink.OnConnect(s)
	if err != nil {
		return err
	}

	s.sinksLock.Lock()
	s.sinks = append(s.sinks, sinkObj{
		sink: sink,
		ch:   sinkChan,
	})
	s.sinksLock.Unlock()

	if runStandardHandler {
		if standard, ok := sink.(StandardStreamSink); ok {
			go RunStandardStream(sinkChan, standard)
		} else {
			return errors.New("sink does not implement StandardStreamSink, so you can't use runStandardHandler = true")
		}
	}

	return nil
}

// This is just an explicitly typed wrapper around ConnectSink(sink, true)
func (s *Stream) ConnectSinkAndRun(sink StandardStreamSink) error {
	return s.ConnectSink(sink, true)
}

func (s *Stream) RemoveSink(sink StreamSink) {
	s.sinksLock.Lock()
	idx := s.sinkIndex(sink)
	if idx != -1 {
		s.sinks = append(s.sinks[0:idx], s.sinks[idx+1:]...)
	}
	s.sinksLock.Unlock()
}

func (s *Stream) sendSinkMsg(sink StreamSinkChan, msgType StreamMsgType, packet *gortsplib.ClientOnPacketRTPCtx) {
	sink <- StreamMsg{
		Type:   msgType,
		Stream: s,
		Packet: packet,
	}
}

// NOTE: This function does not take sinksLock, but assumes you have already done so
func (s *Stream) sinkIndex(sink StreamSink) int {
	for i := 0; i < len(s.sinks); i++ {
		if s.sinks[i].sink == sink {
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
