package camera

import (
	"github.com/bmharper/cimg/v2"
	"github.com/bmharper/cyclops/server/log"
	"github.com/bmharper/cyclops/server/videox"
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
	//highReader := &VideoDumpReader{
	//	Filename: name + ".ts",
	//}
	highReader := NewVideoDumpReader(32 * 1024 * 1024)
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
		c.LowRes = nil
	}
	if c.HighRes != nil {
		c.HighRes.Close()
		c.HighRes = nil
	}
}

func (c *Camera) LatestImage(contentType string) []byte {
	decoder := c.LowRes.Reader.(*VideoDecodeReader)
	img := decoder.LastImage()
	if img == nil {
		return nil
	}
	img2, err := cimg.FromImage(img, true)
	if err != nil {
		c.Log.Errorf("Failed to wrap decoded image into cimg: %v", err)
		return nil
	}
	buf, err := cimg.Compress(img2, cimg.MakeCompressParams(cimg.Sampling(cimg.Sampling420), 80, cimg.Flags(0)))
	if err != nil {
		c.Log.Errorf("Failed to compress image: %v", err)
		return nil
	}
	return buf
}

func (c *Camera) ExtractHighRes(method ExtractMethod) *videox.RawBuffer {
	dumper := c.HighRes.Reader.(*VideoDumpReader)
	return dumper.ExtractRawBuffer(method)
}
