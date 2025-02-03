package monitor

import "github.com/cyclopcam/cyclops/pkg/gen"

// SYNC-WATCHER-CHANNEL-SIZE
const WatcherChannelSize = 100

// Register to receive detection results for a specific camera.
func (m *Monitor) AddWatcher(cameraID int64) chan *AnalysisState {
	m.watchersLock.Lock()
	defer m.watchersLock.Unlock()
	ch := make(chan *AnalysisState, WatcherChannelSize)
	m.watchers[cameraID] = append(m.watchers[cameraID], ch)
	return ch
}

// Unregister from detection results for a specific camera
func (m *Monitor) RemoveWatcher(cameraID int64, ch chan *AnalysisState) {
	m.watchersLock.Lock()
	defer m.watchersLock.Unlock()
	for i, w := range m.watchers[cameraID] {
		if w == ch {
			m.watchers[cameraID] = gen.DeleteFromSliceUnordered(m.watchers[cameraID], i)
			return
		}
	}
	m.Log.Warnf("Monitor.RemoveWatcher failed to find channel for camera %v", cameraID)
}

// Add a watcher that is interested in all camera activity
func (m *Monitor) AddWatcherAllCameras() chan *AnalysisState {
	m.watchersLock.Lock()
	defer m.watchersLock.Unlock()
	ch := make(chan *AnalysisState, WatcherChannelSize)
	m.watchersAllCameras = append(m.watchersAllCameras, ch)
	return ch
}

// Unregister from detection results of all cameras
func (m *Monitor) RemoveWatcherAllCameras(ch chan *AnalysisState) {
	m.watchersLock.Lock()
	defer m.watchersLock.Unlock()
	for i, wch := range m.watchersAllCameras {
		if wch == ch {
			m.watchersAllCameras = gen.DeleteFromSliceUnordered(m.watchersAllCameras, i)
			return
		}
	}
	m.Log.Warnf("Monitor.RemoveWatcherAllCameras failed to find channel")
}

// Add a new agent that is going to watch for alarm triggers
func (m *Monitor) AddAlarmWatcher() chan *AlarmEvent {
	m.watchersLock.Lock()
	defer m.watchersLock.Unlock()
	ch := make(chan *AlarmEvent, 20)
	m.alarmWatchers = append(m.alarmWatchers, ch)
	return ch
}

// Unregister an alarm watcher
func (m *Monitor) RemoveAlarmWatcher(ch chan *AlarmEvent) {
	m.watchersLock.Lock()
	defer m.watchersLock.Unlock()
	for i, wch := range m.alarmWatchers {
		if wch == ch {
			m.alarmWatchers = gen.DeleteFromSliceUnordered(m.alarmWatchers, i)
			return
		}
	}
	m.Log.Warnf("Monitor.RemoveAlarmWatcher failed to find channel")
}

func (m *Monitor) sendToWatchers(state *AnalysisState) {
	m.watchersLock.RLock()
	// Regarding our behaviour here to drop frames:
	// Perhaps it would be better not to drop frames, but simply to stall.
	// This would presumably wake up the threads that consume the analysis.
	// HOWEVER - if a watcher is waiting on IO, then waking up other threads
	// wouldn't help.
	// ALSO - I think we want the behaviour that even if one watcher stalls, other watchers
	// can continue to run. If we didn't drop frames, then all watchers would stall.
	for _, ch := range m.watchers[state.CameraID] {
		// SYNC-WATCHER-CHANNEL-SIZE
		if len(ch) >= cap(ch)*9/10 {
			// This should never happen. But as a safeguard against monitor stalls, we choose to drop frames.
			m.Log.Warnf("Monitor watcher on camera %v is falling behind. I am going to drop frames.", state.CameraID)
		} else {
			ch <- state
		}
	}
	for _, ch := range m.watchersAllCameras {
		// SYNC-WATCHER-CHANNEL-SIZE
		if len(ch) >= cap(ch)*9/10 {
			// This should never happen. But as a safeguard against a monitor stalls, we choose to drop frames.
			m.Log.Warnf("Monitor watcher on all cameras is falling behind. I am going to drop frames.")
		} else {
			ch <- state
		}
	}
	m.watchersLock.RUnlock()
}

func (m *Monitor) sendToAlarmWatchers(event *AlarmEvent) {
	m.alarmWatchersLock.RLock()
	for _, ch := range m.alarmWatchers {
		ch <- event
	}
	m.alarmWatchersLock.RUnlock()
}
