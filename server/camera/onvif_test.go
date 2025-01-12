package camera

import (
	"os"
	"testing"
	"time"

	"github.com/cyclopcam/logs"
	"github.com/stretchr/testify/require"
)

type testCameraDetails struct {
	Host     string
	Username string
	Password string
}

func loadTestCameraDetails(t *testing.T) testCameraDetails {
	d := testCameraDetails{}
	d.Host = os.Getenv("CAMERA_HOST")
	d.Username = os.Getenv("CAMERA_USERNAME")
	d.Password = os.Getenv("CAMERA_PASSWORD")
	// Hardcode for easy debug sessions
	//d.Host = "192.168.10.10"
	//d.Username = "x"
	//d.Password = "x"

	if d.Host == "" {
		t.Logf("CAMERA_HOST not set, skipping test")
		t.SkipNow()
	}
	if d.Username == "" {
		t.Logf("CAMERA_USERNAME not set, skipping test")
		t.SkipNow()
	}
	if d.Password == "" {
		t.Logf("CAMERA_PASSWORD not set, skipping test")
		t.SkipNow()
	}

	return d
}

func testStream(t *testing.T, streamUrl string, rtspInfo *CameraRTSPInfo) {
	t.Logf("Testing stream: %v", streamUrl)
	logger := logs.NewTestingLog(t)
	s := NewStream(logger, "test", "testStream", rtspInfo.PacketsAreAnnexBEncoded)
	require.NoError(t, s.Listen(streamUrl))
	decoder := NewVideoDecodeReader()
	require.NoError(t, s.ConnectSinkAndRun("decoder", decoder))
	start := time.Now()
	for {
		if time.Now().Sub(start) > 5*time.Second {
			t.Errorf("Failed to get video frames")
			break
		}
		time.Sleep(500 * time.Millisecond)
		img, id := decoder.LastImageCopy()
		if img != nil {
			t.Logf("Got frame %v (%v x %v)", id, img.Width, img.Height)
			break
		}
	}
	s.Close(nil)
}

// This test does the following:
// 1. Uses ONVIF to get information about the camera
// 2. Connects to the low and main streams, and makes sure we can decode at least 1 frame
// Example:
// CAMERA_HOST=192.168.10.10 CAMERA_USERNAME=admin CAMERA_PASSWORD=foo go test -v -run Onvif ./server/camera
func TestOnvif(t *testing.T) {
	d := loadTestCameraDetails(t)
	info, err := OnvifGetDeviceInfo(d.Host, d.Username, d.Password)
	require.NoError(t, err)
	t.Logf("Camera: %v (%v, %v, %v)", info.Brand, info.Model, info.Firmware, info.Serial)
	t.Logf("MainStreamURL: %v", info.MainStreamURL)
	t.Logf("SubStreamURL: %v", info.SubStreamURL)
	require.NotEmpty(t, info.Brand)
	require.NotEqual(t, CameraBrandGenericONVIF, info.Brand) // If you've got the camera in your possession, then add a new enum for it
	require.NotEmpty(t, info.MainStreamURL)
	require.NotEmpty(t, info.SubStreamURL)
	rtspInfo, err := GetCameraRTSP(info.Brand, d.Host, d.Username, d.Password, 0, info.SubStreamURL, info.MainStreamURL)
	require.NoError(t, err)
	testStream(t, rtspInfo.LowResURL, rtspInfo)
	if t.Failed() {
		return
	}
	testStream(t, rtspInfo.HighResURL, rtspInfo)
}
