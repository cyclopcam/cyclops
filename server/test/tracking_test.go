package test

import (
	"errors"
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/cyclopcam/cyclops/pkg/videox"
	"github.com/cyclopcam/cyclops/server/monitor"
	"github.com/cyclopcam/logs"
	"github.com/stretchr/testify/require"

	"github.com/fogleman/gg"
)

// If true, then render a video of our tracking results.
// This is very useful for quickly visualizing what the monitor/analyzer is doing.
const DumpTrackingVideo = true

type EventTrackingParams struct {
	ModelName  string  // eg "yolov8m"
	NNCoverage float64 // eg 75%, if we're able to run NN analysis on 75% of video frames (i.e. because we're resource constrained)
}

type EventTrackingTestCase struct {
	VideoFilename string // eg "tracking/0001-LD.mp4"
	NumPeople     int    // Expected number of people
	NumVehicles   int    // Expected number of vehicles
}

type EventTrackingObject struct {
	ID            uint32
	Class         string
	NumDetections int
}

// Go from "server/test" to ""
func FromTestPathToRepoRoot(testpath string) string {
	return filepath.Clean(filepath.Join("../..", testpath))
}

func testEventTrackingCase(t *testing.T, params *EventTrackingParams, tcase *EventTrackingTestCase) {
	absPath := FromTestPathToRepoRoot(tcase.VideoFilename)
	decoder, err := videox.NewVideoFileDecoder2(absPath)
	require.NoError(t, err)
	defer decoder.Close()

	logger := logs.NewTestingLog(t)

	t.Logf("Video is %v x %v", decoder.Width(), decoder.Height())
	var debugVideo *videox.VideoEncoder
	var debugDraw *gg.Context
	if DumpTrackingVideo {
		debugVideo, err = videox.NewVideoEncoder("mp4", "debug.mp4", decoder.Width(), decoder.Height())
		require.NoError(t, err)
		debugDraw = gg.NewContext(decoder.Width(), decoder.Height())
	}

	monitorOptions := monitor.DefaultMonitorOptions()
	monitorOptions.ModelName = params.ModelName
	monitorOptions.EnableFrameReader = false
	monitorOptions.ModelPaths = []string{FromTestPathToRepoRoot("models")}
	mon, err := monitor.NewMonitor(logger, monitorOptions)
	require.NoError(t, err)
	defer mon.Close()

	objects := []*EventTrackingObject{}

	// Create a function that watches for incoming monitor messages.
	// These are the result of some post-processing that the monitor does on the raw NN object detection outputs.
	monChan := mon.AddWatcherAllCameras()
	trackerExited := make(chan bool)
	go func() {
		for {
			msg := <-monChan
			if msg == nil {
				break
			}
			for _, obj := range msg.Objects {
				found := false
				for _, existing := range objects {
					if existing.ID == obj.ID {
						found = true
						existing.NumDetections++
						break
					}
				}
				if !found {
					className := mon.AllClasses()[obj.Class]
					t.Logf("Found new object: %v at %v", className, obj.Box)
					objects = append(objects, &EventTrackingObject{
						ID:            obj.ID,
						NumDetections: 1,
						Class:         mon.AllClasses()[obj.Class],
					})
				}
			}
		}
		close(trackerExited)
	}()

	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	mon.InjectTestCamera() // cameraIndex = 0

	// Probability of including frame
	inclusionProbability := 0.0
	nFramesInjected := 0

	for i := 0; true; i++ {
		frame, err := decoder.NextFrame()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)
		inclusionProbability += params.NNCoverage
		if inclusionProbability >= 1 {
			inclusionProbability -= 1
			mon.InjectTestFrame(0, baseTime.Add(decoder.FrameTimeToDuration(frame.PTS)), frame.Image)
			nFramesInjected++
			if DumpTrackingVideo {
				img, err := frame.Image.ToCImageRGB().ToImage()
				require.NoError(t, err)
				debugDraw.DrawImage(img, 0, 0)
			}
		}
	}
	t.Logf("Injected %v frames. Waiting for NN to finish processing", nFramesInjected)
	for mon.NNThreadQueueLength() != 0 {
		time.Sleep(10 * time.Millisecond)
	}

	// Signal our watcher function to exit
	t.Logf("Waiting for tracker to exit")
	monChan <- nil
	<-trackerExited

	nPerson := 0
	nVehicle := 0
	for _, obj := range objects {
		if obj.Class == "person" {
			nPerson++
		} else if obj.Class == "vehicle" {
			nVehicle++
		}
	}

	if DumpTrackingVideo {
		debugVideo.WriteTrailer()
		debugVideo.Close()
	}

	require.Equal(t, tcase.NumPeople, nPerson, "people")
	require.Equal(t, tcase.NumVehicles, nVehicle, "vehicles")
}

// Test NN object detection, and our interpretation of what is a 'new' object,
// vs an existing object that has moved.
func TestEventTracking(t *testing.T) {
	paramPurmutations := []*EventTrackingParams{
		{
			ModelName:  "yolov8m",
			NNCoverage: 1,
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
			VideoFilename: "testdata/tracking/0001-LD.mp4",
			NumPeople:     1,
			NumVehicles:   0,
		},
	}
	for _, params := range paramPurmutations {
		for _, tcase := range cases {
			testEventTrackingCase(t, params, tcase)
		}
		break
	}
}
