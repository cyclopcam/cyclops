package monitor

import (
	"math"

	"github.com/cyclopcam/cyclops/pkg/mybits"
)

// This function runs on its own thread, and monitors for any events that trigger the alarm.
func (m *Monitor) alarmer() {
	inChan := m.AddWatcherAllCameras()

runloop:
	for {
		select {
		case event := <-inChan:
			m.analyzeFrameForAlarmTrigger(event)
		case <-m.alarmingStop:
			break runloop
		}
	}

	m.RemoveWatcherAllCameras(inChan)
	m.alarmingStopped <- true
}

func (m *Monitor) analyzeFrameForAlarmTrigger(event *AnalysisState) {
	var objectBitmap []byte
	for _, obj := range event.Objects {
		if obj.Genuine == 0 || m.nnClassList[obj.Class] != "person" {
			continue
		}
		camera := m.cameraByID(event.CameraID)
		dz := camera.detectionZone
		trigger := false
		if dz != nil {
			// Compute the binary AND of the object rectangle with the detection zone bitmap.
			// If enough pixels are lit, trigger the alarm.
			if objectBitmap == nil {
				objectBitmap = make([]byte, dz.Width*dz.Height/8)
			} else {
				clear(objectBitmap)
			}
			xscale := float64(dz.Width) / float64(event.Input.ImageWidth)
			yscale := float64(dz.Height) / float64(event.Input.ImageHeight)
			box := obj.LastFrame().Box
			x1 := int(math.Floor(float64(box.X) * xscale))
			y1 := int(math.Floor(float64(box.Y) * yscale))
			x2 := int(math.Ceil(float64(box.X2()) * xscale))
			y2 := int(math.Ceil(float64(box.Y2()) * yscale))
			x1 = max(0, x1)
			y1 = max(0, y1)
			x2 = min(dz.Width, x2)
			y2 = min(dz.Height, y2)
			mybits.BitmapFillRect(objectBitmap, dz.Width, x1, y1, x2-x1, y2-y1)
			if mybits.AndBitmapsNonZero(objectBitmap, dz.Active) {
				trigger = true
			}
		} else {
			trigger = true
		}

		if trigger {
			alarmEvent := &AlarmEvent{
				CameraID: event.CameraID,
				Time:     event.Input.FramePTS,
			}
			m.sendToAlarmWatchers(alarmEvent)
		}
	}
}
