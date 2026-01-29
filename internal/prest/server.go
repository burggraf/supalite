package prest

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/prest/prest/adapters/postgres"
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
	// Parse connection string to extract PostgreSQL connection details
	// Format: postgres://user:pass@localhost:port/database
	parsedURL, err := url.Parse(s.config.ConnString)
	if err != nil {
		return fmt.Errorf("failed to parse connection string: %w", err)
	}

	// Extract user:pass from URL
	var pgUser, pgPass string
	if parsedURL.User != nil {
		pgUser = parsedURL.User.Username()
		pgPass, _ = parsedURL.User.Password()
	}

	// Extract port from URL (default to 5432 if not specified)
	pgPort := 5432
	if parsedURL.Port() != "" {
		pgPort, err = strconv.Atoi(parsedURL.Port())
		if err != nil {
			return fmt.Errorf("invalid port in connection string: %w", err)
		}
	}

	// Extract database name from URL path
	pgDatabase := "postgres"
	if parsedURL.Path != "" && parsedURL.Path != "/" {
		// Remove leading slash from path
		pgDatabase = parsedURL.Path[1:]
	}

	// Extract host from URL
	pgHost := parsedURL.Hostname()
	if pgHost == "" {
		pgHost = "localhost"
	}

	// Configure pREST via package config
	config.PrestConf = &config.Prest{
		HTTPHost:           "127.0.0.1",
		HTTPPort:           s.config.Port,
		HTTPTimeout:        60, // HTTP timeout in seconds (was defaulting to 0!)
		PGHost:             pgHost,
		PGPort:             pgPort,
		PGDatabase:         pgDatabase,
		PGUser:             pgUser,
		PGPass:             pgPass,
		PGSSLMode:          "disable",
		PGMaxIdleConn:      10,
		PGMaxOpenConn:      10,
		PGConnTimeout:      10, // Connection timeout in seconds
		// CORS configuration to prevent AccessControl middleware panic
		CORSAllowOrigin:    []string{"*"},
		CORSAllowHeaders:   []string{"*"},
		CORSAllowMethods:   []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"},
		CORSAllowCredentials: false,
	}

	fmt.Printf("pREST config: host=%s, port=%d, database=%s, user=%s\n",
		pgHost, pgPort, pgDatabase, pgUser)

	// Initialize the postgres adapter (required for pREST to work)
	postgres.Load()

	// Debug: Enable pREST debug logging
	config.PrestConf.Debug = true

	return nil
}
