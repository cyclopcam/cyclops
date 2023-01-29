package camera

import (
	"fmt"
	"sync"
	"time"

	"github.com/bmharper/cimg/v2"
	"github.com/bmharper/cyclops/pkg/log"
	"github.com/bmharper/cyclops/server/configdb"
	"github.com/bmharper/cyclops/server/defs"
	"github.com/bmharper/cyclops/server/videox"
)

// Camera represents a single physical camera, with two streams (high and low res)
type Camera struct {
	ID         int64 // Same as ID in database
	Name       string
	Log        log.Log
	LowStream  *Stream
	HighStream *Stream
	HighDumper *VideoDumpReader
	LowDecoder *VideoDecodeReader
	LowDumper  *VideoDumpReader
	lowResURL  string
	highResURL string
}

func NewCamera(log log.Log, cam configdb.Camera, ringBufferSizeBytes int) (*Camera, error) {
	baseURL := "rtsp://" + cam.Username + ":" + cam.Password + "@" + cam.Host
	if cam.Port == 0 {
		baseURL += ":554"
	} else {
		baseURL += fmt.Sprintf(":%v", cam.Port)
	}

	lowResURL, err := URLForCamera(cam.Model, baseURL, cam.LowResURLSuffix, cam.HighResURLSuffix, false)
	if err != nil {
		return nil, err
	}
	highResURL, err := URLForCamera(cam.Model, baseURL, cam.LowResURLSuffix, cam.HighResURLSuffix, true)
	if err != nil {
		return nil, err
	}

	highDumper := NewVideoDumpReader(ringBufferSizeBytes)

	// See discussion in videoDumpReader.go about the amount of storage that we need here
	// SYNC-MAX-TRAIN-RECORD-TIME
	lowDumper := NewVideoDumpReader(3 * 1024 * 1024)

	lowDecoder := NewVideoDecodeReader()
	high := NewStream(log, cam.Name, "high")
	low := NewStream(log, cam.Name, "low")

	return &Camera{
		Name:       cam.Name,
		Log:        log,
		LowStream:  low,
		HighStream: high,
		HighDumper: highDumper,
		LowDecoder: lowDecoder,
		LowDumper:  lowDumper,
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
	if err := c.LowStream.ConnectSinkAndRun(c.LowDumper); err != nil {
		return err
	}
	return nil
}

// Close the camera.
// If wg is not nil, then you must use it to signal when all of your resources are closed.
func (c *Camera) Close(wg *sync.WaitGroup) {
	if c.LowStream != nil {
		c.LowStream.Close(wg)
		c.LowStream = nil
	}
	if c.HighStream != nil {
		c.HighStream.Close(wg)
		c.HighStream = nil
	}
}

func (c *Camera) LatestImage(contentType string) []byte {
	img := c.LowDecoder.LastImage()
	if img == nil {
		return nil
	}
	buf, err := cimg.Compress(img, cimg.MakeCompressParams(cimg.Sampling(cimg.Sampling420), 85, cimg.Flags(0)))
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
func (c *Camera) GetStream(res defs.Resolution) *Stream {
	switch res {
	case defs.ResLD:
		return c.LowStream
	case defs.ResHD:
		return c.HighStream
	}
	return nil
}
