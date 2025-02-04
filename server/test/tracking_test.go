package test

import (
	"errors"
	"fmt"
	"io"
	"os"
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
const DumpTrackingVideo = true

// If true, then render a still image with annotation of the first violating detection
const DumpFirstFalsePositive = true

// concrete classes which map to "vehicle"
var vehicleClasses = map[string]bool{
	"car":        true,
	"motorcycle": true,
	"truck":      true,
	"bus":        true,
}

type EventTrackingParams struct {
	ModelNameLQ string  // eg "yolov8m"
	ModelNameHQ string  // eg "yolov8l"
	NNCoverage  float64 // eg 75%, if we're able to run NN analysis on 75% of video frames (i.e. because we're resource constrained)
	NNThreads   int     // 0 = default
}

type Range struct {
	Min int
	Max int
}

type TestCaseResult struct {
	Expected                EventTrackingTestCase
	ActualPeople            int
	ActualPeopleUnconfirmed int
	ActualVehicles          int
}

func (t *TestCaseResult) IsPass() bool {
	return t.ActualPeople >= t.Expected.NumPeople.Min && t.ActualPeople <= t.Expected.NumPeople.Max &&
		t.ActualVehicles >= t.Expected.NumVehicles.Min && t.ActualVehicles <= t.Expected.NumVehicles.Max
}

type EventTrackingTestCase struct {
	VideoFilename string // eg "tracking/0001-LD.mp4"
	NumPeople     Range  // Expected number of people
	NumVehicles   Range  // Expected number of vehicles
	LowRes        bool   // True if this was intended to be used on a 320x256 NN model
}

type EventTrackingObject struct {
	ID            uint32
	Class         string
	NumDetections int
	Rect          nn.Rect // Most recent box
	PrevRect      nn.Rect // Previous box
	Genuine       int
}

// Go from "server/test" to ""
func FromTestPathToRepoRoot(testpath string) string {
	return filepath.Clean(filepath.Join("../..", testpath))
}

func drawTrackedObjects(d *gg.Context, objs []*EventTrackingObject) {
	for _, obj := range objs {
		text := "o"
		d.SetLineWidth(1)
		if obj.Class == "person" {
			d.SetRGB(1, 0, 0)
			text = "p"
		} else if obj.Class == "vehicle" {
			d.SetRGB(1, 1, 0)
			text = "v"
		} else {
			d.SetRGB(0, 0, 1)
		}
		if obj.Genuine != 0 {
			text = "G" + text
			d.SetLineWidth(2)
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

func testEventTrackingCase(t *testing.T, params *EventTrackingParams, tcase *EventTrackingTestCase, nnWidth, nnHeight int, needResult bool) TestCaseResult {
	decoder, err := videox.NewVideoFileDecoder(FromTestPathToRepoRoot(tcase.VideoFilename))
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
	monitorOptions.DebugTracking = true
	monitorOptions.ModelsDir = FromTestPathToRepoRoot("models")
	monitorOptions.ModelWidth = nnWidth
	monitorOptions.ModelHeight = nnHeight
	if params.NNThreads != 0 {
		monitorOptions.NNThreads = params.NNThreads
	}
	if DumpTrackingVideo || DumpFirstFalsePositive {
		// In order to produce our videos/image dumps, we send one frame, then wait for it to be processed.
		// This gets extremely complicated with batch sizes greater than 1, especially when considering
		// the validation network running. So that's why we reduce batch size to 1 when producing these dumps.
		monitorOptions.ForceBatchSizeOne = true
	}
	// hmmmmm
	monitorOptions.ForceBatchSizeOne = true

	mon, err := monitor.NewMonitor(logger, monitorOptions)
	require.NoError(t, err)
	defer mon.Close()

	trackedObjects := []*EventTrackingObject{}

	countTrackedObjects := func() (person, vehicle, personUnconfirmed int) {
		for _, obj := range trackedObjects {
			if obj.Class == "person" {
				personUnconfirmed++
			}
			if obj.Genuine == 0 {
				continue
			}
			if obj.Class == "person" {
				person++
			} else if obj.Class == "vehicle" || vehicleClasses[obj.Class] {
				vehicle++
			}
		}
		return
	}

	// Create a function that watches for incoming monitor messages.
	// These are the result of some post-processing that the monitor does on the raw NN object detection outputs.
	monChan := mon.AddWatcherAllCameras()
	nAnalysisReceived := atomic.Int32{}
	trackerExited := make(chan bool)

	// Wait for the monitor to be done processing whatever frames we've given it
	waitForMonitorToProcessFrames := func() {
		lastActivity := time.Now()
		for true {
			if mon.NNQueueLength() != 0 || mon.NumNNThreadsActive() != 0 {
				lastActivity = time.Now()
			}
			if time.Since(lastActivity) > 5*time.Millisecond {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
	}

	go func() {
		for {
			msg := <-monChan
			if msg == nil {
				break
			}
			//t.Logf("monChan incoming")
			nAnalysisReceived.Add(1)
			for _, obj := range msg.Objects {
				className := mon.AllClasses()[obj.Class]
				//if vehicleClasses[className] {
				//	continue
				//}
				found := false
				confidence := obj.LastFrame().Confidence
				for _, existing := range trackedObjects {
					if existing.ID == obj.ID {
						found = true
						existing.NumDetections++
						existing.PrevRect = existing.Rect
						existing.Rect = obj.LastFrame().Box
						existing.Genuine = obj.Genuine
						//t.Logf("Existing: %v (%.2f) at %v (genuine %v)", className, confidence, obj.LastFrame().Box, obj.Genuine)
						break
					}
				}
				if !found {
					t.Logf("New object: %v (%.2f) at %v (genuine %v)", className, confidence, obj.LastFrame().Box, obj.Genuine)
					trackedObjects = append(trackedObjects, &EventTrackingObject{
						ID:            obj.ID,
						NumDetections: 1,
						Class:         className,
						Rect:          obj.LastFrame().Box,
						Genuine:       obj.Genuine,
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
	imgID := int64(0)
	var lastFrame *videox.Frame

	for i := 0; true; i++ {
		frame, err := decoder.NextFrame()
		if errors.Is(err, io.EOF) {
			break
		}
		lastFrame = frame
		require.NoError(t, err)
		imgID++
		inclusionProbability += params.NNCoverage
		if inclusionProbability >= 1 {
			inclusionProbability -= 1
			mon.InjectTestFrame(0, imgID, baseTime.Add(decoder.FrameTimeToDuration(frame.PTS)), frame.Image)
			nFramesInjected++

			if DumpTrackingVideo {
				// Wait for the NN thread to drain so that we can draw detections on top of this frame (for debug viewing)
				//t.Logf("wait")
				waitForMonitorToProcessFrames()
				//t.Logf("done")

				img, err := frame.Image.ToCImageRGB().ToImage()
				require.NoError(t, err)
				debugDraw.DrawImage(img, 0, 0)
				drawTrackedObjects(debugDraw, trackedObjects)
				drawImageToVideo(t, debugVideo, debugDraw, decoder.FrameTimeToDuration(frame.PTS))
			}
			if DumpFirstFalsePositive && !haveDumpedFalsePositive {
				waitForMonitorToProcessFrames()
				nPerson, nVehicle, _ := countTrackedObjects()
				dump := nPerson > 0 && tcase.NumPeople.Max == 0 ||
					nVehicle > 0 && tcase.NumVehicles.Max == 0
				if dump {
					img, err := frame.Image.ToCImageRGB().ToImage()
					require.NoError(t, err)
					debugDraw.DrawImage(img, 0, 0)
					drawTrackedObjects(debugDraw, trackedObjects)
					im, err := cimg.FromImage(debugDraw.Image(), true)
					require.NoError(t, err)
					frame.Image.ToCImageRGB().WriteJPEG("false-positive-first-raw.jpg", cimg.MakeCompressParams(cimg.Sampling444, 99, 0), 0644)
					im.WriteJPEG("false-positive-first-box.jpg", cimg.MakeCompressParams(cimg.Sampling444, 95, 0), 0644)
				}
			}
		}
	}
	// Inject fake frames to ensure that the NN batches are flushed and the system actually analyzes all of the frames
	nFake := 7
	t.Logf("Injected %v real frames. Injecting %v fake frames to ensure batches are flushed", nFramesInjected, nFake)
	for i := 0; i < nFake; i++ {
		imgID++
		mon.InjectTestFrame(0, imgID, baseTime.Add(decoder.FrameTimeToDuration(lastFrame.PTS)), lastFrame.Image)
	}

	t.Logf("Injected %v frames. Waiting for NN to finish processing", nFramesInjected)
	waitForMonitorToProcessFrames()

	// Signal our watcher function to exit
	t.Logf("Waiting for tracker to exit")
	monChan <- nil
	<-trackerExited

	nPerson, nVehicle, nPersonUnconfirmed := countTrackedObjects()

	if DumpTrackingVideo {
		debugVideo.WriteTrailer()
		debugVideo.Close()
	}

	t.Logf("Found %v people, %v vehicles (%v unconfirmed people)", nPerson, nVehicle, nPersonUnconfirmed)
	if nPerson < tcase.NumPeople.Min || nPerson > tcase.NumPeople.Max {
		t.Logf("Expected %v people, but found %v", tcase.NumPeople, nPerson)
		if !needResult {
			t.Fail()
		}
	}
	if nVehicle < tcase.NumVehicles.Min || nVehicle > tcase.NumVehicles.Max {
		t.Logf("Expected %v vehicles, but found %v", tcase.NumVehicles, nVehicle)
		if !needResult {
			t.Fail()
		}
	}
	return TestCaseResult{
		Expected:                *tcase,
		ActualPeople:            nPerson,
		ActualPeopleUnconfirmed: nPersonUnconfirmed,
		ActualVehicles:          nVehicle,
	}
}

// Test NN object detection, and our interpretation of what is a 'new' object,
// vs an existing object that has moved.
func TestEventTracking(t *testing.T) {
	nnload.LoadAccelerators(logs.NewTestingLog(t), true)

	// If true, then do not fail the test on the first failure, but write our results out to
	// a CSV file. Someday I hope to have zero failures, but right now that's not happening.
	// With a correctly tuned model we should get there.
	writeToResultFile := true

	defaultNNWidth := 0
	defaultNNHeight := 0
	if nnload.HaveAccelerator() {
		// hailo
		defaultNNWidth = 640
		defaultNNHeight = 640
	} else {
		// ncnn
		// If we omit any explicit config, we'll get 320x256
		defaultNNWidth = 640
		defaultNNHeight = 480
	}

	paramPurmutations := []EventTrackingParams{
		{
			ModelNameLQ: "yolov8m",
			ModelNameHQ: "yolov8l",
			NNCoverage:  1,
			//NNWidth:     defaultNNWidth,
			//NNHeight:    defaultNNHeight,
			NNThreads: 1, // Setting this to 1 can aid debugging
		},
	}
	cases := []*EventTrackingTestCase{
		{
			VideoFilename: "testdata/tracking/0001-LD.mp4",
			NumPeople:     Range{0, 0},
			NumVehicles:   Range{1, 1},
			LowRes:        true,
		},
		{
			VideoFilename: "testdata/tracking/0003-LD.mp4",
			NumPeople:     Range{0, 1}, // The guy on the back of the trailer is not detected by the 640x480 NCNN model.
			NumVehicles:   Range{1, 2}, // sometimes the trailer is detected as a 2nd vehicle. This is reasonable.
			LowRes:        true,
		},
		{
			VideoFilename: "testdata/tracking/0004-LD.mp4",
			NumPeople:     Range{1, 2}, // yolov8l finds both people. yolov8m only finds 1.
			NumVehicles:   Range{0, 0},
			LowRes:        true, // Fails to find people on hailo8L yolov8m 640x640
		},
		{
			VideoFilename: "testdata/tracking/0005-LD.mp4",
			NumPeople:     Range{1, 1},
			NumVehicles:   Range{0, 0},
			LowRes:        true,
		},
		{
			VideoFilename: "testdata/tracking/0007-LD.mp4",
			NumPeople:     Range{0, 0},
			NumVehicles:   Range{0, 0},
			LowRes:        true,
		},
		{
			VideoFilename: "testdata/tracking/0008-LD.mp4",
			NumPeople:     Range{0, 0},
			NumVehicles:   Range{0, 0},
			LowRes:        true,
		},
		{
			VideoFilename: "testdata/tracking/0009-LD.mp4",
			NumPeople:     Range{1, 1},
			NumVehicles:   Range{0, 0},
			LowRes:        true,
		},
		{
			// This generated a false positive of a "person" on the hailo8L yolov8m, but it was
			// actually a cat at night. The NCNN model doesn't have this problem.
			VideoFilename: "testdata/tracking/0010-LD.mp4",
			NumPeople:     Range{0, 0},
			NumVehicles:   Range{0, 0},
			LowRes:        true,
		},
		{
			// This generated a false positive of a "person" on the hailo8L yolov8m, but it was
			// just leaves blowing in front of the camera. The NCNN model doesn't have this problem.
			// Grrrrr.... NCNN yolov8l at 640x480 still has the false positive here.
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
		{
			// This generated a false positive of a vehicle (just a water tank) on the hailo8L yolov8m
			VideoFilename: "testdata/tracking/0013-LD.mp4",
			NumPeople:     Range{0, 0},
			NumVehicles:   Range{0, 0},
		},
		{
			// This generated a false positive of a person (actually a cat at night) on the hailo8L yolov8m
			VideoFilename: "testdata/tracking/0014-LD.mp4",
			NumPeople:     Range{0, 0},
			NumVehicles:   Range{0, 0},
		},
		{
			// This generated a false positive of a person (actually a dog at twilight) on the hailo8L yolov8m ANB yolov8l
			VideoFilename: "testdata/tracking/0015-LD.mp4",
			NumPeople:     Range{0, 0},
			NumVehicles:   Range{0, 0},
		},
		{
			// Spider!
			VideoFilename: "testdata/tracking/0016-LD.mp4",
			NumPeople:     Range{0, 0},
			NumVehicles:   Range{0, 0},
		},
		{
			// Same spider
			VideoFilename: "testdata/tracking/0017-LD.mp4",
			NumPeople:     Range{0, 0},
			NumVehicles:   Range{0, 0},
		},
		{
			// Cat (not person)
			VideoFilename: "testdata/tracking/0018-LD.mp4",
			NumPeople:     Range{0, 0},
			NumVehicles:   Range{0, 0},
		},
		{
			// Cat (not person)
			VideoFilename: "testdata/tracking/0019-LD.mp4",
			NumPeople:     Range{0, 0},
			NumVehicles:   Range{0, 0},
		},
		{
			// Plant (not person)
			VideoFilename: "testdata/tracking/0020-LD.mp4",
			NumPeople:     Range{0, 0},
			NumVehicles:   Range{0, 0},
		},
	}

	// The following chunk is useful for isolating a single test case
	//cases = gen.Filter(cases, func(tcase *EventTrackingTestCase) bool {
	//	// regex to extract the number 20 out of "testdata/tracking/0020-LD.mp4"
	//	nr := regexp.MustCompile(`\d+`)
	//	m := nr.FindString(tcase.VideoFilename)
	//	n, _ := strconv.Atoi(m)
	//	return n >= 19 && n <= 20
	//})

	var resultFile *os.File
	if writeToResultFile {
		var err error
		resultFile, err = os.Create("tracking-results.csv")
		require.NoError(t, err)
		defer resultFile.Close()
		_, err = fmt.Fprintf(resultFile, "Video,Pass,Min People,Max People,Actual People,Actual People Unconfirmed,Min Vehicles,Max Vehicles,Actual Vehicles,Has False Positives,Has False Negatives,Weak False Positives\n")
		require.NoError(t, err)
	}
	writeResult := func(r TestCaseResult) {
		e := r.Expected
		hasFalsePositives := r.ActualPeople > r.Expected.NumPeople.Max || r.ActualVehicles > r.Expected.NumVehicles.Max
		hasFalseNegatives := r.ActualPeople < r.Expected.NumPeople.Min || r.ActualVehicles < r.Expected.NumVehicles.Min
		weakFalsePositives := max(0, r.ActualPeopleUnconfirmed-r.Expected.NumPeople.Max)
		pass := !hasFalsePositives && !hasFalseNegatives
		fmt.Fprintf(resultFile,
			"%v,%v,%v,%v,%v,%v,%v,%v,%v,%v,%v,%v\n",
			e.VideoFilename, pass, e.NumPeople.Max, e.NumPeople.Max, r.ActualPeople, r.ActualPeopleUnconfirmed, e.NumVehicles.Min, e.NumVehicles.Max, r.ActualVehicles, hasFalsePositives, hasFalseNegatives, weakFalsePositives)
	}

	numPass := 0
	numFail := 0

	for iparams, params := range paramPurmutations {
		t.Logf("Testing parameter permutation %v/%v (%v, %v, %v)", iparams, len(paramPurmutations), params.ModelNameLQ, params.ModelNameHQ, params.NNCoverage)
		for _, tcase := range cases {
			if tcase.LowRes && nnload.HaveAccelerator() {
				// For Hailo we only publish 640x640
				t.Logf("Testing case %v (skipping because footage is low res, and NN is high res)", tcase.VideoFilename)
				continue
			}
			nnWidth := defaultNNWidth
			nnHeight := defaultNNHeight
			if tcase.LowRes {
				nnWidth = 320
				nnHeight = 256
			}
			t.Logf("Testing case %v", tcase.VideoFilename)
			result := testEventTrackingCase(t, &params, tcase, nnWidth, nnHeight, writeToResultFile)
			if writeToResultFile {
				writeResult(result)
			}
			if result.IsPass() {
				numPass++
			} else {
				numFail++
			}
		}
	}

	t.Logf("Passed %v/%v", numPass, numPass+numFail)
	if numFail != 0 {
		t.Fail()
	}
}
