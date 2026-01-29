package mailcapture

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/emersion/go-smtp"
	"github.com/markb/supalite/internal/log"
)

// Server is a mail capture SMTP server that stores emails to a database
type Server struct {
	config   Config
	smtpSrv  *smtp.Server
	listener net.Listener
	mu       sync.RWMutex
	running  bool
}

// NewServer creates a new mail capture server
func NewServer(cfg Config) *Server {
	if cfg.Port == 0 {
		cfg.Port = 1025
	}
	if cfg.Host == "" {
		cfg.Host = "localhost"
	}
	return &Server{
		config: cfg,
	}
}

// Start begins listening for SMTP connections
func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("server already running")
	}

	// Create SMTP backend
	backend := &smtpBackend{
		database: s.config.Database,
	}

	// Create SMTP server
	s.smtpSrv = smtp.NewServer(backend)
	s.smtpSrv.Addr = fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	s.smtpSrv.Domain = "localhost"
	s.smtpSrv.AllowInsecureAuth = true

	// Start listener
	listener, err := net.Listen("tcp", s.smtpSrv.Addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.smtpSrv.Addr, err)
	}
	s.listener = listener

	// Start serving in goroutine
	go func() {
		if err := s.smtpSrv.Serve(listener); err != nil {
			log.Warn("mail capture server stopped", "error", err)
		}
	}()

	s.running = true
	log.Info("mail capture server started", "addr", s.smtpSrv.Addr)
	return nil
}

// Stop gracefully stops the mail capture server
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	if s.smtpSrv != nil {
		s.smtpSrv.Close()
	}
	if s.listener != nil {
		s.listener.Close()
	}

	s.running = false
	log.Info("mail capture server stopped")
	return nil
}

// IsRunning returns true if the server is running
func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// Port returns the port the server is listening on
func (s *Server) Port() int {
	return s.config.Port
}
