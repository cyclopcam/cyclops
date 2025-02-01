package monitor

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
		if dz != nil {
			// Compute the binary AND of the object rectangle with the detection zone bitmap.
			// If enough pixels are lit, trigger the alarm.
			if objectBitmap == nil {
				objectBitmap = make([]byte, dz.Width*dz.Height/8)
			} else {
				clear(objectBitmap)
			}
			box := obj.LastFrame().Box
			box.X = max(box.X, 0)
			box.Y = max(box.Y, 0)
			// .... convert box coordinate system to bitmap coordinate system
			//box.Width = min(box.Width, camera.Width)
			//mybits.BitmapFillRect(objectBitmap, dz.Width, int(box.X), int(box.Y), int(box.Width), int(box.Height))
			//onbits := mybits.AndBitmaps(objectBitmap, dz.Active)
			//if onbits != 0 {
			//	// alarm!
			//}
		}
	}
}
