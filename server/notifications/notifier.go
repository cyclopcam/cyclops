package notifications

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/cyclopcam/cyclops/server/configdb"
	"github.com/cyclopcam/cyclops/server/eventdb"
	"github.com/cyclopcam/logs"
)

// Notifier is responsible for sending notifications (eg alarm activations) to accounts.cyclopcam.org.
type Notifier struct {
	ShutdownComplete chan bool // Closed after we shutdown

	log              logs.Log
	configDB         *configdb.ConfigDB
	eventDB          *eventdb.EventDB
	newEvent         chan *eventdb.Event // New events from the eventDB
	mainServerCtx    context.Context
	internalShutdown sync.WaitGroup

	// We want this timeout to be small, so that we don't risk blocking for a long time,
	// waiting for an HTTP connection. We make sure that our messages are small.
	httpTimeout time.Duration

	// Maximum number of events that we'll keep in a queue for transmission.
	maxQueueSize int
}

func NewNotifier(logger logs.Log, configDB *configdb.ConfigDB, eventDB *eventdb.EventDB, mainServerCtx context.Context) (*Notifier, error) {
	logger = logs.NewPrefixLogger(logger, "Notifier:")
	queue, err := eventDB.GetCloudQueue()
	if err != nil {
		return nil, fmt.Errorf("Failed to fetch cloud queue transmission queue: %w", err)
	}
	logger.Infof("Event transmission queue has %v events", len(queue))
	n := &Notifier{
		ShutdownComplete: make(chan bool),
		log:              logger,
		configDB:         configDB,
		eventDB:          eventDB,
		newEvent:         make(chan *eventdb.Event, 50), // we really don't want to block sending to this channel
		mainServerCtx:    mainServerCtx,
		httpTimeout:      15 * time.Second,
		maxQueueSize:     100,
	}
	eventDB.AddListener(n.newEvent)

	go n.cloudPinger()
	go n.cloudTransmit(queue)

	// Wait for the above two goroutines to finish before we return.
	n.internalShutdown.Add(2)
	go func() {
		n.internalShutdown.Wait()
		n.log.Infof("Notifier shutdown complete")
		close(n.ShutdownComplete)
	}()

	return n, nil
}

func (n *Notifier) newAuthorizedRequest(token, method, url string, body io.Reader) (*http.Request, error, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(n.mainServerCtx, n.httpTimeout)
	r, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		cancel()
		return nil, err, cancel
	}
	r.Header.Set("Authorization", "Bearer "+token)
	return r, err, cancel
}

// Returns true if we should attempt to ping the cloud, and send notifications there.
func (n *Notifier) shouldTryConnectToCloud() bool {
	nVerified, _ := n.configDB.NumVerifiedIdentities()
	return nVerified > 0
}
