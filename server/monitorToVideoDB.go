package server

// It doesn't seem right to make 'videodb' dependent on 'monitor' or vice versa,
// so we hook them up via this intermediate thread here.
func (s *Server) attachMonitorToVideoDB() {
	go func() {
		s.Log.Infof("Monitor -> VideoDB thread starting")
		incoming := s.monitor.AddWatcherAllCameras()
		keepRunning := true
		for keepRunning {
			select {
			case <-s.ShutdownStarted:
				keepRunning = false
			case msg := <-incoming:
				classes := s.monitor.AllClasses()
				cam := s.LiveCameras.CameraFromID(msg.CameraID)
				if cam == nil {
					s.Log.Warnf("Ignoring monitor message for unknown camera %v", msg.CameraID)
					continue
				}
				for _, obj := range msg.Objects {
					s.videoDB.ObjectDetected(cam.LongLivedName(), obj.ID, obj.Box, classes[obj.Class], obj.LastSeen)
				}
			}
		}
		s.monitor.RemoveWatcherAllCameras(incoming)
		s.Log.Infof("Monitor -> VideoDB thread exiting")
		close(s.monitorToVideoDBClosed)
	}()
}
