package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/caddyserver/certmagic"
	"github.com/cyclopcam/cyclops/pkg/kibi"
	"github.com/cyclopcam/cyclops/pkg/videoformat/fsv"
	"github.com/cyclopcam/cyclops/pkg/videox"
	"github.com/cyclopcam/cyclops/server/arc"
	"github.com/cyclopcam/cyclops/server/configdb"
	"github.com/cyclopcam/cyclops/server/eventdb"
	"github.com/cyclopcam/cyclops/server/livecameras"
	"github.com/cyclopcam/cyclops/server/monitor"
	"github.com/cyclopcam/cyclops/server/notifications"
	"github.com/cyclopcam/cyclops/server/perfstats"
	"github.com/cyclopcam/cyclops/server/util"
	"github.com/cyclopcam/cyclops/server/videodb"
	"github.com/cyclopcam/cyclops/server/vpn"
	"github.com/cyclopcam/logs"
	"github.com/cyclopcam/safewg/wgroot"
	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type Server struct {
	Log              logs.Log
	TempFiles        *util.TempFiles
	RingBufferSize   int             // Size of the circular buffer for each camera
	MustRestart      bool            // Value of the 'restart' parameter to Shutdown()
	ShutdownStarted  chan bool       // This channel is closed when shutdown starts. So you can select() on it, to wait for shutdown.
	ShutdownComplete chan error      // Used by main() to report any shutdown errors
	ShutdownContext  context.Context // Context that is cancelled when shutdown starts. This is used to cancel long-running operations.
	OwnIP            net.IP          // If not nil, overrides the IP address used when scanning the LAN for cameras
	HotReloadWWW     bool            // Don't embed the 'www' directory into our binary, but load it from disk, and assume it's not immutable. This is for dev time on the 'www' source.
	StartupErrors    []StartupError  // Critical errors encountered at startup. Note that these are errors that are resolvable by fixing the config through the App UI.
	EnableDebugAPI   bool            // If true, then the /api/debug endpoint is enabled. This is used for debugging purposes, and should not be enabled in production.

	// Public Subsystems
	LiveCameras   *livecameras.LiveCameras
	Notifications *notifications.Notifier // Notifier is used to send notifications to the cloud (eg Firebase).

	// Private Subsystems
	signalIn               chan os.Signal
	cancelShutdown         context.CancelFunc // Cancel function for the ShutdownContext
	httpServer             *http.Server
	httpsServer            *http.Server
	httpRouter             *httprouter.Router
	configDB               *configdb.ConfigDB
	videoDB                *videodb.VideoDB // Can be nil! If the video path is not accessible, then we can fail to create this.
	eventDB                *eventdb.EventDB // High level events such as alarm activations, and armed state changes.
	wsUpgrader             websocket.Upgrader
	monitor                *monitor.Monitor
	seekFrameCache         *videox.FrameCache // Speeds up seeking
	arcCredentialsLock     sync.Mutex
	arcCredentials         *arc.ArcServerCredentials // If Arc server is not configured, then this is nil.
	lanIPs                 []net.IP                  // Auto detected LAN IPs of this server. Overridden by OwnIP for camera scanner, if OwnIP is set.
	monitorToVideoDBClosed chan bool                 // If this channel is closed, then monitor to video DB has stopped
	alarmHandlerClosed     chan bool                 // If this channel is closed, then the alarm handler has stopped
}

const (
	ServerFlagHotReloadWWW = 1 // Don't embed the 'www' directory into our binary, but load it from disk, and assume it's not immutable. This is for dev time on the 'www' source.
	ServerFlagDebug        = 2 // Enable debug mode. Opens /api/debug for debugging purposes.
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
func NewServer(logger logs.Log, cfg *configdb.ConfigDB, serverFlags int, nnModelsDir, nnModelName string) (*Server, error) {

	// These are the sizes of two large memory buffers that we allocate.
	ringBufferMB := 200    // This is *per camera*
	seekFrameCacheMB := 50 // This is shared between all cameras

	shutdownContext, cancelShutdown := context.WithCancel(context.Background())

	s := &Server{
		Log:                    logger,
		RingBufferSize:         ringBufferMB * 1024 * 1024,
		ShutdownComplete:       make(chan error, 1),
		ShutdownStarted:        make(chan bool),
		ShutdownContext:        shutdownContext,
		cancelShutdown:         cancelShutdown,
		HotReloadWWW:           (serverFlags & ServerFlagHotReloadWWW) != 0,
		EnableDebugAPI:         (serverFlags & ServerFlagDebug) != 0,
		monitorToVideoDBClosed: make(chan bool),
		alarmHandlerClosed:     make(chan bool),
		configDB:               cfg,
		seekFrameCache:         videox.NewFrameCache(seekFrameCacheMB * 1024 * 1024),
	}

	s.loadLANIPs()

	// Open the events database (high level stuff, like arm/disarm)
	eventDBFilename := filepath.Join(filepath.Dir(cfg.DBFilename()), "events.sqlite")
	eventDB, err := eventdb.NewEventDB(logger, eventDBFilename)
	if err != nil {
		return nil, err
	}
	s.eventDB = eventDB

	// Since storage location needs to be configured, we can't fail to startup just because we're
	// unable to access our video archive.
	if err := s.StartVideoDB(); err != nil {
		s.StartupErrors = append(s.StartupErrors, StartupError{StartupErrorArchivePath, err.Error()})
		logger.Errorf("%v", err)
	}
	var fsvArchive *fsv.Archive
	if s.videoDB != nil {
		fsvArchive = s.videoDB.Archive
	}

	s.ApplyConfig()

	// Start notification system, which sends realtime events to the cloud/LAN
	notifier, err := notifications.NewNotifier(s.Log, s.configDB, s.eventDB, s.ShutdownContext)
	if err != nil {
		return nil, err
	}
	s.Notifications = notifier

	monitorOptions := monitor.DefaultMonitorOptions()
	if nnModelsDir != "" {
		monitorOptions.ModelsDir = nnModelsDir
	}
	if nnModelName != "" {
		monitorOptions.ModelNameLQ = nnModelName
		// Set HQ model automatically
		if strings.HasPrefix(nnModelName, "yolov8") {
			plus1 := map[rune]rune{'n': 's', 's': 'm', 'm': 'l', 'l': 'x'}
			if hq, ok := plus1[rune(nnModelName[len(nnModelName)-1])]; ok {
				monitorOptions.ModelNameHQ = nnModelName[:len(nnModelName)-1] + string(hq)
			}
		}
	}
	monitor, err := monitor.NewMonitor(s.Log, monitorOptions)
	if err != nil {
		return nil, err
	}
	s.monitor = monitor

	if s.videoDB != nil {
		s.attachMonitorToVideoDB()
	} else {
		close(s.monitorToVideoDBClosed)
	}

	s.runAlarmHandler()

	s.LiveCameras = livecameras.NewLiveCameras(s.Log, s.configDB, s.ShutdownStarted, s.monitor, fsvArchive, s.RingBufferSize)

	// Cameras start connecting here
	s.LiveCameras.Run()

	if err := s.SetupHTTP(); err != nil {
		return nil, err
	}

	return s, nil
}

// Start a VPN client
func StartVPN(log logs.Log, privateKey wgtypes.Key, kernelWGSecret string, forceIPv4 bool) (*vpn.VPN, error) {
	// Setup VPN and register with proxy
	vpnClient := vpn.NewVPN(log, privateKey, kernelWGSecret, forceIPv4)

	if err := vpnClient.ConnectKernelWG(); err != nil {
		return nil, fmt.Errorf("Failed to connect to Cyclops KernelWG service: %w", err)
	}

	if err := vpnClient.Start(); err != nil {
		vpnClient.DisconnectKernelWG()
		return nil, fmt.Errorf("Failed to start Wireguard VPN: %w", err)
	}

	return vpnClient, nil
}

func (s *Server) ListenHTTP(port int, privilegeLimiter *wgroot.PrivilegeLimiter) error {
	portStr := fmt.Sprintf(":%v", port)
	s.Log.Infof("Listening on %v (automatic SSL disabled)", port)
	s.httpServer = &http.Server{
		Addr:    portStr,
		Handler: s.httpRouter,
	}

	privilegedPort := port == 80

	if privilegeLimiter != nil && privilegedPort {
		if err := privilegeLimiter.Elevate(); err != nil {
			return err
		}
	}

	listener, err := net.Listen("tcp", portStr)
	if err != nil {
		return err
	}

	if privilegeLimiter != nil && privilegedPort {
		if err := privilegeLimiter.Drop(); err != nil {
			s.Log.Errorf("Error dropping privileges: %v", err)
		}
	}

	return s.httpServer.Serve(listener)
}

func (s *Server) ListenHTTPS(sslCertDirectory string, privilegeLimiter *wgroot.PrivilegeLimiter) error {
	s.Log.Infof("Enabling automatic SSL")
	sslHostname := vpn.ProxiedHostName(s.configDB.PublicKey)
	return s.listenHTTPS(sslCertDirectory, []string{sslHostname}, privilegeLimiter, s.httpRouter)
}

// Copied from certmagic.dataDir(), but no support for Windows
//func certmagicDataDir() string {
//	baseDir := filepath.Join(os.Getenv("HOME"), ".local", "share")
//	if xdgData := os.Getenv("XDG_DATA_HOME"); xdgData != "" {
//		baseDir = xdgData
//	}
//	return filepath.Join(baseDir, "certmagic")
//}

// Copied and modified from certmagic.HTTPS()
func (s *Server) listenHTTPS(sslCertDirectory string, domainNames []string, privilegeLimiter *wgroot.PrivilegeLimiter, mux http.Handler) error {
	certmagic.DefaultACME.Agreed = true                           // read and agree to your CA's legal documents
	certmagic.DefaultACME.Email = "rogojin+cyclopscert@gmail.com" // email address
	//certmagic.DefaultACME.CA = certmagic.LetsEncryptStagingCA    // use the staging endpoint while we're developing
	certmagic.DefaultACME.CA = certmagic.LetsEncryptProductionCA

	ctx := context.Background()
	cfg := certmagic.NewDefault()

	// We need to create a new FileStorage objects, because the default certmagic FileStorage
	// is created at process startup, but at that time we are root, and we only setuid later.
	cfg.Storage = &certmagic.FileStorage{Path: sslCertDirectory}

	if privilegeLimiter != nil {
		if err := privilegeLimiter.Elevate(); err != nil {
			return err
		}
	}
	defer func() {
		if privilegeLimiter != nil {
			if err := privilegeLimiter.Drop(); err != nil {
				s.Log.Errorf("Error dropping privileges: %v", err)
			}
		}
	}()

	err := cfg.ManageSync(ctx, domainNames) // should probably use ManageAsync
	if err != nil {
		return err
	}

	tlsConfig := cfg.TLSConfig()
	tlsConfig.NextProtos = append([]string{"h2", "http/1.1"}, tlsConfig.NextProtos...)

	httpLn, err80 := net.Listen("tcp", ":80")
	httpsLn, err443 := tls.Listen("tcp", ":443", tlsConfig)

	if privilegeLimiter != nil {
		if err := privilegeLimiter.Drop(); err != nil {
			s.Log.Errorf("Error dropping privileges: %v", err)
		}
		privilegeLimiter = nil // cancel the defer'ed drop
	}

	if err80 != nil || err443 != nil {
		if httpLn != nil {
			httpLn.Close()
		}
		if httpsLn != nil {
			httpsLn.Close()
		}
		if err80 != nil {
			return err80
		}
		return err443
	}

	httpServer := &http.Server{
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
		Handler:           mux,
		BaseContext:       func(listener net.Listener) context.Context { return ctx },
	}
	if len(cfg.Issuers) > 0 {
		if am, ok := cfg.Issuers[0].(*certmagic.ACMEIssuer); ok {
			// We don't want auto redirect, because this must work on a LAN on port 80, where all you have is an IP.
			// Perhaps some day we can figure out a way to do a lan-local DNS address, but that would probably require
			// using DNS-01 auth for LetsEncrypt, and a bunch of extra work to make that all secure.
			//httpServer.Handler = am.HTTPChallengeHandler(http.HandlerFunc(httpRedirectHandler))
			httpServer.Handler = am.HTTPChallengeHandler(mux)
		}
	}
	httpsServer := &http.Server{
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      2 * time.Minute,
		IdleTimeout:       5 * time.Minute,
		Handler:           mux,
		BaseContext:       func(listener net.Listener) context.Context { return ctx },
	}

	s.Log.Infof("%v Serving HTTP/HTTPS on %v and %v", domainNames, httpLn.Addr(), httpsLn.Addr())

	s.httpServer = httpServer
	s.httpsServer = httpsServer
	go func() {
		if err := httpServer.Serve(httpLn); err != nil {
			s.Log.Infof("httpServer.Serve() returned: %v", err)
		}
	}()
	return httpsServer.Serve(httpsLn)
}

func (s *Server) ListenForKillSignals() {
	s.Log.Infof("ListenForKillSignals starting")
	s.signalIn = make(chan os.Signal, 1)
	signal.Notify(s.signalIn, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig, ok := <-s.signalIn
		if ok {
			s.Log.Infof("Received OS signal '%v'. ListenForKillSignals will exit after shutdown", sig.String())
			s.Shutdown(false)
		} else {
			// This path gets hit when Shutdown() is called by something other than ourselves, and Shutdown() closes the signalIn channel.
			s.Log.Infof("signalIn closed. ListenForKillSignals will exit now")
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

	// If any functions are using ShutdownContext, then they'll get cancelled now
	s.cancelShutdown()

	close(s.ShutdownStarted)

	//s.Log.Infof("SHUTDOWN 1")

	// Remove our signal handler (we'll re-enable it again if we restart)
	signal.Stop(s.signalIn)

	//s.Log.Infof("SHUTDOWN 2")

	// If Shutdown was invoked by something *other* than a signal, then this will get ListenForKillSignals() to exit
	close(s.signalIn)

	//s.Log.Infof("SHUTDOWN 3")

	s.monitor.Close()

	//s.Log.Infof("SHUTDOWN 4")

	// Closing cameras should close all WebSockets, by virtue of the Streams closing, which sends
	// a message to the websocket thread.
	// This is relevant because calling Shutdown() on our http server will not do anything to upgraded
	// connections such as WebSockets (this is explicit in the http server docs).
	// NOTE: the http server also has RegisterOnShutdown.. which might be useful
	errors := []error{}

	if s.httpsServer != nil {
		s.Log.Infof("Closing HTTPS server")
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		errors = append(errors, s.httpsServer.Shutdown(ctx))
		defer cancel()
	}

	if s.httpServer != nil {
		s.Log.Infof("Closing HTTP server")
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		errors = append(errors, s.httpServer.Shutdown(ctx))
		defer cancel()
	}

	s.Log.Infof("Waiting for monitor -> videoDB thread to close")
	<-s.monitorToVideoDBClosed

	s.Log.Infof("Waiting for cameras to close")
	<-s.LiveCameras.ShutdownComplete

	s.Log.Infof("Shutting down video archive")
	if s.videoDB != nil {
		s.videoDB.Close()
	}

	s.Log.Infof("Waiting for Notifications system to close")
	<-s.Notifications.ShutdownComplete

	var firstError error
	for _, err := range errors {
		if err != nil {
			firstError = err
			break
		}
	}

	if firstError != nil {
		s.Log.Warnf("Shutdown complete, with error: %v", firstError)
	} else {
		s.Log.Infof("Shutdown complete")
	}
	s.Log.Close()
	s.ShutdownComplete <- firstError
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

func (s *Server) loadLANIPs() {
	// Query the OS for our LAN IP addresses.
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		s.Log.Errorf("Failed to get LAN IP addresses: %v", err)
		return
	}
	s.lanIPs = make([]net.IP, 0, len(addrs))
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			s.lanIPs = append(s.lanIPs, ipnet.IP)
			s.Log.Infof("Found LAN IP address: %v", ipnet.IP)
		}
	}
}
