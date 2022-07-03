package camera

import (
	"time"

	"github.com/bmharper/cimg/v2"
	"github.com/bmharper/cyclops/server/log"
	"github.com/bmharper/cyclops/server/videox"
)

type Resolution int

const (
	ResolutionHigh Resolution = iota
	ResolutionLow
)

// Camera represents a single physical camera, with two streams (high and low res)
type Camera struct {
	Name       string
	Log        log.Log
	LowStream  *Stream
	HighStream *Stream
	HighDumper *VideoDumpReader
	LowDecoder *VideoDecodeReader
	lowResURL  string
	highResURL string
}

func NewCamera(name string, log log.Log, lowResURL, highResURL string, ringBufferSizeBytes int) (*Camera, error) {
	highDumper := NewVideoDumpReader(ringBufferSizeBytes)
	lowDecoder := NewVideoDecodeReader()
	high := NewStream(log, name, "high")
	low := NewStream(log, name, "low")

	return &Camera{
		Name:       name,
		Log:        log,
		LowStream:  low,
		HighStream: high,
		HighDumper: highDumper,
		LowDecoder: lowDecoder,
		lowResURL:  lowResURL,
		highResURL: highResURL,
	}, nil
}

func (c *Camera) Start() error {
	if err := c.HighStream.Listen(c.highResURL); err != nil {
		return err
	}
	if err := c.LowStream.Listen(c.lowResURL); err != nil {
		return err
	}
	if err := c.HighStream.ConnectSinkAndRun(c.HighDumper); err != nil {
		return err
	}
	if err := c.LowStream.ConnectSinkAndRun(c.LowDecoder); err != nil {
		return err
	}
	return nil
}

func (c *Camera) Close() {
	if c.LowStream != nil {
		c.LowStream.Close()
		c.LowStream = nil
	}
	if c.HighStream != nil {
		c.HighStream.Close()
		c.HighStream = nil
	}
}

func (c *Camera) LatestImage(contentType string) []byte {
	img := c.LowDecoder.LastImage()
	if img == nil {
		return nil
	}
	img2, err := cimg.FromImage(img, true)
	if err != nil {
		c.Log.Errorf("Failed to wrap decoded image into cimg: %v", err)
		return nil
	}
	buf, err := cimg.Compress(img2, cimg.MakeCompressParams(cimg.Sampling(cimg.Sampling420), 85, cimg.Flags(0)))
	if err != nil {
		c.Log.Errorf("Failed to compress image: %v", err)
		return nil
	}
	return buf
}

// Extract from <now - duration> until <now>.
// duration is a positive number.
func (c *Camera) ExtractHighRes(method ExtractMethod, duration time.Duration) (*videox.RawBuffer, error) {
	return c.HighDumper.ExtractRawBuffer(method, duration)
}

// Get either the high or low resolution stream
func (c *Camera) GetStream(resolution Resolution) *Stream {
	switch resolution {
	case ResolutionLow:
		return c.LowStream
	case ResolutionHigh:
		return c.HighStream
	}
	return nil
}
