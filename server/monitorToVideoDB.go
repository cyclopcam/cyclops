package server

import (
	"time"

	"github.com/cyclopcam/cyclops/pkg/gen"
	"github.com/cyclopcam/cyclops/pkg/nn"
	"github.com/cyclopcam/cyclops/server/monitor"
	"github.com/cyclopcam/cyclops/server/videodb"
)

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
				resolution := [2]int{msg.Input.ImageWidth, msg.Input.ImageHeight}
				if cam == nil {
					s.Log.Warnf("Ignoring monitor message for unknown camera %v", msg.CameraID)
					continue
				}
				for _, obj := range msg.Objects {
					if obj.Genuine {
						s.videoDB.ObjectDetected(cam.LongLivedName(), resolution, obj.ID, obj.Box, obj.Confidence, classes[obj.Class], obj.LastSeen)
					}
				}
			}
		}
		s.monitor.RemoveWatcherAllCameras(incoming)
		s.Log.Infof("Monitor -> VideoDB thread exiting")
		close(s.monitorToVideoDBClosed)
	}()
}

// Synthesize a 'live' monitor.AnalysisState from historical events.
// This is used to show the user what objects were detected in the past.
// We arbitrarily choose to use monitor.AnalysisState as our JSON-serializable
// transmission format, because that was the first one that we built support for
// in the front-end.
func (s *Server) copyEventsToMonitorAnalysis(cameraID int64, events []*videodb.Event, frameTime time.Time) *monitor.AnalysisState {
	analysis := monitor.AnalysisState{
		CameraID: cameraID,
		Input: &nn.DetectionResult{
			CameraID: cameraID,
			FramePTS: frameTime,
			Objects:  make([]nn.ObjectDetection, 0),
		},
		Objects: make([]monitor.TrackedObject, 0),
	}
	for _, e := range events {
		if e.Time.Get().After(frameTime) || e.Time.Get().Add(time.Duration(e.Duration)*time.Millisecond).Before(frameTime) {
			// event doesn't span frameTime, so skip it entirely
			continue
		}
		if e.Detections != nil {
			analysis.Input.ImageWidth = e.Detections.Data.Resolution[0]
			analysis.Input.ImageHeight = e.Detections.Data.Resolution[1]
			for _, d := range e.Detections.Data.Objects {
				// Find the frame closest to frameTime
				frameTimeMilli := frameTime.UnixMilli()
				bestDelta := int64(1<<63 - 1)
				bestPos := -1
				for i, p := range d.Positions {
					posTimeMilli := int64(e.Time) + int64(p.Time)
					delta := gen.Abs(posTimeMilli - frameTimeMilli)
					if delta < bestDelta {
						bestDelta = delta
						bestPos = i
					}
				}
				if bestPos != -1 {
					best := d.Positions[bestPos]
					analysis.Objects = append(analysis.Objects, monitor.TrackedObject{
						ID: d.ID,
						//Class:      d.Class,
						Box: nn.MakeRect(int(best.Box[0]), int(best.Box[1]), int(best.Box[2]-best.Box[0]), int(best.Box[3]-best.Box[1])),
						//Genuine:    true,
						Confidence: best.Confidence,
					})
				}
			}
		}
	}
	return &analysis
}
