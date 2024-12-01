package monitor

import (
	"math"
	"sync/atomic"
	"time"
)

type nnPerfStats struct {
	avgTimeNSPerFrameNNPrep atomic.Int64 // Average time (ns) per frame, for prep of an image before it hits the NN
	avgTimeNSPerFrameNNDet  atomic.Int64 // Average time (ns) per frame, for just the neural network (time inside a thread)
}

// State internal to the NN frame reader, for each camera
type frameReaderCameraState struct {
	mcam               *monitorCamera
	lastFrameID        int64 // Last frame we've seen from this camera
	numFramesTotal     int64 // Number of frames from this camera that we've seen
	numFramesProcessed int64 // Number of frames from this camera that we've analyzed
}

func frameReaderStats(cameraStates []*frameReaderCameraState) (totalFrames, totalProcessed int64) {
	for _, state := range cameraStates {
		totalFrames += state.numFramesTotal
		totalProcessed += state.numFramesProcessed
	}
	return
}

// Read camera frames and send them off for analysis.
// A single thread runs this operation.
func (m *Monitor) readFrames() {
	// Make our own private copy of cameras.
	// If the list of cameras changes, then SetCameras() will stop and restart this function
	m.camerasLock.Lock()
	looperCameras := []*frameReaderCameraState{}
	for _, mcam := range m.cameras {
		looperCameras = append(looperCameras, &frameReaderCameraState{
			mcam: mcam,
		})
	}
	m.camerasLock.Unlock()

	// Maintain camera index outside of main loop, so that we're not
	// biased towards processing the frames of the first camera(s).
	// I still need to figure out how to boost priority for cameras
	// that have likely activity in them.
	icam := uint(0)

	lastStats := time.Now()

	nStats := 0
	for !m.mustStopFrameReader.Load() {
		idle := true
		// Why do we have this inner loop?
		// We keep it so that we can detect when to idle.
		// If we complete a loop over all looperCameras, and we didn't have any work to do,
		// then we idle for a few milliseconds.
		for i := 0; i < len(looperCameras); i++ {
			if m.mustStopFrameReader.Load() {
				break
			}
			// SYNC-NN-THREAD-QUEUE-MIN-SIZE
			if len(m.nnThreadQueue) >= cap(m.nnThreadQueue) {
				// Our NN queue is full, so drop frames.
				break
			}

			// It's vital that this incrementing happens after the queue check above,
			// otherwise you don't get round robin behaviour.
			icam = (icam + 1) % uint(len(looperCameras))
			camState := looperCameras[icam]
			mcam := camState.mcam

			//m.Log.Infof("%v", icam)
			img, imgID, imgPTS := mcam.camera.LowDecoder.GetLastImageIfDifferent(camState.lastFrameID)
			if img != nil {
				if camState.lastFrameID == 0 {
					camState.numFramesTotal++
				} else {
					camState.numFramesTotal += imgID - camState.lastFrameID
				}
				//m.Log.Infof("Got image %d from camera %s (%v / %v)", imgID, mcam.camera.Name, camState.numFramesProcessed, camState.numFramesTotal)
				camState.numFramesProcessed++
				camState.lastFrameID = imgID
				idle = false
				m.nnThreadQueue <- monitorQueueItem{
					isHQ:     false,
					monCam:   mcam,
					yuv:      img,
					rgb:      nil,
					framePTS: imgPTS,
				}
			}
		}
		if m.mustStopFrameReader.Load() {
			break
		}
		if idle {
			time.Sleep(5 * time.Millisecond)
		}

		interval := 10 * math.Pow(1.5, float64(nStats))
		interval = max(interval, 5)
		interval = min(interval, 3600)
		if time.Now().Sub(lastStats) > time.Duration(interval)*time.Second {
			nStats++
			totalFrames, totalProcessed := frameReaderStats(looperCameras)
			lq := &m.nnPerfStatsLQ
			hq := &m.nnPerfStatsLQ
			m.Log.Infof("%.0f%% frames analyzed by LQ NN. %v Threads. Times per frame: (%.1f ms Prep, %.1f ms NN)",
				100*float64(totalProcessed)/float64(totalFrames),
				m.numNNThreads,
				float64(lq.avgTimeNSPerFrameNNPrep.Load())/1e6,
				float64(lq.avgTimeNSPerFrameNNDet.Load())/1e6,
			)
			m.Log.Infof("HQ validation network: %.1f ms Prep, %.1f ms NN", float64(hq.avgTimeNSPerFrameNNPrep.Load())/1e6, float64(hq.avgTimeNSPerFrameNNDet.Load())/1e6)
			lastStats = time.Now()
		}
	}
	close(m.frameReaderStopped)
}
