package scanner

import (
	"os"
	"testing"
	"time"

	"github.com/cyclopcam/cyclops/server/camera"
)

type testCameraDetails struct {
	Host    string
	UseHTTP bool
	UseRTSP bool
}

func loadTestCameraDetails() testCameraDetails {
	d := testCameraDetails{}
	d.Host = os.Getenv("CAMERA_HOST")
	d.UseHTTP = os.Getenv("CAMERA_USE_HTTP") == "1"
	d.UseRTSP = os.Getenv("CAMERA_USE_RTSP") == "1"
	// Hardcode for easy debug sessions
	//d.Host = "192.168.10.10"
	//d.UseHTTP = true
	return d
}

// Test only HTTP probing:
// CAMERA_HOST=192.168.10.10 CAMERA_USE_HTTP=1 go test -v -run TryConnect ./server/scanner
//
// Test only RTSP probing:
// CAMERA_HOST=192.168.10.10 CAMERA_USE_RTSP=1 go test -v -run TryConnect ./server/scanner
//
// Test HTTP + RTSP probing:
// CAMERA_HOST=192.168.10.10 CAMERA_USE_HTTP=1 CAMERA_USE_RTSP=1 go test -v -run TryConnect ./server/scanner
func TestTryConnect(t *testing.T) {
	d := loadTestCameraDetails()
	if d.Host == "" {
		t.Logf("CAMERA_HOST not set, skipping test")
		t.SkipNow()
	}
	if !d.UseHTTP && !d.UseRTSP {
		t.Logf("CAMERA_USE_HTTP and CAMERA_USE_RTSP not set, skipping test")
		t.SkipNow()
	}
	var methods ScanMethod
	if d.UseHTTP {
		methods |= ScanMethodHTTP
	}
	if d.UseRTSP {
		methods |= ScanMethodRTSP
	}

	t.Logf("Trying to contact camera at %v", d.Host)
	cam, err := TryToContactCamera(d.Host, 200*time.Millisecond, methods)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if cam == camera.CameraBrandUnknown {
		t.Errorf("Unknown camera model")
	} else {
		t.Logf("Camera type: %v", cam)
	}
}
