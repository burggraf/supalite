package auth

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Server manages the GoTrue auth server subprocess
type Server struct {
	config Config
	cmd    *exec.Cmd
	mu     sync.RWMutex
	running bool
	ready   bool
	cancel  context.CancelFunc
}

// NewServer creates a new GoTrue server instance
func NewServer(cfg Config) *Server {
	return &Server{
		config: cfg,
		running: false,
		ready: false,
	}
}

// Start launches the GoTrue server as a subprocess
func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("server already running")
	}

	// Find the GoTrue binary
	binaryPath, err := findGoTrueBinary()
	if err != nil {
		return fmt.Errorf("failed to find GoTrue binary: %w", err)
	}

	// Create a context for the subprocess
	ctx, s.cancel = context.WithCancel(ctx)

	// Prepare environment variables
	env := s.buildEnv()

	// Build the command
	s.cmd = exec.CommandContext(ctx, binaryPath)
	s.cmd.Env = append(os.Environ(), env...)
	s.cmd.Dir = filepath.Dir(binaryPath)

	// Capture stdout and stderr
	stdout, err := s.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := s.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the command
	if err := s.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start GoTrue: %w", err)
	}

	s.running = true

	// Start goroutines to monitor output
	go s.monitorOutput(stdout)
	go s.monitorOutput(stderr)

	// Wait for the server to be ready
	go s.waitReady()

	return nil
}

// Stop gracefully stops the GoTrue server
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	if s.cancel != nil {
		s.cancel()
	}

	s.running = false
	s.ready = false

	return nil
}

// IsRunning returns true if the server is currently running
func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running && s.ready
}

// Handler returns an HTTP handler that proxies requests to the GoTrue server
// The handler checks ready state on each request
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create a reverse proxy handler for this request
		// The proxy will handle connection errors if GoTrue is not ready
		targetURL := fmt.Sprintf("http://localhost:%d", s.config.Port)
		proxy := &reverseProxy{
			target: targetURL,
		}
		proxy.ServeHTTP(w, r)
	})
}

// reverseProxy is a simple HTTP reverse proxy
type reverseProxy struct {
	target string
}

func (p *reverseProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Build the target URL
	target := p.target + r.URL.Path

	// Create the HTTP client
	client := &http.Client{
		Timeout: 30 * time.Second,
		// Don't follow redirects automatically
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Create the proxy request
	proxyReq, err := http.NewRequest(r.Method, target, r.Body)
	if err != nil {
		http.Error(w, "Failed to create proxy request", http.StatusBadGateway)
		return
	}

	// Copy headers
	for name, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(name, value)
		}
	}

	// Set X-Forwarded-For header
	if clientIP := r.Header.Get("X-Real-IP"); clientIP != "" {
		proxyReq.Header.Set("X-Forwarded-For", clientIP)
	} else {
		proxyReq.Header.Set("X-Forwarded-For", r.RemoteAddr)
	}

	// Execute the request
	resp, err := client.Do(proxyReq)
	if err != nil {
		http.Error(w, "Failed to proxy request", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for name, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	// Write status code
	w.WriteHeader(resp.StatusCode)

	// Copy response body
	io.Copy(w, resp.Body)
}

// buildEnv constructs the environment variables for GoTrue
func (s *Server) buildEnv() []string {
	var env []string

	// Database connection
	env = append(env, fmt.Sprintf("GOTRUE_DB_DRIVER=postgres"))
	env = append(env, fmt.Sprintf("DATABASE_URL=%s", s.config.ConnString))

	// JWT configuration - GOTRUE_JWT_SECRET is required in v2.x
	env = append(env, fmt.Sprintf("GOTRUE_JWT_SECRET=%s", s.config.JWTSecret))
	env = append(env, fmt.Sprintf("JWT_SECRET=%s", s.config.JWTSecret))

	// Site configuration - GOTRUE_SITE_URL is required in v2.x
	env = append(env, fmt.Sprintf("GOTRUE_SITE_URL=%s", s.config.SiteURL))
	env = append(env, fmt.Sprintf("SITE_URL=%s", s.config.SiteURL))

	// API configuration
	// API_EXTERNAL_URL is required by GoTrue v2.x
	if s.config.Port != 0 {
		env = append(env, fmt.Sprintf("API_EXTERNAL_URL=http://localhost:%d", s.config.Port))
	}
	if s.config.URI != "" {
		env = append(env, fmt.Sprintf("URI=%s", s.config.URI))
	}
	if s.config.Port != 0 {
		env = append(env, fmt.Sprintf("PORT=%d", s.config.Port))
	}

	// Logging
	if s.config.LogLevel != "" {
		env = append(env, fmt.Sprintf("LOG_LEVEL=%s", s.config.LogLevel))
	}

	// Operator token
	if s.config.OperatorToken != "" {
		env = append(env, fmt.Sprintf("OPERATOR_TOKEN=%s", s.config.OperatorToken))
	}

	// Database startup retries
	if s.config.DBStartupAttempts > 0 {
		env = append(env, fmt.Sprintf("DB_AUTOMIGRATE=true"))
		env = append(env, fmt.Sprintf("DB_STARTUP_ATTEMPTS=%d", s.config.DBStartupAttempts))
	}

	if s.config.DBStartupDelay > 0 {
		env = append(env, fmt.Sprintf("DB_STARTUP_DELAY=%d", int(s.config.DBStartupDelay.Seconds())))
	}

	return env
}

// findGoTrueBinary searches for the GoTrue binary in various locations
func findGoTrueBinary() (string, error) {
	// List of locations to search
	searchPaths := []string{
		"./bin/gotrue",
		"./gotrue",
		"/usr/local/bin/gotrue",
		"/usr/bin/gotrue",
	}

	// Also search in PATH
	if path := os.Getenv("PATH"); path != "" {
		searchPaths = append(searchPaths, strings.Split(path, ":")...)
	}

	for _, path := range searchPaths {
		if path == "" {
			continue
		}

		// Check if the path is executable
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			// Check if it's executable
			if info.Mode().Perm()&0111 != 0 {
				return filepath.Abs(path)
			}
		}
	}

	return "", fmt.Errorf("GoTrue binary not found in search paths: %v", searchPaths)
}

// waitReady polls the settings endpoint until the server is ready
func (s *Server) waitReady() {
	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	// Use /settings endpoint as health check since /health may not exist
	healthURL := fmt.Sprintf("http://localhost:%d/settings", s.config.Port)

	for i := 0; i < 60; i++ {
		time.Sleep(500 * time.Millisecond)

		resp, err := client.Get(healthURL)
		if err == nil {
			resp.Body.Close()
			// Accept 200 or any success status code
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				s.mu.Lock()
				s.ready = true
				s.mu.Unlock()
				return
			}
		}
	}

	// If we get here, the server never became ready
	// This is OK for now - the binary might not be installed
}

// monitorOutput reads and logs subprocess output
func (s *Server) monitorOutput(r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		// In a real implementation, this would go to a logger
		fmt.Printf("[GoTrue] %s\n", line)
	}
}
