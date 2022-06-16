package camera

import (
	"github.com/bmharper/cyclops/server/log"
)

// Camera represents a single physical camera, with two streams (high and low res)
type Camera struct {
	Name       string
	Log        log.Log
	LowRes     *Stream
	HighRes    *Stream
	lowResURL  string
	highResURL string
}

func NewCamera(name string, log log.Log, lowResURL, highResURL string) (*Camera, error) {
	highReader := &VideoDumpReader{}
	lowReader := &VideoDecodeReader{}
	high := NewStream(log, highReader)
	low := NewStream(log, lowReader)

	return &Camera{
		Name:       name,
		Log:        log,
		LowRes:     low,
		HighRes:    high,
		lowResURL:  lowResURL,
		highResURL: highResURL,
	}, nil
}

func (c *Camera) Start() error {
	if err := c.HighRes.Listen(c.highResURL); err != nil {
		return err
	}
	if err := c.LowRes.Listen(c.lowResURL); err != nil {
		return err
	}
	return nil
}

func (c *Camera) Close() {
	if c.LowRes != nil {
		c.LowRes.Close()
	}
	if c.HighRes != nil {
		c.HighRes.Close()
	}
}
