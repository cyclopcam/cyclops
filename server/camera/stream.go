package camera

import (
	"errors"
	"fmt"
	"sync"

	"github.com/bmharper/cyclops/server/gen"
	"github.com/bmharper/cyclops/server/log"

	"github.com/aler9/gortsplib"
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
	StreamMsgTypePacket StreamMsgType = iota
	StreamMsgTypeClose
)

type StreamMsg struct {
	Type   StreamMsgType
	Stream *Stream
	Packet *gortsplib.ClientOnPacketRTPCtx
}

type StreamSinkChan chan StreamMsg

// There isn't much rhyme or reason behind this number
const StreamSinkChanDefaultBufferSize = 2

// Internal sink data structure of Stream
type sinkObj struct {
	sink StreamSink
	ch   StreamSinkChan
}

type Stream struct {
	Log    log.Log
	Client gortsplib.Client
	Ident  string // Identity of stream, with username:password stripped out

	// These are read at the start of Listen(), and will be populated before Listen() returns
	H264TrackID int                  // 0-based track index
	H264Track   *gortsplib.TrackH264 // track object

	sinksLock sync.Mutex
	sinks     []sinkObj
}

func NewStream(log log.Log) *Stream {
	return &Stream{
		Log: log,
	}
}

func (s *Stream) Listen(address string) error {
	s.Client = gortsplib.Client{}
	client := &s.Client

	// parse URL
	u, err := url.Parse(address)
	if err != nil {
		return fmt.Errorf("Invalid stream URL: %w", err)
	}
	s.Ident = u.Host + u.Path

	// connect to the server
	s.Log.Infof("Connecting to %v", s.Ident)
	err = client.Start(u.Scheme, u.Host)
	if err != nil {
		return fmt.Errorf("Failed to start stream: %w", err)
	}

	// find published tracks
	tracks, baseURL, _, err := client.Describe(u)
	if err != nil {
		panic(err)
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
	s.Log.Infof("Connected to %v, track %v", s.Ident, h264TrackID)

	//if err := s.Reader.Initialize(s.Log, h264TrackID, h264track); err != nil {
	//	return err
	//}

	client.OnPacketRTP = func(ctx *gortsplib.ClientOnPacketRTPCtx) {
		//s.Reader.OnPacketRTP(ctx)
		// We hold sinksLock for the entire duration of the packet here,
		// to ensure that we don't have races when Close() is called.
		// Imagine an h264 decoder has already been destroyed by Close(),
		// and then we call OnPacketRTP on that sink.
		s.sinksLock.Lock()
		sinks := gen.CopySlice(s.sinks)
		s.sinksLock.Unlock()

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

	s.Log.Infof("Connection to %v success", s.Ident)

	// wait until a fatal error
	//panic(c.Wait())
	return nil
}

func (s *Stream) Close() {
	s.Log.Infof("Closing stream %v", s.Ident)

	s.Client.Close()

	s.sinksLock.Lock()
	sinks := gen.CopySlice(s.sinks)
	s.sinks = []sinkObj{}
	s.sinksLock.Unlock()

	for _, sink := range sinks {
		s.sendSinkMsg(sink.ch, StreamMsgTypeClose, nil)
	}
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
		Type:   StreamMsgTypePacket,
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
