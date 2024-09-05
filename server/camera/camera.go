package camera

import (
	"fmt"
	"sync"
	"time"

	"github.com/bmharper/cimg/v2"
	"github.com/cyclopcam/cyclops/pkg/videox"
	"github.com/cyclopcam/cyclops/server/configdb"
	"github.com/cyclopcam/cyclops/server/defs"
	"github.com/cyclopcam/cyclops/server/videodb"
	"github.com/cyclopcam/logs"
)

// Camera represents a single physical camera, with two streams (high and low res)
type Camera struct {
	Log        logs.Log
	Config     configdb.Camera // Copy from the config database, from the moment when the camera was created. Can be out of date if camera config has been modified since.
	LowStream  *Stream
	HighStream *Stream
	HighDumper *VideoRingBuffer
	LowDecoder *VideoDecodeReader
	LowDumper  *VideoRingBuffer
	lowResURL  string
	highResURL string
}

func NewCamera(log logs.Log, cfg configdb.Camera, ringBufferSizeBytes int) (*Camera, error) {
	baseURL := "rtsp://" + cfg.Username + ":" + cfg.Password + "@" + cfg.Host
	if cfg.Port == 0 {
		baseURL += ":554"
	} else {
		baseURL += fmt.Sprintf(":%v", cfg.Port)
	}

	params, err := GetCameraModelParameters(cfg.Model, baseURL, cfg.LowResURLSuffix, cfg.HighResURLSuffix)
	if err != nil {
		return nil, err
	}

	highDumper := NewVideoRingBuffer(ringBufferSizeBytes)

	// See discussion in videoDumpReader.go about the amount of storage that we need here
	// SYNC-MAX-TRAIN-RECORD-TIME
	lowDumper := NewVideoRingBuffer(3 * 1024 * 1024)

	lowDecoder := NewVideoDecodeReader()
	high := NewStream(log, cfg.Name, "high", params.PacketsAreAnnexBEncoded)
	low := NewStream(log, cfg.Name, "low", params.PacketsAreAnnexBEncoded)

	return &Camera{
		Log:        log,
		Config:     cfg,
		LowStream:  low,
		HighStream: high,
		HighDumper: highDumper,
		LowDecoder: lowDecoder,
		LowDumper:  lowDumper,
		lowResURL:  params.LowResURL,
		highResURL: params.HighResURL,
	}, nil
}

func (c *Camera) ID() int64 {
	return c.Config.ID
}

func (c *Camera) Name() string {
	return c.Config.Name
}

func (c *Camera) LongLivedName() string {
	return c.Config.LongLivedName
}

// The name of the low res recording stream in the video archive
func (c *Camera) LowResRecordingStreamName() string {
	return c.RecordingStreamName(defs.ResLD)
}

// The name of the high res recording stream in the video archive
func (c *Camera) HighResRecordingStreamName() string {
	return c.RecordingStreamName(defs.ResHD)
}

// The name of high/low res recording stream in the video archive
func (c *Camera) RecordingStreamName(resolution defs.Resolution) string {
	return videodb.VideoStreamNameForCamera(c.Config.LongLivedName, resolution)
}

func (c *Camera) Start() error {
	if err := c.HighStream.Listen(c.highResURL); err != nil {
		return err
	}
	if err := c.LowStream.Listen(c.lowResURL); err != nil {
		return err
	}
	if err := c.HighStream.ConnectSinkAndRun("HD Ring", c.HighDumper); err != nil {
		return err
	}
	if err := c.LowStream.ConnectSinkAndRun("LD decode", c.LowDecoder); err != nil {
		return err
	}
	if err := c.LowStream.ConnectSinkAndRun("LD Ring", c.LowDumper); err != nil {
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

// Return the time of the most recently received packet from the camera.
// But NOTE! We return the oldest such packet of the two streams.
// In other words, we return min(LowStream.LastPacketReceivedAt(), HighStream.LastPacketReceivedAt()).
// This is to work around an issue where I have a camera who's HD stream
// frequently fails to send any packets, but the LD stream works fine.
// I'm not sure if this is a bug in gortsplib, but this seems like a robust
// strategy to have anyway. This bug only manifests during startup.
// Once you start receiving packets, it generally keeps going forever.
func (c *Camera) LastPacketAt() time.Time {
	t1 := c.LowStream.LastPacketReceivedAt().UnixMicro()
	t2 := c.HighStream.LastPacketReceivedAt().UnixMicro()
	t := min(t1, t2)
	return time.UnixMicro(t)
}

func (c *Camera) LatestImage(contentType string) []byte {
	img, _ := c.LowDecoder.LastImageCopy()
	if img == nil {
		return nil
	}
	// Yes, this is stupid going from YUV to RGB, to YUV, to JPEG.
	buf, err := cimg.Compress(img.ToCImageRGB(), cimg.MakeCompressParams(cimg.Sampling(cimg.Sampling420), 85, cimg.Flags(0)))
	if err != nil {
		c.Log.Errorf("Failed to compress image: %v", err)
		return nil
	}
	return buf
}

// Extract from <now - duration> until <now>.
// duration is a positive number.
func (c *Camera) ExtractHighRes(method ExtractMethod, duration time.Duration) (*videox.PacketBuffer, error) {
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
