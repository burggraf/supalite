package prest

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/prest/prest/config"
	"github.com/prest/prest/router"
)

type Server struct {
	config     Config
	httpServer *http.Server
	handler    http.Handler
	mu         sync.RWMutex
	running    bool
}

func NewServer(cfg Config) *Server {
	return &Server{
		config: cfg,
	}
}

func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil
	}

	// Configure pREST
	if err := s.configurePrest(); err != nil {
		return fmt.Errorf("failed to configure pREST: %w", err)
	}

	// Get the pREST router
	prestRouter := router.Routes()
	s.handler = prestRouter

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.config.Port),
		Handler:      prestRouter,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// Start server
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("pREST server error: %v\n", err)
		}
	}()

	// Wait for server to be ready
	select {
	case <-time.After(2 * time.Second):
		s.running = true
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Server) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if s.httpServer != nil {
		s.httpServer.Shutdown(ctx)
	}

	s.running = false
}

func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

func (s *Server) Handler() http.Handler {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.handler == nil || !s.running {
		return http.NotFoundHandler()
	}
	return s.handler
}

func (s *Server) configurePrest() error {
	// Configure pREST via package config
	config.PrestConf = &config.Prest{
		HTTPHost:         "127.0.0.1",
		HTTPPort:         s.config.Port,
		PGHost:           "localhost",
		PGPort:           5432, // Will be overridden by conn string
		PGDatabase:       "postgres",
		PGUser:           "postgres",
		PGPass:           "postgres",
		PGURL:            s.config.ConnString,
	}

	return nil
}
