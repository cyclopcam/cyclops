package notifications

import (
	"time"
)

// Start a thread that pings the cloud every minute to inform it
// that we're still alive.
func (n *Notifier) cloudPinger() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			// Ping the cloud
			//http.DefaultClient.Get("https://accounts.cyclopcam.org/api/v1/keepalive?key=" + n.configDB.GetAPIKey())
			n.pingCloud()
		case <-n.shutdown:
			close(n.ShutdownComplete)
			return
		}
	}
}

func (n *Notifier) pingCloud() {
	n.RefreshCloudAuthToken()
}
