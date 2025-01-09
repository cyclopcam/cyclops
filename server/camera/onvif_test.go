package camera

import (
	"os"
	"testing"

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
	//d.Password = "y"

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

func testStream(t *testing.T) {
}

// example:
// CAMERA_HOST=192.168.10.10 CAMERA_USERNAME=admin CAMERA_PASSWORD=foo go test -v -run Onvif ./server/camera
func TestOnvif(t *testing.T) {
	d := loadTestCameraDetails(t)
	info, err := OnvifGetDeviceInfo(d.Host, d.Username, d.Password)
	require.NoError(t, err)
	t.Logf("Model: %v", info.Model)
	t.Logf("MainStreamURL: %v", info.MainStreamURL)
	t.Logf("SubStreamURL: %v", info.SubStreamURL)
	require.NotEmpty(t, info.Model)
	require.NotEqual(t, CameraBrandGenericONVIF, info.Model) // If you've got the camera in your possession, then add a new enum for it
	require.NotEmpty(t, info.MainStreamURL)
	require.NotEmpty(t, info.SubStreamURL)
}
