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

// An NN processing thread
func (m *Monitor) nnThread() {
	lastErrAt := time.Time{}

	// I was originally tempted to reuse the same RGB image across iterations
	// of the loop (the 'rgb' variable). However, this doesn't actually help
	// performance at all, since we need to store a unique lastImg inside the
	// monitorCamera object.
	// I mean.. it did perhaps help performance a tiny bit, but it introduced
	// the bug of returning the incorrect lastImg for a camera (all cameras
	// would share the same lastImg).

	// Resizing image for NN inference:
	// When implementing support for the Hailo8L on Raspberry Pi5, the easiest
	// thing to do was to use the pretrained YOLOv8 model, which has an input
	// size of 640x640. Our cameras are typically setup to emit 2nd stream
	// images as a lower resolution (eg 320 x 256). Until this time, my NCNN
	// YOLOv8 used an input resolution of 320 x 256, so it perfectly matched
	// the camera 2nd streams. So my decision at the time of implementing
	// support for the Hailo8L was to simply add black padding around the
	// 320x256 images, to make them 640x640. This is not ideal. We should
	// either be using larger 2nd stream images from the camera, or creating
	// a custom Hailo8L YOLOv8 model with a smaller input resolution.
	// But now you know why we do it this way. It's not the best, just the
	// easiest, an good enough for now.
	// UPDATE: Since writing that, we now resize our images before sending
	// them to the NN. You take too much of an NN accuracy hit if you do
	// anything else.

	// For Hailo/Accelerators, these parameters are defined at model setup time,
	// but for NCNN, we control them with each detection. We should probably get
	// rid of the per-detection mechanism with NCNN so that it all goes in through
	// the same path.
	detectionParams := nn.NewDetectionParams()
	detectionParams.ProbabilityThreshold = m.nnModelSetup.ProbabilityThreshold
	detectionParams.NmsIouThreshold = m.nnModelSetup.NmsIouThreshold

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

	// batchStride = distance between images in our big memory block.
	// Each image must start on a page boundary, and be a multiple of a whole page size.
	nnWidth := m.detector.Config().Width
	nnHeight := m.detector.Config().Height
	batchStride := nnBatchImageStride(nnWidth, nnHeight)

	// A typical size here will be:
	// 640x640x3 * 8 = 12MB
	wholeBatchImage := nnaccel.PageAlignedAlloc(m.nnBatchSize * batchStride)

	// We might want to make this decision based on the load of the system, or it's
	// static performance. For example, on a desktop class CPU, it's probably worthwhile
	// to do higher quality resampling, but on a Pi maybe not. Ideally we should be
	// using the GPU on the Pi, but that's for another day.
	// Hmm.. after looking at this more, I'm leaning towards always using the "low quality"
	// filter, because it's the sharpest. My guess is that the effect on the NN performance
	// is so small that it's probably worth using the sharpest, fastest filter all the time.
	resizeQuality := ResizeQualityLow

	type batchItem struct {
		monCam       *monitorCamera
		yuv          *accel.YUVImage
		framePTS     time.Time
		xformRgbToNN nn.ResizeTransform
		rgbPure      *cimg.Image
	}

	// Save up enough images until we have a full batch
	batch := []batchItem{}

	for frameCount := 0; true; frameCount++ {
		// Read the next item from the queue.
		// We scope the variables in here to ensure that they don't leak out, and we mistakenly
		// process just this one item instead of an item from the batch.
		{
			item, ok := <-m.nnThreadQueue
			if !ok {
				break
			}
			yuv := item.image
			batchEl := len(batch)
			nnBlock := wholeBatchImage[batchEl*batchStride : (batchEl+1)*batchStride]
			xformRgbToNN, rgbPure, rgbNN := m.prepareImageForNN(yuv, nnBlock, resizeQuality)
			if m.debugDumpFrames {
				m.dumpFrame(rgbPure, item.monCam.camera, "rgb")
				m.dumpFrame(rgbNN, item.monCam.camera, "nn")
			}
			batch = append(batch, batchItem{
				monCam:       item.monCam,
				yuv:          yuv,
				framePTS:     item.framePTS,
				xformRgbToNN: xformRgbToNN,
				rgbPure:      rgbPure,
			})
		}
		if len(batch) < m.nnBatchSize {
			continue
		}
		imageBatch := nn.MakeImageBatch(m.nnBatchSize, batchStride, nnWidth, nnHeight, 3, nnWidth*3, wholeBatchImage)
		start := time.Now()
		batchResult, err := m.detector.DetectObjects(imageBatch, detectionParams)
		perfstats.UpdateMovingAverage(&m.avgTimeNSPerFrameNNDet, time.Now().Sub(start).Nanoseconds())
		if err != nil {
			if time.Now().Sub(lastErrAt) > 15*time.Second {
				m.Log.Errorf("Error detecting objects: %v", err)
				lastErrAt = time.Now()
			}
		} else {
			for i := 0; i < len(batch); i++ {
				input := &batch[i]
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
				input.monCam.lastImg = input.rgbPure
				input.monCam.lock.Unlock()

				if len(m.analyzerQueue) >= cap(m.analyzerQueue)*9/10 {
					// We do not expect this
					m.Log.Warnf("NN analyzer queue is falling behind - dropping frames")
				} else {
					m.analyzerQueue <- analyzerQueueItem{
						monCam:    input.monCam,
						detection: result,
					}
				}
			}
		}
		batch = batch[:0]
	}

	m.nnThreadStopWG.Done()
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
