package test

import (
	"errors"
	"io"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bmharper/cimg/v2"
	"github.com/cyclopcam/cyclops/pkg/nn"
	"github.com/cyclopcam/cyclops/pkg/videox"
	"github.com/cyclopcam/cyclops/server/monitor"
	"github.com/cyclopcam/logs"
	"github.com/stretchr/testify/require"

	"github.com/fogleman/gg"
)

// If true, then render a video of our tracking results.
// This is very useful for quickly visualizing what the monitor/analyzer is doing.
// This makes the process MUCH slower, because we wait extra long for each NN frame.
const DumpTrackingVideo = false

type EventTrackingParams struct {
	ModelName  string  // eg "yolov8m"
	NNCoverage float64 // eg 75%, if we're able to run NN analysis on 75% of video frames (i.e. because we're resource constrained)
}

type Range struct {
	Min int
	Max int
}

type EventTrackingTestCase struct {
	VideoFilename string // eg "tracking/0001-LD.mp4"
	NumPeople     Range  // Expected number of people
	NumVehicles   Range  // Expected number of vehicles
}

type EventTrackingObject struct {
	ID            uint32
	Class         string
	NumDetections int
	Rect          nn.Rect // Most recent box
	PrevRect      nn.Rect // Previous box
}

// Go from "server/test" to ""
func FromTestPathToRepoRoot(testpath string) string {
	return filepath.Clean(filepath.Join("../..", testpath))
}

func drawTrackedObjects(d *gg.Context, objs []*EventTrackingObject) {
	for _, obj := range objs {
		text := "o"
		if obj.Class == "person" {
			d.SetRGB(1, 0, 0)
			text = "p"
		} else if obj.Class == "vehicle" {
			d.SetRGB(1, 1, 0)
			text = "v"
		} else {
			d.SetRGB(0, 0, 1)
		}
		d.DrawRectangle(float64(obj.Rect.X), float64(obj.Rect.Y), float64(obj.Rect.Width), float64(obj.Rect.Height))
		d.Stroke()
		if obj.PrevRect.Area() != 0 {
			c1 := obj.PrevRect.Center()
			c2 := obj.Rect.Center()
			d.DrawLine(float64(c1.X), float64(c1.Y), float64(c2.X), float64(c2.Y))
			d.Stroke()
		}
		d.DrawString(text, float64(obj.Rect.X+2), float64(obj.Rect.Y+8))
	}
}

func drawImageToVideo(t *testing.T, video *videox.VideoEncoder, d *gg.Context, pts time.Duration) {
	img, err := cimg.FromImage(d.Image(), true)
	require.NoError(t, err)
	img = img.ToRGB()
	err = video.WriteImage(pts, [][]uint8{img.Pixels}, []int{img.Stride})
	require.NoError(t, err)
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
		debugVideo, err = videox.NewVideoEncoder("h264", "mp4", "debug.mp4", decoder.Width(), decoder.Height(), videox.AVPixelFormatRGB24, videox.AVPixelFormatYUV420P, videox.VideoEncoderTypeImageFrames, 10)
		require.NoError(t, err)
		debugDraw = gg.NewContext(decoder.Width(), decoder.Height())
	}

	monitorOptions := monitor.DefaultMonitorOptions()
	monitorOptions.ModelName = params.ModelName
	monitorOptions.EnableFrameReader = false
	monitorOptions.ModelPaths = []string{FromTestPathToRepoRoot("models")}
	monitorOptions.MaxSingleThreadPerformance = true
	mon, err := monitor.NewMonitor(logger, monitorOptions)
	require.NoError(t, err)
	defer mon.Close()

	trackedObjects := []*EventTrackingObject{}

	// Ignore all concrete classes which map to "vehicle"
	ignoreClasses := map[string]bool{
		"car":        true,
		"motorcycle": true,
		"truck":      true,
		"bus":        true,
	}

	// Create a function that watches for incoming monitor messages.
	// These are the result of some post-processing that the monitor does on the raw NN object detection outputs.
	monChan := mon.AddWatcherAllCameras()
	nAnalysisReceived := atomic.Int32{}
	trackerExited := make(chan bool)

	// Wait until nFrames frames have been sent by the analyzer
	waitForNFramesAnalyzed := func(nFrames int) {
		for nAnalysisReceived.Load() < int32(nFrames) {
			time.Sleep(5 * time.Millisecond)
		}
	}

	go func() {
		for {
			msg := <-monChan
			if msg == nil {
				break
			}
			nAnalysisReceived.Add(1)
			for _, obj := range msg.Objects {
				if ignoreClasses[mon.AllClasses()[obj.Class]] {
					continue
				}
				if obj.Genuine == 0 {
					// We haven't seen enough frames of this object to confirm that it's a true positive
					continue
				}
				found := false
				for _, existing := range trackedObjects {
					if existing.ID == obj.ID {
						found = true
						existing.NumDetections++
						existing.PrevRect = existing.Rect
						existing.Rect = obj.LastFrame().Box
						break
					}
				}
				if !found {
					className := mon.AllClasses()[obj.Class]
					t.Logf("Found new object: %v at %v", className, obj.LastFrame().Box)
					trackedObjects = append(trackedObjects, &EventTrackingObject{
						ID:            obj.ID,
						NumDetections: 1,
						Class:         className,
						Rect:          obj.LastFrame().Box,
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
				// Wait for the NN thread to drain so that we can draw detections on top of this frame (for debug viewing)
				//t.Logf("wait")
				waitForNFramesAnalyzed(nFramesInjected)
				//t.Logf("done")

				img, err := frame.Image.ToCImageRGB().ToImage()
				require.NoError(t, err)
				debugDraw.DrawImage(img, 0, 0)
				drawTrackedObjects(debugDraw, trackedObjects)
				drawImageToVideo(t, debugVideo, debugDraw, decoder.FrameTimeToDuration(frame.PTS))
			}
		}
	}
	t.Logf("Injected %v frames. Waiting for NN to finish processing", nFramesInjected)
	waitForNFramesAnalyzed(nFramesInjected)

	// Signal our watcher function to exit
	t.Logf("Waiting for tracker to exit")
	monChan <- nil
	<-trackerExited

	nPerson := 0
	nVehicle := 0
	for _, obj := range trackedObjects {
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

	t.Logf("Found %v people, %v vehicles", nPerson, nVehicle)
	if nPerson < tcase.NumPeople.Min || nPerson > tcase.NumPeople.Max {
		t.Fatalf("Expected %v people, but found %v", tcase.NumPeople, nPerson)
	}
	if nVehicle < tcase.NumVehicles.Min || nVehicle > tcase.NumVehicles.Max {
		t.Fatalf("Expected %v vehicles, but found %v", tcase.NumVehicles, nVehicle)
	}
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
			NNCoverage: 0.6,
		},
	}
	cases := []*EventTrackingTestCase{
		{
			VideoFilename: "testdata/tracking/0001-LD.mp4",
			NumPeople:     Range{0, 0},
			NumVehicles:   Range{1, 1},
		},
		{
			VideoFilename: "testdata/tracking/0003-LD.mp4",
			NumPeople:     Range{1, 1},
			NumVehicles:   Range{1, 2}, // sometimes the trailer is detected as a 2nd vehicle. This is reasonable.
		},
		{
			VideoFilename: "testdata/tracking/0004-LD.mp4",
			NumPeople:     Range{1, 2}, // yolov8l finds both people. yolov8m only finds 1.
			NumVehicles:   Range{0, 0},
		},
		{
			VideoFilename: "testdata/tracking/0005-LD.mp4",
			NumPeople:     Range{1, 1},
			NumVehicles:   Range{0, 0},
		},
		{
			VideoFilename: "testdata/tracking/0007-LD.mp4",
			NumPeople:     Range{0, 0},
			NumVehicles:   Range{0, 0},
		},
		{
			VideoFilename: "testdata/tracking/0008-LD.mp4",
			NumPeople:     Range{0, 0},
			NumVehicles:   Range{0, 0},
		},
		{
			VideoFilename: "testdata/tracking/0009-LD.mp4",
			NumPeople:     Range{1, 1},
			NumVehicles:   Range{0, 0},
		},
	}
	// uncomment to test just the last case
	cases = cases[len(cases)-1:]
	for _, params := range paramPurmutations {
		for _, tcase := range cases {
			testEventTrackingCase(t, params, tcase)
		}
		//break
	}
}
