package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/cyclopcam/cyclops/pkg/kibi"
	"github.com/cyclopcam/cyclops/pkg/log"
	"github.com/cyclopcam/cyclops/pkg/videoformat/fsv"
	"github.com/cyclopcam/cyclops/server/arc"
	"github.com/cyclopcam/cyclops/server/configdb"
	"github.com/cyclopcam/cyclops/server/livecameras"
	"github.com/cyclopcam/cyclops/server/monitor"
	"github.com/cyclopcam/cyclops/server/perfstats"
	"github.com/cyclopcam/cyclops/server/util"
	"github.com/cyclopcam/cyclops/server/videodb"
	"github.com/cyclopcam/cyclops/server/vpn"
	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
)

type Server struct {
	Log              log.Log
	TempFiles        *util.TempFiles
	RingBufferSize   int
	MustRestart      bool           // Value of the 'restart' parameter to Shutdown()
	ShutdownStarted  chan bool      // This channel is closed when shutdown starts. So you can select() on it, to wait for shutdown.
	ShutdownComplete chan error     // Used by main() to report any shutdown errors
	OwnIP            net.IP         // If not nil, overrides the IP address used when scanning the LAN for cameras
	HotReloadWWW     bool           // Don't embed the 'www' directory into our binary, but load it from disk, and assume it's not immutable. This is for dev time on the 'www' source.
	StartupErrors    []StartupError // Critical errors encountered at startup. Note that these are errors that are resolvable by fixing the config through the App UI.

	// Public Subsystems
	LiveCameras *livecameras.LiveCameras

	// Private Subsystems
	signalIn               chan os.Signal
	httpServer             *http.Server
	httpRouter             *httprouter.Router
	configDB               *configdb.ConfigDB
	videoDB                *videodb.VideoDB // Can be nil! If the video path is not accessible, then we can fail to create this.
	wsUpgrader             websocket.Upgrader
	monitor                *monitor.Monitor
	arcCredentialsLock     sync.Mutex
	arcCredentials         *arc.ArcServerCredentials // If Arc server is not configured, then this is nil.
	monitorToVideoDBClosed chan bool                 // If this channel is closed, then monitor to video DB has stopped
}

const (
	ServerFlagHotReloadWWW = 1 // Don't embed the 'www' directory into our binary, but load it from disk, and assume it's not immutable. This is for dev time on the 'www' source.
)

// These are critical errors that prevent the system from functioning.
// The idea is that these errors appear on first run, and then you configure
// the system correctly, and once you've restarted the system and everything
// is good, then these errors drop to zero
type StartupErrorCode string

const (
	// SYNC-STARTUP-ERROR-CODES
	StartupErrorArchivePath StartupErrorCode = "ARCHIVE_PATH" // Could be unconfigured or invalid. The front-end can figure that out by taking the user to the config page.
)

// SYNC-STARTUP-ERROR
type StartupError struct {
	Code    StartupErrorCode `json:"code"`
	Message string           `json:"message"` // Possibly detailed message. We never want to throw an error message away, in case there is only one critical code path that elicits it.
}

// Create a new server, load config, start cameras, and listen on HTTP
func NewServer(logger log.Log, configDBFilename string, serverFlags int, nnModelName string, explicitPrivateKey string) (*Server, error) {
	log, err := log.NewLog()
	if err != nil {
		return nil, err
	}
	s := &Server{
		Log:                    log,
		RingBufferSize:         200 * 1024 * 1024,
		ShutdownComplete:       make(chan error, 1),
		ShutdownStarted:        make(chan bool),
		HotReloadWWW:           (serverFlags & ServerFlagHotReloadWWW) != 0,
		monitorToVideoDBClosed: make(chan bool),
	}
	if cfg, err := configdb.NewConfigDB(s.Log, configDBFilename, explicitPrivateKey); err != nil {
		return nil, err
	} else {
		s.configDB = cfg
	}
	s.Log.Infof("Public key: %v", s.configDB.PublicKey)

	// Since storage location needs to be configured, we can't fail to startup just because we're
	// unable to access our video archive.
	if err := s.StartVideoDB(); err != nil {
		s.StartupErrors = append(s.StartupErrors, StartupError{StartupErrorArchivePath, err.Error()})
		log.Errorf("%v", err)
	}
	var fsvArchive *fsv.Archive
	if s.videoDB != nil {
		fsvArchive = s.videoDB.Archive
	}

	s.ApplyConfig()

	monitor, err := monitor.NewMonitor(s.Log, nnModelName)
	if err != nil {
		return nil, err
	}
	s.monitor = monitor

	if s.videoDB != nil {
		s.attachMonitorToVideoDB()
	} else {
		close(s.monitorToVideoDBClosed)
	}

	s.LiveCameras = livecameras.NewLiveCameras(s.Log, s.configDB, s.ShutdownStarted, s.monitor, fsvArchive, s.RingBufferSize)

	// Cameras start connecting here
	s.LiveCameras.Run()

	if err := s.SetupHTTP(); err != nil {
		return nil, err
	}

	return s, nil
}

// Start a VPN client
func (s *Server) StartVPN(kernelWGSecret string) (*vpn.VPN, error) {
	// Setup VPN and register with proxy
	vpnClient := vpn.NewVPN(s.Log, s.configDB.PrivateKey, s.configDB.PublicKey, kernelWGSecret)

	if err := vpnClient.ConnectKernelWG(); err != nil {
		return nil, fmt.Errorf("Failed to connect to Cyclops KernelWG service: %w", err)
	}

	if err := vpnClient.Start(); err != nil {
		vpnClient.DisconnectKernelWG()
		return nil, fmt.Errorf("Failed to start Wireguard VPN: %w", err)
	}

	return vpnClient, nil
}

// port example: ":8080"
func (s *Server) ListenHTTP(port string) error {
	s.Log.Infof("Listening on %v", port)
	s.httpServer = &http.Server{
		Addr:    port,
		Handler: s.httpRouter,
	}
	return s.httpServer.ListenAndServe()
}

func (s *Server) ListenForKillSignals() {
	s.Log.Infof("ListenForKillSignals starting")
	s.signalIn = make(chan os.Signal, 1)
	signal.Notify(s.signalIn, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case sig, ok := <-s.signalIn:
			if ok {
				s.Log.Infof("Received OS signal '%v'. ListenForKillSignals will exit after shutdown", sig.String())
				s.Shutdown(false)
			} else {
				// This path gets hit when Shutdown() is called by something other than ourselves, and Shutdown() closes the signalIn channel.
				s.Log.Infof("signalIn closed. ListenForKillSignals will exit now")
			}
		}
	}()
}

func (s *Server) Shutdown(restart bool) {
	if restart {
		s.Log.Infof("Shutdown and restart")
	} else {
		s.Log.Infof("Shutdown")
	}

	s.Log.Infof("PerfStats: %v", perfstats.Stats.String())

	s.MustRestart = restart

	close(s.ShutdownStarted)

	// Remove our signal handler (we'll re-enable it again if we restart)
	signal.Stop(s.signalIn)

	// If Shutdown was invoked by something *other* than a signal, then this will get ListenForKillSignals() to exit
	close(s.signalIn)

	s.monitor.Close()

	// Closing cameras should close all WebSockets, by virtue of the Streams closing, which sends
	// a message to the websocket thread.
	// This is relevant because calling Shutdown() on our http server will not do anything to upgraded
	// connections (this is explicit in the http server docs).
	// NOTE: the http server also has RegisterOnShutdown.. which might be useful

	s.Log.Infof("Closing HTTP server")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	err := s.httpServer.Shutdown(ctx)
	defer cancel()

	s.Log.Infof("Waiting for monitor -> videoDB thread to close")
	<-s.monitorToVideoDBClosed

	s.Log.Infof("Waiting for cameras to close")
	<-s.LiveCameras.ShutdownComplete

	s.Log.Infof("Shutting down video archive")
	if s.videoDB != nil {
		s.videoDB.Close()
	}

	if err != nil {
		s.Log.Warnf("Shutdown complete, with error: %v", err)
	} else {
		s.Log.Infof("Shutdown complete")
	}
	s.Log.Close()
	s.ShutdownComplete <- err
}

// This is called whenever system config changes
func (s *Server) ApplyConfig() {
	cfg := s.configDB.GetConfig()

	// Arc server
	arcCredentials := &arc.ArcServerCredentials{}
	arcCredentials.ApiKey = cfg.ArcApiKey
	arcCredentials.ServerUrl = strings.TrimSuffix(cfg.ArcServer, "/")
	s.arcCredentialsLock.Lock()
	s.arcCredentials = arcCredentials
	s.arcCredentialsLock.Unlock()

	if s.videoDB != nil {
		maxStorage, _ := kibi.ParseBytes(cfg.Recording.MaxStorageSize)
		if maxStorage == 0 {
			s.Log.Warnf("Max archive storage size is 0 (unlimited). We will eventually run out of disk space.")
		} else {
			s.Log.Infof("Max archive storage size is %v bytes (%v)", maxStorage, cfg.Recording.MaxStorageSize)
		}
		s.videoDB.SetMaxArchiveSize(maxStorage)
	}

	if s.LiveCameras != nil {
		s.LiveCameras.ConfigurationChanged()
	}
}

func (s *Server) StartVideoDB() error {
	config := s.configDB.GetConfig()
	if config.Recording.Path == "" {
		return errors.New("Video archive path is not configured")
	}
	v, err := videodb.NewVideoDB(s.Log, config.Recording.Path)
	if err != nil {
		return err
	}
	s.videoDB = v
	return nil
}
