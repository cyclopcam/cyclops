package notifications

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/cyclopcam/cyclops/pkg/gen"
	"github.com/cyclopcam/cyclops/server/configdb"
	"github.com/cyclopcam/cyclops/server/eventdb"
	"github.com/cyclopcam/xeddsa"
)

// Start a thread that pings the cloud every minute to inform it
// that we're still alive.
func (n *Notifier) cloudPinger() {
	// Before doing anything else, try connect to cloud.
	// This will acquire a new token if needed. We don't want
	// to wait for the first ping before getting this ready,
	// because we need the token to be able to send notifications.
	shouldConnect := n.shouldTryConnectToCloud()
	if shouldConnect {
		n.log.Infof("First ping to cloud")
		n.pingCloud()
	} else {
		n.log.Infof("Not attempting to connect to cloud")
	}

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			prev := shouldConnect
			shouldConnect = n.shouldTryConnectToCloud()
			if shouldConnect != prev {
				n.log.Infof("Connect to cloud status changed to: %v", shouldConnect)
			}
			if shouldConnect {
				if n.pingCloud() {
					// try again immediately (because we acquired a new token)
					n.pingCloud()
				}
			}
		case <-n.mainServerCtx.Done():
			n.internalShutdown.Done()
			return
		}
	}
}

func (n *Notifier) cloudTransmit(initialQueue []*eventdb.Event) {
	minPause := 1
	maxPause := 30
	pause := maxPause
	queue := initialQueue
	for {
		select {
		case ev := <-n.newEvent:
			//n.log.Debugf("Queue event received")
			if len(queue) > n.maxQueueSize {
				// Drop old messages
				n.log.Warnf("Dropping old messages from notifier queue, size: %v", len(queue))
				queue = queue[len(queue)-n.maxQueueSize:]
			}
			queue = append(queue, ev)
			pause = 0
		case <-time.After(time.Second * time.Duration(pause)):
			//n.log.Debugf("Queue transmit wakeup")
			if len(queue) != 0 {
				queue = n.transmitQueue(queue)
			}
			if len(queue) == 0 {
				// Queue was cleared, so we can pause until receiving a new event
				pause = maxPause
			} else {
				// Queue was not cleared, so we start backing off
				pause = gen.Clamp(pause*2, minPause, maxPause)
			}
		case <-n.mainServerCtx.Done():
			n.internalShutdown.Done()
			return
		}
	}
}

// Ensure that we have a valid cloud auth token.
// This doubles as our 'ping' function.
// Returns true if this function should be retried immediately.
func (n *Notifier) pingCloud() bool {
	token := n.configDB.GetAccountsToken()
	if token != "" {
		req, _, cancel := n.newAuthorizedRequest(token, "GET", configdb.AccountsUrl+"/api/box/ping", nil)
		defer cancel()
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			n.log.Errorf("Failed to contact cloud: %v", err)
			return false
		}
		defer resp.Body.Close()
		switch resp.StatusCode {
		case http.StatusOK:
			// All good
			return false
		case http.StatusUnauthorized:
			// Acquire a new token
			token = ""
		default:
			// Something else (server error) - do nothing
			bodyB, _ := io.ReadAll(resp.Body)
			n.log.Errorf("Failed to ping cloud: %v '%v'", resp.Status, string(bodyB))
			return false
		}
	}
	if err := n.getNewCloudAuthToken(); err != nil {
		n.log.Errorf("Failed to get new cloud auth token: %v", err)
		return false
	}
	n.log.Infof("Successfully acquired new cloud auth token")
	return true
}

// Get a new cloud auth token.
func (n *Notifier) getNewCloudAuthToken() error {
	n.log.Infof("Acquiring new cloud auth token")

	// Sign a request to login to the accounts server.
	// We sign using xeddsa, which allows us to sign with the same key
	// that we use for VPN and everything else.
	nonce := n.configDB.GetAccountsNonce()
	message := fmt.Sprintf("box/login/%d", nonce)

	xPriv := [32]byte{}
	copy(xPriv[:], n.configDB.PrivateKey[:])
	signature := xeddsa.Sign(&xPriv, []byte(message))
	sigb64 := base64.URLEncoding.EncodeToString(signature[:])
	pubKeyb64 := base64.URLEncoding.EncodeToString(n.configDB.PublicKey[:])

	fullUrl := fmt.Sprintf(configdb.AccountsUrl+"/api/box/login/%v/%v/%v", pubKeyb64, nonce, sigb64)

	ctx, cancel := context.WithTimeout(n.mainServerCtx, n.httpTimeout)
	defer cancel()
	r, err := http.NewRequestWithContext(ctx, "POST", fullUrl, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Failed to get new cloud auth token: %v (%v)", resp.Status, string(msg))
	}
	tokenB, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Failed to read cloud auth token body: %w", err)
	}
	token := string(tokenB)
	if err := n.configDB.SetAccountsToken(token); err != nil {
		return fmt.Errorf("Failed to save cloud auth token: %w", err)
	}
	n.log.Infof("Acquired new cloud auth token: %v...", token[:5])
	return nil
}

// Returns the list of events that still need to be sent.
func (n *Notifier) transmitQueue(queue []*eventdb.Event) []*eventdb.Event {
	accountsToken := n.configDB.GetAccountsToken()
	if accountsToken == "" {
		return queue
	}
	for i, ev := range queue {
		if !n.transmitEvent(accountsToken, ev) {
			return queue[i:]
		}
	}
	return nil
}

func (n *Notifier) makeEventDetail(ev *eventdb.Event) string {
	switch ev.EventType {
	case eventdb.EventTypeArm, eventdb.EventTypeDisarm:
		action := ""
		switch ev.EventType {
		case eventdb.EventTypeArm:
			action = "Armed"
		case eventdb.EventTypeDisarm:
			action = "Disarmed"
		}

		userID := ev.Detail.Data.Arm.UserID
		if user, err := n.configDB.GetUserFromID(userID); err == nil {
			return fmt.Sprintf("%v by %v", action, user.Name)
		} else {
			return fmt.Sprintf("%v by unknown user", action)
		}
	case eventdb.EventTypeAlarm:
		cameraID := ev.Detail.Data.Alarm.CameraID
		if camera, err := n.configDB.GetCameraFromID(cameraID); err == nil {
			return fmt.Sprintf("Alarm triggered by camera %v", camera.Name)
		} else {
			return "Alarm triggered by unknown camera"
		}
	}
	// We shouldn't get here
	return string(ev.EventType)
}

func (n *Notifier) transmitEvent(accountsToken string, ev *eventdb.Event) bool {
	// SYNC-BOX-NOTIFICATION-JSON
	type boxNotificationJSON struct {
		ID        int64  `json:"id"`
		Time      int64  `json:"time"`      // Unix timestamp in milliseconds
		EventType string `json:"eventType"` // arm, disarm, alarm
		Detail    string `json:"detail"`    // eg "Armed by Megan"
		Priority  string `json:"priority"`  // "high" for alarms, or blank string for everything else
	}

	bn := boxNotificationJSON{
		ID:        ev.ID,
		Time:      ev.Time.Get().UnixMilli(),
		EventType: string(ev.EventType),
		Detail:    n.makeEventDetail(ev),
		Priority:  "",
	}
	if ev.EventType == eventdb.EventTypeAlarm {
		bn.Priority = "high"
	}
	j, _ := json.Marshal(&bn)
	req, err, cancel := n.newAuthorizedRequest(accountsToken, "POST", configdb.AccountsUrl+"/api/box/sendNotification", bytes.NewReader(j))
	if err != nil {
		n.log.Errorf("Failed to create request to send notification: %v", err)
		cancel()
		return false
	}
	defer cancel()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		n.log.Errorf("Failed to send notification: %v", err)
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(resp.Body)
		n.log.Errorf("Failed to send notification: %v (%v)", resp.Status, string(msg))
		return false
	}
	n.eventDB.MarkInCloud([]int64{ev.ID})
	return true
}
