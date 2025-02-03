package server

import "github.com/cyclopcam/cyclops/server/configdb"

// Listen for alarm triggering events from the monitor, and take appropriate action.
func (s *Server) runAlarmHandler() {
	go func() {
		alarmEventChan := s.monitor.AddAlarmWatcher()
	runLoop:
		for {
			select {
			case ev := <-alarmEventChan:
				state := s.configDB.TriggerAlarmIfArmed()
				if state == configdb.AlarmTriggerTripped {
					camera := s.LiveCameras.CameraFromID(ev.CameraID)
					if camera != nil {
						s.Log.Warnf("Alarm triggered on camera %v", camera.Name())
					} else {
						s.Log.Warnf("Alarm triggered on unrecognized camera")
					}
				}
			case <-s.ShutdownStarted:
				break runLoop
			}
		}
		s.monitor.RemoveAlarmWatcher(alarmEventChan)
		s.Log.Infof("Alarm handler exiting")
		close(s.alarmHandlerClosed)
	}()
}
