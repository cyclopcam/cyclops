package nnload

// Package nnload wraps up our 'nn' interface layer, and has concrete references to our
// neural network implementation (eg ncnn), so that you can just call one function to
// load a model, and not need to know about the implementation details.
//
// This is also the place where we detect the presence of an NN accelerator (eg Hailo),
// and then use that if it is available.

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/cyclopcam/cyclops/pkg/ncnn"
	"github.com/cyclopcam/cyclops/pkg/nn"
	"github.com/cyclopcam/cyclops/pkg/nnaccel"
	"github.com/cyclopcam/logs"
)

// If not nil, then we have successfully loaded the Hailo AI accelerator module
var hailoAccel *nnaccel.Accelerator
var isLoaded bool

// Return true if we are using a hardware NN accelerator
func HaveAccelerator() bool {
	return HaveHailo()
}

// Return true if we have a Hailo accelerator
func HaveHailo() bool {
	return hailoAccel != nil
}

// Return the NN accelerator that we choose to use (or nil if we must use NCNN)
func Accelerator() *nnaccel.Accelerator {
	// If we supported more accelerators, then they'd go here
	return hailoAccel
}

func downloadFile(srcUrl, targetFile string) error {
	tempFile := targetFile + ".tmp"
	if err := os.MkdirAll(filepath.Dir(targetFile), 0755); err != nil {
		return err
	}
	resp, err := http.DefaultClient.Get(srcUrl)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP error %v", resp.Status)
	}
	file, err := os.Create(tempFile)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return err
	}
	file.Close()
	return os.Rename(tempFile, targetFile)
}

func ModelFiles(device *nnaccel.Device, modelName string) (subdir string, ext []string) {
	if device != nil {
		// subdir is eg "hailo/8L", for the "8L" accelerator.
		subdir, ext := device.ModelFiles()
		return "coco/" + subdir, ext
	} else {
		return "coco/ncnn", []string{".param", ".bin"}
	}
}

func ModelStub(modelName string, width, height int) string {
	// eg "yolov8m_320_256"
	return fmt.Sprintf("%v_%v_%v", modelName, width, height)
}

// If the model files are not yet downloaded, then download them now.
// Returns immediately if the files are already downloaded.
func DownloadModel(logs logs.Log, device *nnaccel.Device, modelDir, modelName string, width, height int) error {
	baseUrl := "https://models.cyclopcam.org"
	subdir, ext := ModelFiles(device, modelName)
	extensions := append([]string{".json"}, ext...)
	modelStub := ModelStub(modelName, width, height)

	for _, ext := range extensions {
		diskPath := filepath.Join(modelDir, subdir, modelStub+ext)
		networkUrl := baseUrl + "/" + subdir + "/" + modelStub + ext
		if _, err := os.Stat(diskPath); os.IsNotExist(err) {
			logs.Infof("Downloading %v to %v", networkUrl, diskPath)
			if err := downloadFile(networkUrl, diskPath); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
	}

	return nil
}

// LoadModel loads a neural network from disk.
// If the model consists of several files, then modelName is the base filename, without the extensions.
func LoadModel(logs logs.Log, device *nnaccel.Device, modelDir, modelName string, width, height int, threadingMode nn.ThreadingMode, modelSetup *nn.ModelSetup) (nn.ObjectDetector, error) {
	// modelName examples:
	// yolov8m
	// yolo11s   (with yolo 11 they stopped using the "v" in the name)

	// width/height examples:
	// 320, 256
	// 640, 480

	if err := DownloadModel(logs, device, modelDir, modelName, width, height); err != nil {
		return nil, fmt.Errorf("Download failed: %w", err)
	}

	modelSubDir, modelExt := ModelFiles(device, modelName)

	// examples:
	// modelName		yolov8s
	// modelDir			/home/user/cyclops/models
	// modelSubDir		"coco/ncnn"
	// modelExt			[".param", ".bin"]
	// width			320
	// height			256

	fullPathBase := filepath.Join(modelDir, modelSubDir, ModelStub(modelName, width, height))
	config, err := nn.LoadModelConfig(fullPathBase + ".json")
	if err != nil {
		return nil, err
	}

	if device != nil {
		fullModelFilename := fullPathBase
		if len(modelExt) == 1 {
			// eg  modelExt[0] = ".hef"
			fullModelFilename += modelExt[0]
		}
		model, err := device.LoadModel(fullModelFilename, modelSetup)
		if err == nil {
			return model, nil
		} else {
			logs.Warnf("Failed to load accelerated NN model '%v': %v", modelName, err)
			logs.Infof("Falling back to ncnn")
		}
	}

	_, eparam := os.Stat(fullPathBase + ".param")
	_, ebin := os.Stat(fullPathBase + ".bin")

	if eparam == nil && ebin == nil {
		// NCNN file
		return ncnn.NewDetector(config, threadingMode, fullPathBase+".param", fullPathBase+".bin")
	} else {
		return nil, fmt.Errorf("Unrecognized NN model type %v", fullPathBase)
	}
}

func LoadAccelerators(logs logs.Log, enableHailo bool) {
	if isLoaded {
		logs.Warnf("Accelerators already loaded")
		return
	}
	isLoaded = true
	logs.Infof("Loading NN accelerators")
	var err error
	if enableHailo {
		hailoAccel, err = nnaccel.Load("hailo")
		if err != nil {
			logs.Infof("Failed to load Hailo NN accelerator: %v", err)
		} else {
			logs.Infof("Loaded Hailo NN accelerator")
		}
	} else {
		logs.Infof("Hailo disabled - skipping")
	}
}
