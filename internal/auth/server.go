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
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/markb/supalite/internal/log"
)

// GoTrueVersion is the version of GoTrue to download/use
// Should match Supabase hosted auth version for 100% compatibility
const GoTrueVersion = "v2.186.0"

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

	// Monitor the subprocess and restart if it crashes
	go s.monitorAndRestart()

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

	// Check the running flag
	if !s.running {
		return false
	}

	// Additionally check if the process is still alive
	if s.cmd != nil && s.cmd.Process != nil {
		// Signal 0 checks if process exists without actually sending a signal
		if err := s.cmd.Process.Signal(syscall.Signal(0)); err != nil {
			// Process is dead
			s.running = false
			s.ready = false
			return false
		}
	}

	return s.ready
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
	// Build the target URL (including query parameters)
	target := p.target + r.URL.Path
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}

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

	// Copy response headers, but skip CORS headers since the main server handles those
	// This prevents duplicate CORS headers (e.g., "Access-Control-Allow-Origin: *, *")
	corsHeaders := map[string]bool{
		"Access-Control-Allow-Origin":      true,
		"Access-Control-Allow-Methods":     true,
		"Access-Control-Allow-Headers":     true,
		"Access-Control-Allow-Credentials": true,
		"Access-Control-Expose-Headers":    true,
		"Access-Control-Max-Age":           true,
	}
	for name, values := range resp.Header {
		if !corsHeaders[name] {
			for _, value := range values {
				w.Header().Add(name, value)
			}
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
	if s.config.SiteURL != "" {
		env = append(env, fmt.Sprintf("API_EXTERNAL_URL=%s", s.config.SiteURL))
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

	// Email configuration
	if s.config.Email != nil {
		if s.config.Email.SMTPHost != "" {
			env = append(env, fmt.Sprintf("GOTRUE_SMTP_HOST=%s", s.config.Email.SMTPHost))
		}
		if s.config.Email.SMTPPort > 0 {
			env = append(env, fmt.Sprintf("GOTRUE_SMTP_PORT=%d", s.config.Email.SMTPPort))
		}
		if s.config.Email.SMTPUser != "" {
			env = append(env, fmt.Sprintf("GOTRUE_SMTP_USER=%s", s.config.Email.SMTPUser))
		}
		if s.config.Email.SMTPPass != "" {
			env = append(env, fmt.Sprintf("GOTRUE_SMTP_PASS=%s", s.config.Email.SMTPPass))
		}
		if s.config.Email.AdminEmail != "" {
			env = append(env, fmt.Sprintf("GOTRUE_SMTP_ADMIN_EMAIL=%s", s.config.Email.AdminEmail))
		}
		if s.config.Email.Autoconfirm {
			env = append(env, "GOTRUE_MAILER_AUTOCONFIRM=true")
		}
		if s.config.Email.URLPathsInvite != "" {
			env = append(env, fmt.Sprintf("GOTRUE_MAILER_URLPATHS_INVITE=%s", s.config.Email.URLPathsInvite))
		}
		if s.config.Email.URLPathsConfirmation != "" {
			env = append(env, fmt.Sprintf("GOTRUE_MAILER_URLPATHS_CONFIRMATION=%s", s.config.Email.URLPathsConfirmation))
		}
		if s.config.Email.URLPathsRecovery != "" {
			env = append(env, fmt.Sprintf("GOTRUE_MAILER_URLPATHS_RECOVERY=%s", s.config.Email.URLPathsRecovery))
		}
		if s.config.Email.URLPathsEmailChange != "" {
			env = append(env, fmt.Sprintf("GOTRUE_MAILER_URLPATHS_EMAIL_CHANGE=%s", s.config.Email.URLPathsEmailChange))
		}
	}

	return env
}

// findGoTrueBinary searches for the GoTrue binary in various locations,
// and downloads from GitHub releases if not found locally
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

	// Not found locally, download from GitHub releases
	return downloadGoTrueFromGitHub()
}

// downloadGoTrueFromGitHub downloads the GoTrue binary from GitHub releases
func downloadGoTrueFromGitHub() (string, error) {
	// Version to download - should match Supabase hosted auth
	version := GoTrueVersion

	// Determine the platform-specific binary name
	// GitHub releases use: darwin-arm64, linux-amd64, etc.
	platform := runtime.GOOS + "-" + runtime.GOARCH
	binaryName := "gotrue-" + platform

	// Cache directory for downloaded binaries
	cacheDir := filepath.Join(os.TempDir(), "supalite-gotrue")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create cache directory: %w", err)
	}

	extractPath := filepath.Join(cacheDir, binaryName+"-"+version)

	// Check if already downloaded and valid
	if info, err := os.Stat(extractPath); err == nil && info.Mode().Perm()&0111 != 0 {
		return extractPath, nil
	}

	// Download URL from GitHub releases
	// Assumes releases are structured as: gotrue-darwin-arm64, gotrue-linux-amd64, etc.
	downloadURL := fmt.Sprintf("https://github.com/burggraf/supalite/releases/download/%s/%s", version, binaryName)

	fmt.Printf("[GoTrue] Downloading from %s\n", downloadURL)

	// Download the binary
	resp, err := http.Get(downloadURL)
	if err != nil {
		return "", fmt.Errorf("failed to download GoTrue: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download GoTrue: HTTP %d", resp.StatusCode)
	}

	// Read the response body
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// Write to cache
	if err := os.WriteFile(extractPath, data, 0755); err != nil {
		return "", fmt.Errorf("failed to write GoTrue binary: %w", err)
	}

	fmt.Printf("[GoTrue] Downloaded to %s\n", extractPath)
	return extractPath, nil
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

// monitorAndRestart waits for the GoTrue process to exit and restarts it
func (s *Server) monitorAndRestart() {
	// Wait for the command to exit
	err := s.cmd.Wait()

	s.mu.Lock()
	s.running = false
	s.ready = false
	s.mu.Unlock()

	// Log the exit
	if err != nil {
		log.Warn("GoTrue process exited unexpectedly", "error", err)
	} else {
		log.Info("GoTrue process exited")
	}

	// Attempt to restart after a short delay
	time.Sleep(2 * time.Second)

	log.Info("Attempting to restart GoTrue...")

	// Restart by calling Start again with a new context
	// We need to use the parent context that was passed to the original Start call
	// Since we don't have access to it here, we'll create a new one
	ctx := context.Background()
	if err := s.Start(ctx); err != nil {
		log.Error("Failed to restart GoTrue", "error", err)
		log.Warn("Auth API will not be available until GoTrue is manually restarted")
	}
}
