package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cyclopcam/cyclops/arc/server/auth"
	"github.com/cyclopcam/cyclops/arc/server/storage"
	"github.com/cyclopcam/cyclops/arc/server/storagecache"
	"github.com/cyclopcam/cyclops/arc/server/video"
	"github.com/cyclopcam/logs"
	"github.com/julienschmidt/httprouter"
	"gorm.io/gorm"
)

type Server struct {
	HotReloadWWW bool
	Log          logs.Log
	DB           *gorm.DB

	signalIn     chan os.Signal
	httpServer   *http.Server
	httpRouter   *httprouter.Router
	auth         *auth.AuthServer
	video        *video.VideoServer
	storage      storage.Storage
	storageCache *storagecache.StorageCache
}

func NewServer(configFile string) (*Server, error) {
	cfg := Config{}
	if cfgB, err := os.ReadFile(configFile); err != nil {
		return nil, err
	} else {
		if err := json.Unmarshal(cfgB, &cfg); err != nil {
			return nil, fmt.Errorf("Error parsing config file %v: %w", configFile, err)
		}
	}
	logger, err := logs.NewLog()
	if err != nil {
		return nil, err
	}
	db, err := openDB(logger, cfg.DB)
	if err != nil {
		return nil, err
	}
	authServer := auth.NewAuthServer(db, logger, "arc-session")

	// Open blob store
	var storageServer storage.Storage
	var storageCache *storagecache.StorageCache
	if cfg.VideoStorage.GCS != nil {
		// Google Cloud Storage
		storageServer, err = storage.NewStorageGCS(logger, cfg.VideoStorage.GCS.Bucket, cfg.VideoStorage.GCS.Public)
		if err != nil {
			return nil, err
		}
	} else if cfg.VideoStorage.Filesystem != nil {
		// Filesystem
		storageServer, err = storage.NewStorageFS(logger, cfg.VideoStorage.Filesystem.Root)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("One of the storage options must be configured (i.e. either 'filesystem' or 'gcs')")
	}

	// Our aim is to not need to use the storage cache. But for private blob store buckets, we need to.
	// We could also implement signed URLs, but I haven't bothered with that yet.
	storageCache, err = storagecache.NewStorageCache(logger, storageServer, cfg.VideoCache, 256*1024*1024)
	if err != nil {
		return nil, err
	}

	videoServer := video.NewVideoServer(logger, db, storageServer, storageCache)
	s := &Server{
		Log:          logger,
		DB:           db,
		auth:         authServer,
		video:        videoServer,
		storage:      storageServer,
		storageCache: storageCache,
	}
	if err := s.setupHttpRoutes(); err != nil {
		return nil, err
	}
	return s, nil
}

// port example: ":8081"
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
				s.Shutdown()
			} else {
				// This path gets hit when Shutdown() is called by something other than ourselves, and Shutdown() closes the signalIn channel.
				s.Log.Infof("signalIn closed. ListenForKillSignals will exit now")
			}
		}
	}()
}

func (s *Server) Shutdown() {
	s.Log.Infof("Shutdown")
	signal.Stop(s.signalIn)
	close(s.signalIn)
	s.Log.Infof("Closing HTTP server")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	err := s.httpServer.Shutdown(ctx)
	defer cancel()
	if err != nil {
		s.Log.Warnf("Shutdown complete, with error: %v", err)
	} else {
		s.Log.Infof("Shutdown complete")
	}
	s.Log.Close()
}
