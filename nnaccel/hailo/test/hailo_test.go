package hailotest

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"
	"unsafe"

	"github.com/bmharper/cimg/v2"
	"github.com/cyclopcam/cyclops/pkg/nn"
	"github.com/cyclopcam/cyclops/pkg/nnaccel"
	"github.com/stretchr/testify/require"
)

const repoRoot = "../../.."

// modelName is eg "yolov8s_640_640"
func loadModel(modelName string, batchSize int) (*nnaccel.Accelerator, *nnaccel.Model, error) {
	device, err := nnaccel.Load("hailo")
	if err != nil {
		return nil, nil, err
	}

	setup := nn.NewModelSetup()
	setup.BatchSize = batchSize
	model, err := device.LoadModel(filepath.Join(repoRoot, "models/coco/hailo/8L", modelName)+".hef", setup)
	if err != nil {
		return nil, nil, err
	}

	return device, model, nil
}

// Replicate the same image 'batchSize' times into a giant batch image buffer.
func replicateImageIntoBatch(img *cimg.Image, batchSize int) (batchStride int, wholeBatch []byte) {
	rgb := img.ToRGB()
	imgBytes := rgb.Width * rgb.Height * rgb.NChan()
	batchStride = nnaccel.RoundUpToPageSize(imgBytes)
	wholeBatch = make([]byte, batchStride*batchSize)
	for i := 0; i < batchSize; i++ {
		batchEl := wholeBatch[i*batchStride : (i+1)*batchStride]
		copy(batchEl, rgb.Pixels)
	}
	return
}

func BenchmarkObjectDetection(b *testing.B) {
	modelName := "yolov8m_640_640"
	batchSize := 8
	// Maximum number of hailo jobs that we'll have going in parallel.
	// It's important to set this to at least 2 to stress parallel operation.
	// Also, when maxParallelJobs = 1, you'll get slightly lower FPS numbers,
	// because you won't be able to start the next job while the previous
	// one is still running.
	// On yolov8m with Hailo8L on Pi5, I get these numbers:
	// 44.7 FPS with maxParallelJobs = 1
	// 50.8 FPS with maxParallelJobs = 2
	maxParallelJobs := 2

	_, model, err := loadModel(modelName, batchSize)
	require.NoError(b, err)

	img, _ := cimg.ReadFile(filepath.Join(repoRoot, "testdata/yard-640x640.jpg"))
	batchStride, wholeBatch := replicateImageIntoBatch(img, batchSize)

	// The first inference run is slow, so don't include that in the benchmark
	job, _ := model.Run(batchSize, batchStride, img.Width, img.Height, img.NChan(), img.Width*img.NChan(), unsafe.Pointer(&wholeBatch[0]))
	job.Wait(5 * time.Second)
	job.Close()
	b.ResetTimer()

	jobQueue := make(chan bool, maxParallelJobs-1)
	doneQueue := make(chan bool, 10+b.N)
	runTicket := make(chan bool, maxParallelJobs)
	for i := 0; i < maxParallelJobs; i++ {
		runTicket <- true
	}

	runJob := func() {
		<-runTicket
		var job *nnaccel.AsyncJob
		for i := 0; i < 20; i++ {
			var err error
			job, err = model.Run(batchSize, batchStride, img.Width, img.Height, img.NChan(), img.Width*img.NChan(), unsafe.Pointer(&wholeBatch[0]))
			if err == nil {
				break
			} else if i == 19 {
				panic(err)
			}
			b.Logf("Sleeping for %v", time.Millisecond*(1<<i))
			time.Sleep(time.Millisecond * (1 << i))
		}
		require.True(b, job.Wait(10*time.Second))
		job.GetObjectDetections(0)
		job.Close()
		runTicket <- true
		doneQueue <- true
	}

	// consume the jobQueue
	go func() {
		for {
			v := <-jobQueue
			if !v {
				// exit
				return
			}
			// Spin up a new goroutine for every job
			go runJob()
		}
	}()

	// fill the queue with N requests
	for i := 0; i < b.N; i++ {
		jobQueue <- true // run a job
	}
	jobQueue <- false // exit

	// drain doneQueue
	for i := 0; i < b.N; i++ {
		<-doneQueue
	}

	nFrames := b.N * batchSize
	b.Logf("FPS: %v (%v / %v)", float64(nFrames)/float64(b.Elapsed().Seconds()), nFrames, b.Elapsed().Seconds())

	model.Close()
}

func TestObjectDetection(t *testing.T) {
	for _, batchSize := range []int{1, 2, 8} {
		_, model, err := loadModel("yolov8s_640_640", batchSize)
		require.NoError(t, err)

		fmt.Printf("cache breaker 3\n")

		img, err := cimg.ReadFile(filepath.Join(repoRoot, "testdata/yard-640x640.jpg"))
		require.NoError(t, err)
		img = img.ToRGB()

		// 1st run, where everything is as straightforward and 'default' as possible

		batchStride, wholeBatch := replicateImageIntoBatch(img, batchSize)

		job, err := model.Run(batchSize, batchStride, img.Width, img.Height, img.NChan(), img.Stride, unsafe.Pointer(&wholeBatch[0]))
		require.NoError(t, err)

		// Wait for async job to complete
		require.True(t, job.Wait(time.Second))

		for batchEl := 0; batchEl < batchSize; batchEl++ {
			dets, err := job.GetObjectDetections(batchEl)
			require.NoError(t, err)
			if batchSize == 1 {
				for _, d := range dets {
					t.Logf("Class %v (confidence %.3f): %v,%v - %v,%v", d.Class, d.Confidence, d.Box.X, d.Box.Y, d.Box.X+d.Box.Width, d.Box.Y+d.Box.Height)
				}
			}

			expectDets := []nn.ObjectDetection{
				{Class: 0, Box: nn.Rect{X: 452, Y: 244, Width: 75, Height: 222}},
				{Class: 2, Box: nn.Rect{X: 61, Y: 205, Width: 336, Height: 159}},
			}
			require.Equal(t, len(expectDets), len(dets))
			for i := 0; i < len(expectDets); i++ {
				//t.Logf("iou %v\n", expectDets[i].Box.IOU(dets[i].Box))
				require.Equal(t, expectDets[i].Class, dets[i].Class)
				require.GreaterOrEqualf(t, expectDets[i].Box.IOU(dets[i].Box), float32(0.9), "IOU too low")
			}
		}

		job.Close()

		// I have removed this functionality. It is not supported by the actual accelerator,
		// we were just covering for it in C++ code. No point doing that. Rather force users to
		// pack their images tightly.
		/*
			// Test a 2nd run, where we send a crop of the image

			// But first, we create a LARGER image, because we can't send the NN an image that is not
			// the exact size it expected.
			bigImg := cimg.NewImage(img.Width+64, img.Height+64, cimg.PixelFormatRGB)
			err = bigImg.CopyImage(rgb, 32, 32)
			require.NoError(t, err)

			// And then out of the larger image, we crop a 640x640 rectangle.
			// This tests the ability of the NN accelerator to handle a stride that is not equal to width*nchan.
			cropRect := nn.MakeRect(32, 32, img.Width, img.Height)
			cropped := nn.WholeImage(bigImg.NChan(), bigImg.Pixels, bigImg.Width, bigImg.Height).Crop(int(cropRect.X), int(cropRect.Y), int(cropRect.X2()), int(cropRect.Y2()))
			require.Equal(t, img.Width, cropped.CropWidth)
			require.Equal(t, img.Height, cropped.CropHeight)
			dets, err = model.DetectObjects(cropped, nn.NewDetectionParams())
			require.NoError(t, err)

			require.Equal(t, len(expectDets), len(dets))
			for i := 0; i < len(expectDets); i++ {
				//t.Logf("iou %v\n", expectDets[i].Box.IOU(dets[i].Box))
				require.Equal(t, expectDets[i].Class, dets[i].Class)
				require.GreaterOrEqualf(t, expectDets[i].Box.IOU(dets[i].Box), float32(0.9), "IOU too low")
			}
		*/

		model.Close()
	}

	//fmt.Printf("Done\n")
}
