package hailotest

import (
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

// modelName is eg "yolov8s"
func loadModel(modelName string, batchSize int) (*nnaccel.Accelerator, *nnaccel.Model, error) {
	device, err := nnaccel.Load("hailo")
	if err != nil {
		return nil, nil, err
	}

	setup := nn.NewModelSetup()
	setup.BatchSize = batchSize
	model, err := device.LoadModel(filepath.Join(repoRoot, "models"), modelName, setup)
	if err != nil {
		return nil, nil, err
	}

	return device, model, nil
}

func BenchmarkObjectDetection(b *testing.B) {
	modelName := "yolov8s"
	batchSize := 1

	_, model, err := loadModel(modelName, batchSize)
	require.NoError(b, err)

	img, _ := cimg.ReadFile(filepath.Join(repoRoot, "testdata/yard-640x640.jpg"))
	rgb := img.ToRGB()
	batch := make([]byte, batchSize*img.Width*img.Height*img.NChan())
	for i := 0; i < batchSize; i++ {
		copy(batch[i*img.Width*img.Height*img.NChan():], rgb.Pixels)
	}

	// The first inference run is slow, so don't include that in the benchmark
	job, _ := model.Run(batchSize, img.Width, img.Height, img.NChan(), 0, unsafe.Pointer(&batch[0]))
	job.Wait(5 * time.Second)
	job.Close()
	b.ResetTimer()

	maxParallelJobs := 1
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
			job, err = model.Run(batchSize, img.Width, img.Height, img.NChan(), 0, unsafe.Pointer(&batch[0]))
			if err == nil {
				break
			} else if i == 19 {
				panic(err)
			}
			b.Logf("Sleeping for %v", time.Millisecond*(1<<i))
			time.Sleep(time.Millisecond * (1 << i))
		}
		job.Wait(time.Second)
		job.GetObjectDetections()
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

	b.Logf("FPS: %v (%v / %v)", float64(b.N)/float64(b.Elapsed().Seconds()), b.N, b.Elapsed().Seconds())

	model.Close()
}

func TestObjectDetection(t *testing.T) {
	_, model, err := loadModel("yolov8s", 1)
	require.NoError(t, err)

	img, err := cimg.ReadFile(filepath.Join(repoRoot, "testdata/yard-640x640.jpg"))
	require.NoError(t, err)
	rgb := img.ToRGB() // might already be RGB, but just to be sure

	// 1st run, where everything is as straightforward and 'default' as possible

	job, err := model.Run(1, img.Width, img.Height, img.NChan(), 0, unsafe.Pointer(&rgb.Pixels[0]))
	require.NoError(t, err)

	// Wait for async job to complete
	require.True(t, job.Wait(time.Second))

	dets, err := job.GetObjectDetections()
	require.NoError(t, err)
	for _, d := range dets {
		t.Logf("Class %v (confidence %.3f): %v,%v - %v,%v", d.Class, d.Confidence, d.Box.X, d.Box.Y, d.Box.X+d.Box.Width, d.Box.Y+d.Box.Height)
	}
	job.Close()

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

	// Test a 2nd run, where we send a crop of the image

	// But first, we create a LARGER image, because we can't send the NN an image that is not
	// the exact size it expected.
	bigImg := cimg.NewImage(img.Width+64, img.Height+64, cimg.PixelFormatRGB)
	err = bigImg.CopyImage(rgb, 32, 32)
	require.NoError(t, err)

	// And then out of the larger image, we crop a 640x640 rectangle.
	// This tests the ability of the NN accelerator to handle a stride that is not equal to width*nchan.
	cropRect := nn.MakeRect(32, 32, img.Width, img.Height)
	cropped := nn.WholeImage(bigImg.NChan(), bigImg.Pixels, bigImg.Width, bigImg.Height).Crop(cropRect.X, cropRect.Y, cropRect.X2(), cropRect.Y2())
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

	model.Close()

	//fmt.Printf("Done\n")
}
