package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/markb/supalite/internal/auth"
	"github.com/markb/supalite/internal/log"
	"github.com/markb/supalite/internal/pg"
	"github.com/markb/supalite/internal/prest"
)

type Server struct {
	config     Config
	router     *chi.Mux
	httpServer *http.Server

	pgDatabase  *pg.EmbeddedDatabase
	prestServer *prest.Server
	authServer  *auth.Server
}

type Config struct {
	Host        string
	Port        int
	PGPort      uint16
	DataDir     string
	JWTSecret   string
	SiteURL     string
	PGUsername  string
	PGPassword  string
	PGDatabase  string
	RuntimePath string // Optional: unique runtime path for test isolation
}

func New(cfg Config) *Server {
	return &Server{
		config: cfg,
		router: chi.NewRouter(),
	}
}

func (s *Server) Start(ctx context.Context) error {
	log.Info("starting Supalite server...")

	// 1. Start embedded PostgreSQL
	log.Info("starting embedded PostgreSQL...")

	// Set default credentials if not provided
	pgUsername := s.config.PGUsername
	if pgUsername == "" {
		pgUsername = "postgres"
	}
	pgPassword := s.config.PGPassword
	if pgPassword == "" {
		pgPassword = "postgres"
	}
	pgDatabase := s.config.PGDatabase
	if pgDatabase == "" {
		pgDatabase = "postgres"
	}

	pgCfg := pg.Config{
		Port:        s.config.PGPort,
		Username:    pgUsername,
		Password:    pgPassword,
		Database:    pgDatabase,
		DataDir:     s.config.DataDir,
		Version:     "16.9.0",
		RuntimePath: s.config.RuntimePath,
	}
	s.pgDatabase = pg.NewEmbeddedDatabase(pgCfg)

	if err := s.pgDatabase.Start(ctx); err != nil {
		return fmt.Errorf("failed to start PostgreSQL: %w", err)
	}
	log.Info("PostgreSQL started", "port", s.config.PGPort)

	// 2. Initialize database schema
	if err := s.initSchema(ctx); err != nil {
		return fmt.Errorf("failed to initialize schema: %w", err)
	}

	connString := s.pgDatabase.ConnectionString()

	// 3. Start pREST server
	log.Info("starting pREST server...")
	prestCfg := prest.DefaultConfig(connString)
	s.prestServer = prest.NewServer(prestCfg)
	if err := s.prestServer.Start(ctx); err != nil {
		return fmt.Errorf("failed to start pREST: %w", err)
	}
	log.Info("pREST started", "port", prestCfg.Port)

	// 4. Start GoTrue auth server
	log.Info("starting GoTrue auth server...")
	authCfg := auth.DefaultConfig()
	authCfg.ConnString = connString
	authCfg.JWTSecret = s.config.JWTSecret
	authCfg.SiteURL = s.config.SiteURL
	s.authServer = auth.NewServer(authCfg)
	if err := s.authServer.Start(ctx); err != nil {
		log.Warn("failed to start GoTrue", "error", err)
		log.Warn("auth API will not be available")
	} else {
		log.Info("GoTrue started", "port", authCfg.Port)
	}

	// 5. Setup orchestration routes
	s.setupRoutes()

	// 6. Start main HTTP server
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("Supalite listening", "addr", addr)
		log.Info("APIs available:")
		log.Info("  Auth:    http://localhost:8080/auth/v1/*")
		log.Info("  REST:    http://localhost:8080/rest/v1/*")
		log.Info("  Health:  http://localhost:8080/health")
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// 7. Wait for shutdown signal (use background context to avoid timeout)
	return s.waitForShutdown(context.Background())
}

func (s *Server) setupRoutes() {
	s.router.Get("/health", s.handleHealth)

	// Proxy to pREST
	s.router.Mount("/rest/v1", s.prestServer.Handler())

	// Proxy to GoTrue
	s.router.Mount("/auth/v1", s.authServer.Handler())
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"healthy"}`)
}

func (s *Server) initSchema(ctx context.Context) error {
	conn, err := s.pgDatabase.Connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	_, err = conn.Exec(ctx, `
		CREATE SCHEMA IF NOT EXISTS auth;
		CREATE SCHEMA IF NOT EXISTS storage;
		CREATE SCHEMA IF NOT EXISTS public;
	`)
	return err
}

func (s *Server) waitForShutdown(ctx context.Context) error {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Info("received signal, shutting down...", "signal", sig)
	case <-ctx.Done():
		return ctx.Err()
	}

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if s.httpServer != nil {
		s.httpServer.Shutdown(shutdownCtx)
	}

	if s.authServer != nil {
		_ = s.authServer.Stop()
	}

	if s.prestServer != nil {
		s.prestServer.Stop()
	}

	if s.pgDatabase != nil {
		s.pgDatabase.Stop()
	}

	log.Info("Supalite stopped")
	return nil
}
