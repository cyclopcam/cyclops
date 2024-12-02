package monitor

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/bmharper/cimg/v2"
	"github.com/cyclopcam/cyclops/pkg/accel"
	"github.com/cyclopcam/cyclops/pkg/nn"
	"github.com/cyclopcam/cyclops/pkg/nnaccel"
	"github.com/cyclopcam/cyclops/server/camera"
	"github.com/cyclopcam/cyclops/server/perfstats"
)

type NNThreadState int

const (
	NNThreadStateIdle NNThreadState = iota
	NNThreadStateRunning
)

// State of a single thread that performs NN detections
// When running on desktop CPUs, there are typically a handful of these (eg the number of physical cores)
// When running with an NN accelerator (eg Hailo), we create just one NN thread.
// Crucially, all of state inside here is owned by a single thread, so there's no
// need to coordinate access to it.
type nnThread struct {
	lastErrAt  time.Time
	detectorLQ nnDetectorState
	detectorHQ nnDetectorState
}

// State of an NN detector (each NN thread has two of these: LQ and HQ)
type nnDetectorState struct {
	detectionParams *nn.DetectionParams
	detector        nn.ObjectDetector
	nnWidth         int // NN input image width
	nnHeight        int // NN input image height
	batchSize       int // Number of items in batch (typically 1 or 8)
	batchStride     int // Bytes between each image in a batch
	wholeBatchImage []byte
	resizeQuality   ResizeQuality
	batch           []nnBatchItem
}

// An image that has been loaded into a batch (eg 1 of 8)
type nnBatchItem struct {
	monCam       *monitorCamera
	imgID        int64
	yuv          *accel.YUVImage
	rgb          *cimg.Image // rgb image directly converted from yuv (i.e. same resolution, etc).
	framePTS     time.Time
	xformRgbToNN nn.ResizeTransform
}

// Run a neural network processing thread
// threadIdx is a zero-based index into Monitor.nnThreadState
func (t *nnThread) run(m *Monitor, threadIdx int) {
	t.lastErrAt = time.Time{}

	t.detectorLQ.init(m.nnModelSetupLQ, m.nnDetectorLQ)
	t.detectorHQ.init(m.nnModelSetupHQ, m.nnDetectorHQ)

	for frameCount := 0; true; frameCount++ {
		// Read the next item from the queue.
		var d *nnDetectorState
		var perf *nnPerfStats
		// We scope the variables in here to ensure that they don't leak out, and mistakenly
		// process just this latest item, instead of an item from the batch.
		{
			// This thread state idle/running is obviously a sloppy synchronization mechanism, because we've got
			// gaps in between our channel read and our toggling of this state. But it was created for
			// unit tests to know when the NN threads are quiescent. Please don't try using it for stricter things.
			m.nnThreadState[threadIdx] = NNThreadStateIdle
			item, ok := <-m.nnThreadQueue
			if !ok {
				break
			}
			m.nnThreadState[threadIdx] = NNThreadStateRunning
			if item.isHQ {
				d = &t.detectorHQ
				perf = &m.nnPerfStatsHQ
			} else {
				d = &t.detectorLQ
				perf = &m.nnPerfStatsLQ
			}
			batchEl := len(d.batch)
			nnBlock := d.wholeBatchImage[batchEl*d.batchStride : (batchEl+1)*d.batchStride]
			start := time.Now()
			xformRgbToNN, rgbPure, rgbNN := m.prepareImageForNN(item.yuv, item.rgb, d.nnWidth, d.nnHeight, nnBlock, d.resizeQuality)
			// Note that rgbNN is actually a window into wholeBatchImage, which is why we don't need to store it.
			perfstats.UpdateMovingAverage(&perf.avgTimeNSPerFrameNNPrep, time.Now().Sub(start).Nanoseconds())
			if m.debugDumpFrames {
				m.dumpFrame(rgbPure, item.monCam.camera, "rgb")
				m.dumpFrame(rgbNN, item.monCam.camera, "nn")
			}
			d.batch = append(d.batch, nnBatchItem{
				monCam:       item.monCam,
				imgID:        item.imgID,
				yuv:          item.yuv,
				rgb:          rgbPure,
				framePTS:     item.framePTS,
				xformRgbToNN: xformRgbToNN,
			})
		}
		if len(d.batch) < d.batchSize {
			continue
		}
		imageBatch := nn.MakeImageBatch(d.batchSize, d.batchStride, d.nnWidth, d.nnHeight, 3, d.nnWidth*3, d.wholeBatchImage)
		start := time.Now()
		batchResult, err := d.detector.DetectObjects(imageBatch, d.detectionParams)
		perfstats.UpdateMovingAverage(&perf.avgTimeNSPerFrameNNDet, time.Now().Sub(start).Nanoseconds())
		if err != nil {
			if time.Now().Sub(t.lastErrAt) > 15*time.Second {
				m.Log.Errorf("Error detecting objects: %v", err)
				t.lastErrAt = time.Now()
			}
		} else {
			for i := 0; i < len(d.batch); i++ {
				input := &d.batch[i]
				objects := batchResult[i]
				input.xformRgbToNN.ApplyBackward(objects)
				//m.Log.Infof("Camera %v detected %v objects", mcam.camera.ID, len(objects))
				result := &nn.DetectionResult{
					CameraID:    input.monCam.camera.ID(),
					ImageWidth:  input.yuv.Width,
					ImageHeight: input.yuv.Height,
					Objects:     objects,
					FramePTS:    input.framePTS,
				}
				input.monCam.lock.Lock()
				input.monCam.lastDetection = result
				input.monCam.lastImg = input.rgb
				input.monCam.lock.Unlock()

				if len(m.analyzerQueue) >= cap(m.analyzerQueue)*9/10 {
					// We do not expect this
					m.Log.Warnf("NN analyzer queue is falling behind - dropping frames")
				} else {
					m.analyzerQueue <- analyzerQueueItem{
						isHQ:      d == &t.detectorHQ,
						imgID:     input.imgID,
						monCam:    input.monCam,
						yuv:       input.yuv,
						rgb:       input.rgb,
						detection: result,
					}
				}
			}
		}
		d.batch = d.batch[:0]
	}

	m.nnThreadStopWG.Done()
}

func (d *nnDetectorState) init(setup *nn.ModelSetup, detector nn.ObjectDetector) {
	// For Hailo/Accelerators, these parameters are defined at model setup time,
	// but for NCNN, we control them with each detection. We should probably get
	// rid of the per-detection mechanism with NCNN so that it all goes in through
	// the same path.
	d.detectionParams = nn.NewDetectionParams()
	d.detectionParams.ProbabilityThreshold = setup.ProbabilityThreshold
	d.detectionParams.NmsIouThreshold = setup.NmsIouThreshold

	// batchStride = distance between images in our big memory block.
	// Each image must start on a page boundary, and be a multiple of a whole page size.
	d.nnWidth = detector.Config().Width
	d.nnHeight = detector.Config().Height
	d.batchSize = setup.BatchSize
	d.batchStride = nnBatchImageStride(d.nnWidth, d.nnHeight)

	// Allocate one big block of memory that will hold all of the images in one batch.
	// I tried at first to use individual blocks of memory (one for each image), but this
	// gets nasty when you try to send these pointers via cgo, because Go's rules
	// prohibit sending a void**.
	// An additional complexity wrinkle here is that the Hailo accelerators want
	// each image to be page aligned, so we need to take that into consideration.
	// One big block of memory feels like the right solution. I'm hoping it might
	// also make things simpler if we want to support CUDA. The most salient
	// principle is that you're not in control of your image memory when working
	// with accelerators, so might as well get used to that.

	// A typical size here will be:
	// 640x640x3 * 8 = 12MB
	d.wholeBatchImage = nnaccel.PageAlignedAlloc(d.batchSize * d.batchStride)

	// We might want to make this decision based on the load of the system, or it's
	// static performance. For example, on a desktop class CPU, it's probably worthwhile
	// to do higher quality resampling, but on a Pi maybe not. Ideally we should be
	// using the GPU on the Pi, but that's for another day.
	// Hmm.. after looking at this more, I'm leaning towards always using the "low quality"
	// filter, because it's the sharpest. My guess is that the effect on the NN performance
	// is so small that it's probably worth using the sharpest, fastest filter all the time.
	d.resizeQuality = ResizeQualityLow

	d.detector = detector
}

func (m *Monitor) dumpFrame(rgb *cimg.Image, cam *camera.Camera, variant string) {
	frameKey := strconv.FormatInt(cam.ID(), 10) + "-" + variant
	m.dumpLock.Lock()
	hasDumped := m.hasDumpedFrame[frameKey]
	if hasDumped {
		m.dumpLock.Unlock()
		return
	}
	m.hasDumpedFrame[frameKey] = true
	m.dumpLock.Unlock()

	b, _ := cimg.Compress(rgb, cimg.MakeCompressParams(cimg.Sampling(cimg.Sampling420), 95, cimg.Flags(0)))
	os.WriteFile(fmt.Sprintf("frame-%v-%v.jpg", cam.Name(), variant), b, 0644)
}

// Return the number of bytes between each RGB image in a batch
func nnBatchImageStride(nnWidth, nnHeight int) int {
	return (nnWidth*nnHeight*3 + nnaccel.PageSize() - 1) & ^(nnaccel.PageSize() - 1)
}
