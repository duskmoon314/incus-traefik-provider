package server

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	incusclient "github.com/lxc/incus/client"
	log "github.com/sirupsen/logrus"
	"github.com/traefik/traefik/v3/pkg/config/dynamic"

	"github.com/duskmoon314/incus-traefik-provider/internal/config"
	"github.com/duskmoon314/incus-traefik-provider/internal/incus"
	"github.com/duskmoon314/incus-traefik-provider/internal/traefik"
)

// Server is the HTTP server that serves Traefik dynamic config.
type Server struct {
	cfg    config.Config
	client incusclient.InstanceServer

	mu         sync.RWMutex
	dynamicCfg *dynamic.Configuration
	lastOK     bool
	lastRefresh time.Time
}

// New creates a new Server.
func New(cfg config.Config, client incusclient.InstanceServer) *Server {
	return &Server{
		cfg:    cfg,
		client: client,
	}
}

// Run starts the background refresh goroutine and the HTTP server.
// It blocks until ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc(s.cfg.Server.Path, s.handleConfig)
	mux.HandleFunc(s.cfg.Server.HealthPath, s.handleHealth)

	srv := &http.Server{
		Addr:    s.cfg.Server.Listen,
		Handler: mux,
	}

	// Initial refresh
	s.refresh()

	// Background refresh goroutine
	go s.refreshLoop(ctx)

	// Graceful shutdown
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.WithError(err).Error("server shutdown error")
		}
	}()

	log.WithField("addr", s.cfg.Server.Listen).Info("server starting")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) refreshLoop(ctx context.Context) {
	ticker := time.NewTicker(s.cfg.Server.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.refresh()
		}
	}
}

func (s *Server) refresh() {
	instances, err := incus.GetInstances(s.client, s.cfg)
	if err != nil {
		log.WithError(err).Error("failed to fetch instances")
		s.mu.Lock()
		s.lastOK = false
		s.mu.Unlock()
		return
	}

	dc := traefik.Build(instances, s.cfg)

	s.mu.Lock()
	s.dynamicCfg = dc
	s.lastOK = true
	s.lastRefresh = time.Now()
	s.mu.Unlock()

	log.WithFields(log.Fields{
		"instances": len(instances),
	}).Info("config refreshed")
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.RLock()
	cfg := s.dynamicCfg
	lastOK := s.lastOK
	s.mu.RUnlock()

	if cfg == nil || !lastOK {
		http.Error(w, `{"error":"config not yet available"}`, http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cfg)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	ok := s.lastOK
	s.mu.RUnlock()

	if ok {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("degraded"))
	}
}
