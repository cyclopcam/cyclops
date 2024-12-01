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
	"github.com/cyclopcam/cyclops/pkg/nnload"
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

// If true, then render a still image with annotation of the first violating detection
const DumpFirstFalsePositive = true

type EventTrackingParams struct {
	ModelNameLQ string  // eg "yolov8m"
	ModelNameHQ string  // eg "yolov8l"
	NNCoverage  float64 // eg 75%, if we're able to run NN analysis on 75% of video frames (i.e. because we're resource constrained)
	NNWidth     int     // eg 320, 640
	NNHeight    int     // eg 256, 480
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
	decoder, err := videox.NewVideoFileDecoder(absPath)
	require.NoError(t, err)
	defer decoder.Close()

	logger := logs.NewTestingLog(t)

	t.Logf("Video is %v x %v", decoder.Width(), decoder.Height())
	var debugVideo *videox.VideoEncoder
	var debugDraw *gg.Context
	if DumpTrackingVideo {
		debugVideo, err = videox.NewVideoEncoder("h264", "mp4", "debug.mp4", decoder.Width(), decoder.Height(), videox.AVPixelFormatRGB24, videox.AVPixelFormatYUV420P, videox.VideoEncoderTypeImageFrames, 10)
		require.NoError(t, err)
	}
	if DumpTrackingVideo || DumpFirstFalsePositive {
		debugDraw = gg.NewContext(decoder.Width(), decoder.Height())
	}

	monitorOptions := monitor.DefaultMonitorOptions()
	monitorOptions.ModelNameLQ = params.ModelNameLQ
	monitorOptions.ModelNameHQ = params.ModelNameHQ
	monitorOptions.EnableFrameReader = false
	monitorOptions.EnableDualModel = true
	monitorOptions.ModelsDir = FromTestPathToRepoRoot("models")
	if params.NNWidth != 0 {
		monitorOptions.ModelWidth = params.NNWidth
		monitorOptions.ModelHeight = params.NNHeight
	}

	// MaxSingleThreadPerformance hurts performance during regular testing, when DumpTrackingVideo = false
	//monitorOptions.MaxSingleThreadPerformance = true

	mon, err := monitor.NewMonitor(logger, monitorOptions)
	require.NoError(t, err)
	defer mon.Close()

	trackedObjects := []*EventTrackingObject{}

	countTrackedObjects := func() (person, vehicle int) {
		nPerson := 0
		nVehicle := 0
		for _, obj := range trackedObjects {
			if obj.Class == "person" {
				nPerson++
			} else if obj.Class == "vehicle" {
				nVehicle++
			}
		}
		return nPerson, nVehicle
	}

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
				confidence := obj.LastFrame().Confidence
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
					t.Logf("Found new object: %v (%.2f) at %v", className, confidence, obj.LastFrame().Box)
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

	haveDumpedFalsePositive := false

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
			if DumpFirstFalsePositive && !haveDumpedFalsePositive {
				waitForNFramesAnalyzed(nFramesInjected)
				nPerson, nVehicle := countTrackedObjects()
				dump := nPerson > 0 && tcase.NumPeople.Max == 0 ||
					nVehicle > 0 && tcase.NumVehicles.Max == 0
				if dump {
					img, err := frame.Image.ToCImageRGB().ToImage()
					require.NoError(t, err)
					debugDraw.DrawImage(img, 0, 0)
					drawTrackedObjects(debugDraw, trackedObjects)
					im, err := cimg.FromImage(debugDraw.Image(), true)
					require.NoError(t, err)
					im.WriteJPEG("first_false_positive.jpg", cimg.MakeCompressParams(cimg.Sampling444, 95, 0), 0644)
				}
			}
		}
	}
	t.Logf("Injected %v frames. Waiting for NN to finish processing", nFramesInjected)
	waitForNFramesAnalyzed(nFramesInjected)

	// Signal our watcher function to exit
	t.Logf("Waiting for tracker to exit")
	monChan <- nil
	<-trackerExited

	nPerson, nVehicle := countTrackedObjects()

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
			ModelNameLQ: "yolov8m",
			ModelNameHQ: "yolov8l",
			NNCoverage:  1,
			//NNWidth:    320,
			//NNHeight:   256,
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
		{
			// This generated a false positive of a "person" on the hailo8L yolov8m, but it was
			// actually a cat at night. The NCNN model doesn't have this problem.
			VideoFilename: "testdata/tracking/0010-LD.mp4",
			NumPeople:     Range{0, 0},
			NumVehicles:   Range{0, 0},
		},
		{
			// This generated a false positive of a "person" on the hailo8L yolov8m, but it was
			// just leaves blowing in front of the camera. The NCNN model doesn't have this problem.
			VideoFilename: "testdata/tracking/0011-LD.mp4",
			NumPeople:     Range{0, 0},
			NumVehicles:   Range{0, 0},
		},
		{
			// Same as above case - leaves masquerading as people. I added this as a second validation
			// case for the above test case (11).
			// HOWEVER!!!
			// This test also fails on various other NN architectures.
			// Models that pass/fail this test:
			// hailo yolov8m 640x640   fail
			// ncnn  yolov8s 320x256   pass
			// ncnn  yolov8m 320x256   fail
			// ncnn  yolov8m 640x480   fail
			// ncnn  yolov8l 640x480   pass
			VideoFilename: "testdata/tracking/0012-LD.mp4",
			NumPeople:     Range{0, 0},
			NumVehicles:   Range{0, 0},
		},
	}
	// uncomment the following line, to test just the last case
	onlyTestLastCase := true // DO NOT COMMIT

	if onlyTestLastCase {
		cases = cases[len(cases)-1:]
		t.Logf("WARNING! Only testing the LAST case") // just in case you forget and commit "onlyTestLastCase := true"
	}

	nnload.LoadAccelerators(logs.NewTestingLog(t), true)

	for iparams, params := range paramPurmutations {
		t.Logf("Testing parameter permutation %v/%v (%v, %v, %v)", iparams, len(paramPurmutations), params.ModelNameLQ, params.ModelNameHQ, params.NNCoverage)
		for _, tcase := range cases {
			t.Logf("Testing case %v", tcase.VideoFilename)
			testEventTrackingCase(t, params, tcase)
		}
		//break
	}
}
