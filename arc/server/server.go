package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/bmharper/cyclops/arc/server/auth"
	"github.com/bmharper/cyclops/pkg/log"
	"github.com/julienschmidt/httprouter"
	"gorm.io/gorm"
)

type Server struct {
	HotReloadWWW bool
	Log          log.Log
	DB           *gorm.DB

	signalIn   chan os.Signal
	httpServer *http.Server
	httpRouter *httprouter.Router
	auth       *auth.AuthServer
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
	logs, err := log.NewLog()
	if err != nil {
		return nil, err
	}
	db, err := openDB(logs, cfg.DB)
	if err != nil {
		return nil, err
	}
	authServer := auth.NewAuthServer(db, logs, "arc-session")
	s := &Server{
		Log:  logs,
		DB:   db,
		auth: authServer,
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

func (s *Server) ListenForInterruptSignal() {
	s.signalIn = make(chan os.Signal, 1)
	signal.Notify(s.signalIn, os.Interrupt)
	go func() {
		for sig := range s.signalIn {
			s.Log.Infof("Received OS signal %v", sig.String())
			s.Shutdown()
		}
	}()
}

func (s *Server) Shutdown() {
	s.Log.Infof("Shutdown")
	signal.Stop(s.signalIn)
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
