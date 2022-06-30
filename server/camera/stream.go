package camera

import (
	"fmt"
	"sync"

	"github.com/bmharper/cyclops/server/log"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/url"
)

// StreamSink receives packets from the stream
// There can be multiple StreamSinks connected to a Stream
type StreamSink interface {
	OnConnect(stream *Stream) error // Called by Stream.ConnectSink()
	OnPacketRTP(ctx *gortsplib.ClientOnPacketRTPCtx)
	Close()
}

type Stream struct {
	Log    log.Log
	Client gortsplib.Client
	Ident  string // Identity of stream, with username:password stripped out

	// These are read at the start of Listen(), and will be populated before Listen() returns
	H264TrackID int                  // 0-based track index
	H264Track   *gortsplib.TrackH264 // track object

	sinksLock sync.Mutex
	sinks     []StreamSink
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
		for _, sink := range s.sinks {
			sink.OnPacketRTP(ctx)
		}
		s.sinksLock.Unlock()
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
	//if s.Reader != nil {
	//	s.Reader.Close()
	//}

	s.sinksLock.Lock()
	for _, sink := range s.sinks {
		sink.Close()
	}
	s.sinks = []StreamSink{}
	s.sinksLock.Unlock()
}

func (s *Stream) ConnectSink(sink StreamSink) error {
	s.sinksLock.Lock()
	defer s.sinksLock.Unlock()
	if err := sink.OnConnect(s); err != nil {
		return err
	}
	s.sinks = append(s.sinks, sink)
	return nil
}

func (s *Stream) RemoveSink(sink StreamSink) {
	s.sinksLock.Lock()
	defer s.sinksLock.Unlock()
	s.sinks = DeleteFirstStreamSink(s.sinks, sink)
	//s.sinks = gen.DeleteFirst[StreamSink](s.sinks, sink) // see DeleteFirstStreamSink
}

// This is copied from our generic DeleteFirst. I don't understand why StreamSink is not comparable
func DeleteFirstStreamSink(slice []StreamSink, elem StreamSink) []StreamSink {
	for i := 0; i < len(slice); i++ {
		if slice[i] == elem {
			return append(slice[0:i], slice[i+1:]...)
		}
	}
	return slice
}
