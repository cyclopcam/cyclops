package notifications

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/cyclopcam/cyclops/server/configdb"
	"github.com/cyclopcam/logs"
)

// Notifier is responsible for sending notifications (eg alarm activations) to accounts.cyclopcam.org.
type Notifier struct {
	ShutdownComplete chan bool // Closed when we have shutdown

	log         logs.Log
	configDB    *configdb.ConfigDB
	accountsUrl string
	httpTimeout time.Duration
	shutdown    chan bool
}

func NewNotifier(logger logs.Log, configDB *configdb.ConfigDB, shutdown chan bool) *Notifier {
	n := &Notifier{
		ShutdownComplete: make(chan bool),
		log:              logger,
		configDB:         configDB,
		accountsUrl:      "https://accounts.cyclopcam.org",
		httpTimeout:      10 * time.Second,
		shutdown:         shutdown,
	}
	go n.cloudPinger()
	return n
}

func (n *Notifier) NewAuthorizedRequest(token, method, url string, body io.Reader) (*http.Request, error, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), n.httpTimeout)
	r, err := http.NewRequestWithContext(ctx, method, url, body)
	if err == nil {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	return r, err, cancel
}

// Ensure that we have a valid cloud auth token
func (n *Notifier) RefreshCloudAuthToken() {
	token := n.configDB.GetAccountsToken()
	if token != "" {
		req, _, cancel := n.NewAuthorizedRequest(token, "GET", n.accountsUrl+"/api/ping", nil)
		defer cancel()
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			n.log.Errorf("Failed to contact cloud: %v", err)
			return
		}
		switch resp.StatusCode {
		case http.StatusOK:
			// All good
			return
		case http.StatusUnauthorized:
			// Acquire a new token
			token = ""
		default:
			// Something else - do nothing
			return
		}
	}
	n.GetNewCloudAuthToken()
}

// Get a new cloud auth token
func (n *Notifier) GetNewCloudAuthToken() {
}
