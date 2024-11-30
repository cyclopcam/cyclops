package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/akamensky/argparse"
	arcmodel "github.com/cyclopcam/cyclops/arc/server/model"
	"github.com/cyclopcam/cyclops/pkg/nn"
	"github.com/cyclopcam/cyclops/pkg/nnaccel"
	"github.com/cyclopcam/cyclops/pkg/nnload"
	"github.com/cyclopcam/logs"
	"github.com/cyclopcam/www"
)

const delayOnFailedModelRun = 15 * time.Minute

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	logger, err := logs.NewLog()
	check(err)

	parser := argparse.NewParser("bglabel", "Run NN model to label a high resolution video stream on an Arc server")
	serverUrl := parser.String("s", "server", &argparse.Options{Help: "Arc server URL (eg https://arc.cyclopcam.org)", Required: true})
	modelFilename := parser.String("m", "model", &argparse.Options{Help: "NN model file", Required: true})
	err = parser.Parse(os.Args)
	if err != nil {
		logger.Errorf(parser.Usage(err))
		os.Exit(1)
	}

	arcApiKey := os.Getenv("ARC_API_KEY")
	if arcApiKey == "" {
		logger.Errorf("Must set ARC_API_KEY environment variable")
		os.Exit(1)
	}

	if strings.HasSuffix(*serverUrl, "/") {
		*serverUrl = (*serverUrl)[:len(*serverUrl)-1]
	}

	// nil device = NCNN
	var device *nnaccel.Device

	model, err := nnload.LoadModel(logger, device, filepath.Dir(*modelFilename), filepath.Base(*modelFilename), 640, 480, nn.ThreadingModeParallel, nn.NewModelSetup())
	if err != nil {
		logger.Errorf("Failed to load NN model '%v': %v", *modelFilename, err)
		os.Exit(1)
	}

	bg := &BackgroundLabeler{
		serverUrl:   *serverUrl,
		apiKey:      arcApiKey,
		tempDir:     os.TempDir(),
		log:         logger,
		model:       model,
		modelConfig: *model.Config(),
		classes:     []string{"person", "bicycle", "car", "truck", "motorcycle", "cat", "dog", "horse", "bear"},
	}

	haveWork := true

	for {
		startFetch := time.Now()
		video, filename := bg.fetchNextUnlabeledVideo()
		if video == nil {
			if time.Now().Sub(startFetch) < 20*time.Second {
				bg.log.Infof("Long poll to fetch videos completed too fast. Sleeping for 30 seconds")
				bg.log.Infof("This is usually an indication that the server/network is down, or authentication is failing")
				time.Sleep(30 * time.Second)
			} else {
				if haveWork {
					bg.log.Infof("No more videos to label. Continuing to poll.")
				}
				haveWork = false
			}
			continue
		}
		haveWork = true
		labels, err := bg.runModelOnVideo(filename)
		if err != nil {
			bg.log.Criticalf("Failed to run model on video %v: %v. Sleeping for %v to avoid flooding logs.", video.ID, err, delayOnFailedModelRun)
			time.Sleep(delayOnFailedModelRun)
			continue
		}
		if err := bg.saveLabels(video.ID, labels); err != nil {
			bg.log.Criticalf("Failed to save labels of %v: %v.", video.ID, err)
		}
	}
}

type BackgroundLabeler struct {
	serverUrl   string
	apiKey      string
	tempDir     string
	log         logs.Log
	model       nn.ObjectDetector
	modelConfig nn.ModelConfig
	classes     []string
}

func (bg *BackgroundLabeler) videoFilename() string {
	return filepath.Join(bg.tempDir, "arc-bg-labeler-video.mp4")
}

func (bg *BackgroundLabeler) newRequest(method, url string, body io.Reader) *http.Request {
	req, err := http.NewRequest(method, bg.serverUrl+url, body)
	check(err)
	req.Header.Set("Authorization", "ApiKey "+bg.apiKey)
	return req
}

func (bg *BackgroundLabeler) fetchNextUnlabeledVideo() (*arcmodel.Video, string) {
	req := bg.newRequest("GET", "/api/videos/unlabeled", nil)
	videos := []arcmodel.Video{}
	err := www.FetchJSON(req, &videos)
	if err != nil {
		bg.log.Warnf("Failed to fetch unlabeled videos: %v", err)
		return nil, ""
	}
	if len(videos) == 0 {
		return nil, ""
	}
	video := &videos[0]
	bg.log.Infof("Fetching video %v", video.ID)
	filename := bg.videoFilename()
	if err := bg.fetchVideoFile(video.ID, filename); err != nil {
		bg.log.Warnf("Failed to fetch high res video %v: %v", video.ID, err)
		return nil, ""
	}
	return video, filename
}

func (bg *BackgroundLabeler) fetchVideoFile(vid int64, outputFilename string) error {
	req := bg.newRequest("GET", fmt.Sprintf("/api/video/%v/video/high", vid), nil)
	resp, err := www.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	f, err := os.Create(outputFilename)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

func (bg *BackgroundLabeler) saveLabels(vid int64, labels *nn.VideoLabels) error {
	jlabels, err := json.Marshal(labels)
	if err != nil {
		return err
	}
	req := bg.newRequest("POST", fmt.Sprintf("/api/video/%v/labels", vid), bytes.NewReader(jlabels))
	resp, err := www.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (bg *BackgroundLabeler) runModelOnVideo(filename string) (*nn.VideoLabels, error) {
	// Our minimum target size is 15 pixel on a low resolution video stream of 320 x 240.
	// Our target high resolution video height is 1000, so we have a ratio of 1000 / 240 = 4.16.
	// 15 * 4.16 = 62.5

	options := nn.InferenceOptions{
		MinSize:        62,
		MaxVideoHeight: 1000,
		Classes:        bg.classes,
		//StartFrame:     0,
		//EndFrame:       20,
		//StdOutProgress: true,
	}
	return nn.RunInferenceOnVideoFile(bg.model, filename, options)
}
