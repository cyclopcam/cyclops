package test

import (
	"errors"
	"io"
	"testing"
	"time"

	"github.com/cyclopcam/cyclops/pkg/videox"
	"github.com/cyclopcam/cyclops/server/monitor"
	"github.com/cyclopcam/logs"
	"github.com/stretchr/testify/require"
)

type EventTrackingParams struct {
	ModelName  string  // eg "yolov8m"
	NNCoverage float64 // eg 75%, if we're able to run NN analysis on 75% of video frames (i.e. because we're resource constrained)
}

type EventTrackingTestCase struct {
	VideoFilename string // eg "tracking/0001-LD.mp4"
	NumPeople     int    // Expected number of people
	NumVehicles   int    // Expected number of vehicles
}

func testEventTrackingCase(t *testing.T, params *EventTrackingParams, tcase *EventTrackingTestCase) {
	decoder, err := videox.NewVideoFileDecoder2(tcase.VideoFilename)
	require.NoError(t, err)
	defer decoder.Close()

	logger := logs.NewTestingLog(t)

	monitor, err := monitor.NewMonitor(logger, params.ModelName, false)
	require.NoError(t, err)
	defer monitor.Close()

	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	monitor.InjectTestCamera() // cameraIndex = 0

	for i := 0; true; i++ {
		frame, err := decoder.NextFrame()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)
		//frame.Image.ToCImageRGB()
		monitor.InjectTestFrame(0, baseTime.Add(decoder.FrameTimeToDuration(frame.PTS)), frame.Image)
	}
}

// Test NN object detection, and our interpretation of what is a 'new' object,
// vs an existing object that has moved.
func TestEventTracking(t *testing.T) {
	paramPurmutations := []*EventTrackingParams{
		{
			ModelName:  "yolov8m",
			NNCoverage: 0.7,
		},
		{
			ModelName:  "yolov8m",
			NNCoverage: 0.5,
		},
		{
			ModelName:  "yolov8s",
			NNCoverage: 1,
		},
		{
			ModelName:  "yolov8s",
			NNCoverage: 0.5,
		},
	}
	cases := []*EventTrackingTestCase{
		{
			VideoFilename: "tracking/0001-LD.mp4",
			NumPeople:     1,
			NumVehicles:   0,
		},
	}
	for _, params := range paramPurmutations {
		for _, tcase := range cases {
			testEventTrackingCase(t, params, tcase)
		}
	}
}
