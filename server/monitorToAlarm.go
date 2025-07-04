package server

import "github.com/cyclopcam/cyclops/server/eventdb"

// Listen for alarm-triggering events from the monitor, and take appropriate action.
func (s *Server) runAlarmHandler() {
	go func() {
		alarmEventChan := s.monitor.AddAlarmWatcher()
	runLoop:
		for {
			select {
			case ev := <-alarmEventChan:
				if s.eventDB.IsArmedAndUntriggered() {
					s.eventDB.AddEvent(eventdb.EventTypeAlarm, &eventdb.EventDetail{
						Alarm: &eventdb.EventDetailAlarm{
							AlarmType: eventdb.AlarmTypeCameraObject,
							CameraID:  ev.CameraID,
						},
					})
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
