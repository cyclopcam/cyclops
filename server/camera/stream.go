package camera

import (
	"fmt"

	"github.com/bmharper/cyclops/server/log"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/url"
)

type Stream struct {
	Log    log.Log
	Reader VideoReader
	Client gortsplib.Client
	Ident  string // Identity of stream, with username:password stripped out
}

func NewStream(log log.Log, reader VideoReader) *Stream {
	return &Stream{
		Log:    log,
		Reader: reader,
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
	s.Log.Infof("Connected to %v, track %v", s.Ident, h264TrackID)

	if err := s.Reader.Initialize(s.Log, h264TrackID, h264track); err != nil {
		return err
	}

	client.OnPacketRTP = func(ctx *gortsplib.ClientOnPacketRTPCtx) {
		s.Reader.OnPacketRTP(ctx)
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
	if s.Reader != nil {
		s.Reader.Close()
	}
}
