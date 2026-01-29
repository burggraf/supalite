# Mail Capture Mode Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Capture emails sent by GoTrue to a database table instead of sending them through SMTP, enabling development/testing without external email services.

**Architecture:** When `capture_mode` is enabled, Supalite runs a local SMTP server that accepts emails from GoTrue and stores them in a `captured_emails` table. GoTrue is configured to use this local SMTP server instead of an external one. The SMTP server only starts when capture mode is explicitly enabled.

**Tech Stack:** Go standard library `net/smtp` for SMTP protocol, `emersion/go-smtp` for SMTP server implementation, `pgx` for database storage.

---

## Task 1: Add Configuration Fields

**Files:**
- Modify: `internal/config/config.go:11-22` (EmailConfig struct)
- Modify: `internal/config/config.go:112-148` (applyEnvFallbacks function)

**Step 1: Write the failing test**

Create `internal/config/config_test.go`:

```go
package config

import (
	"encoding/json"
	"os"
	"testing"
)

func TestEmailConfig_CaptureMode(t *testing.T) {
	// Create temp config file with capture mode
	configJSON := `{
		"email": {
			"capture_mode": true,
			"capture_port": 2525
		}
	}`

	if err := os.WriteFile("supalite.json", []byte(configJSON), 0644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove("supalite.json")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Email == nil {
		t.Fatal("Email config should not be nil")
	}
	if !cfg.Email.CaptureMode {
		t.Error("CaptureMode should be true")
	}
	if cfg.Email.CapturePort != 2525 {
		t.Errorf("CapturePort = %d, want 2525", cfg.Email.CapturePort)
	}
}

func TestEmailConfig_CaptureMode_EnvFallback(t *testing.T) {
	os.Setenv("SUPALITE_CAPTURE_MODE", "true")
	os.Setenv("SUPALITE_CAPTURE_PORT", "3025")
	defer os.Unsetenv("SUPALITE_CAPTURE_MODE")
	defer os.Unsetenv("SUPALITE_CAPTURE_PORT")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if !cfg.Email.CaptureMode {
		t.Error("CaptureMode should be true from env var")
	}
	if cfg.Email.CapturePort != 3025 {
		t.Errorf("CapturePort = %d, want 3025", cfg.Email.CapturePort)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/config/... -v -run TestEmailConfig_CaptureMode`
Expected: FAIL - CaptureMode and CapturePort fields don't exist

**Step 3: Write minimal implementation**

In `internal/config/config.go`, add to EmailConfig struct (around line 11):

```go
type EmailConfig struct {
	SMTPHost            string `json:"smtp_host,omitempty"`
	SMTPPort            int    `json:"smtp_port,omitempty"`
	SMTPUser            string `json:"smtp_user,omitempty"`
	SMTPPass            string `json:"smtp_pass,omitempty"`
	SMTPAdminEmail      string `json:"smtp_admin_email,omitempty"`
	MailerAutoconfirm   bool   `json:"mailer_autoconfirm,omitempty"`
	MailerURLPathsInvite     string `json:"mailer_urlpaths_invite,omitempty"`
	MailerURLPathsConfirmation string `json:"mailer_urlpaths_confirmation,omitempty"`
	MailerURLPathsRecovery    string `json:"mailer_urlpaths_recovery,omitempty"`
	MailerURLPathsEmailChange string `json:"mailer_urlpaths_email_change,omitempty"`

	// Capture mode configuration
	CaptureMode bool `json:"capture_mode,omitempty"`
	CapturePort int  `json:"capture_port,omitempty"`
}
```

In `applyEnvFallbacks()` (after line 147), add:

```go
	// Capture mode settings
	if !cfg.Email.CaptureMode {
		cfg.Email.CaptureMode = strings.ToLower(getEnv("SUPALITE_CAPTURE_MODE", "")) == "true"
	}
	if cfg.Email.CapturePort == 0 {
		cfg.Email.CapturePort = getEnvInt("SUPALITE_CAPTURE_PORT", 0)
	}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/config/... -v -run TestEmailConfig_CaptureMode`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "$(cat <<'EOF'
feat(config): add capture_mode and capture_port email settings

Add configuration fields to enable email capture mode for development.
When enabled, emails will be captured to database instead of sent via SMTP.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: Add CLI Flags for Capture Mode

**Files:**
- Modify: `cmd/serve.go:32-43` (flag variables)
- Modify: `cmd/serve.go:149-182` (applyFlagOverrides function)
- Modify: `cmd/serve.go:194-225` (init function)

**Step 1: Add flag variables**

In `cmd/serve.go`, add to the flag variable block (around line 42):

```go
	// Email capture mode flags
	flagCaptureMode bool
	flagCapturePort int
```

**Step 2: Add flag registration**

In `cmd/serve.go` `init()` function (around line 224), add:

```go
	// Email capture mode (for development)
	serveCmd.Flags().BoolVar(&flagCaptureMode, "capture-mode", false, "Enable email capture mode (captures emails to database instead of sending)")
	serveCmd.Flags().IntVar(&flagCapturePort, "capture-port", 0, "Port for mail capture SMTP server (default: 1025)")
```

**Step 3: Add flag overrides**

In `cmd/serve.go` `applyFlagOverrides()` function (around line 182), add:

```go
	// Capture mode overrides
	if flagCaptureMode {
		cfg.Email.CaptureMode = true
	}
	if flagCapturePort != 0 {
		cfg.Email.CapturePort = flagCapturePort
	}
```

**Step 4: Run existing tests to verify no breakage**

Run: `go build ./...`
Expected: Build succeeds

**Step 5: Commit**

```bash
git add cmd/serve.go
git commit -m "$(cat <<'EOF'
feat(cmd): add --capture-mode and --capture-port CLI flags

Enable email capture mode from command line for development testing.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Create captured_emails Database Table

**Files:**
- Modify: `internal/server/server.go:1626-1639` (initSchema function)

**Step 1: Write the failing test**

Add to `e2e/basic_test.go` or create a new test file `internal/server/schema_test.go`:

```go
package server

import (
	"context"
	"testing"
	"time"

	"github.com/markb/supalite/internal/pg"
)

func TestCapturedEmailsTableCreation(t *testing.T) {
	// Start embedded postgres
	db := pg.NewEmbeddedDatabase(pg.Config{
		Port:        15433,
		Username:    "test",
		Password:    "test",
		Database:    "testdb",
		RuntimePath: "/tmp/supalite-test-schema",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := db.Start(ctx); err != nil {
		t.Fatalf("Failed to start database: %v", err)
	}
	defer db.Stop()

	// Create server with capture mode enabled
	srv := &Server{
		config: Config{
			Email: &auth.EmailConfig{CaptureMode: true},
		},
		pgDatabase: db,
	}

	if err := srv.initSchema(ctx); err != nil {
		t.Fatalf("initSchema() failed: %v", err)
	}

	// Verify table exists
	conn, err := db.Connect(ctx)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close(ctx)

	var exists bool
	err = conn.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables
			WHERE table_schema = 'public'
			AND table_name = 'captured_emails'
		)
	`).Scan(&exists)

	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if !exists {
		t.Error("captured_emails table should exist")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/server/... -v -run TestCapturedEmailsTableCreation`
Expected: FAIL - table doesn't exist

**Step 3: Write minimal implementation**

Modify `initSchema()` in `internal/server/server.go` (around line 1626):

```go
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

		-- Captured emails table for development/testing
		CREATE TABLE IF NOT EXISTS public.captured_emails (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			from_addr TEXT NOT NULL,
			to_addr TEXT NOT NULL,
			subject TEXT,
			text_body TEXT,
			html_body TEXT,
			raw_message BYTEA
		);

		CREATE INDEX IF NOT EXISTS captured_emails_created_at_idx
			ON public.captured_emails(created_at DESC);

		CREATE INDEX IF NOT EXISTS captured_emails_to_addr_idx
			ON public.captured_emails(to_addr);
	`)
	return err
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/server/... -v -run TestCapturedEmailsTableCreation`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/server/server.go
git commit -m "$(cat <<'EOF'
feat(schema): add captured_emails table for mail capture mode

Creates table to store captured emails during development.
Includes indexes for efficient querying by timestamp and recipient.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: Create Mail Capture SMTP Server Package

**Files:**
- Create: `internal/mailcapture/config.go`
- Create: `internal/mailcapture/server.go`
- Create: `internal/mailcapture/smtp.go`
- Create: `internal/mailcapture/server_test.go`

**Step 1: Add go-smtp dependency**

Run: `go get github.com/emersion/go-smtp`

**Step 2: Create config.go**

Create `internal/mailcapture/config.go`:

```go
package mailcapture

import "github.com/jackc/pgx/v5"

// Config holds configuration for the mail capture server
type Config struct {
	// Port is the port to listen on for SMTP connections
	Port int

	// Host is the hostname to listen on (default: localhost)
	Host string

	// Database is the PostgreSQL connection for storing emails
	Database interface {
		Connect(ctx context.Context) (*pgx.Conn, error)
	}
}

// DefaultConfig returns configuration with sensible defaults
func DefaultConfig() Config {
	return Config{
		Port: 1025,
		Host: "localhost",
	}
}
```

**Step 3: Create server.go**

Create `internal/mailcapture/server.go`:

```go
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
```

**Step 4: Create smtp.go (SMTP protocol handler)**

Create `internal/mailcapture/smtp.go`:

```go
package mailcapture

import (
	"bytes"
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"strings"
	"time"

	"github.com/emersion/go-smtp"
	"github.com/jackc/pgx/v5"
	"github.com/markb/supalite/internal/log"
)

// Database interface for storing captured emails
type Database interface {
	Connect(ctx context.Context) (*pgx.Conn, error)
}

// smtpBackend implements smtp.Backend
type smtpBackend struct {
	database Database
}

func (b *smtpBackend) NewSession(_ *smtp.Conn) (smtp.Session, error) {
	return &smtpSession{database: b.database}, nil
}

// smtpSession handles a single SMTP session
type smtpSession struct {
	database Database
	from     string
	to       []string
}

func (s *smtpSession) AuthPlain(username, password string) error {
	// Accept any auth for capture mode
	return nil
}

func (s *smtpSession) Mail(from string, opts *smtp.MailOptions) error {
	s.from = from
	return nil
}

func (s *smtpSession) Rcpt(to string, opts *smtp.RcptOptions) error {
	s.to = append(s.to, to)
	return nil
}

func (s *smtpSession) Data(r io.Reader) error {
	// Read the full message
	rawMessage, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	// Parse the message
	msg, err := mail.ReadMessage(bytes.NewReader(rawMessage))
	if err != nil {
		log.Warn("failed to parse email", "error", err)
		// Still store it even if parsing fails
		return s.storeEmail("", "", "", "", rawMessage)
	}

	subject := msg.Header.Get("Subject")

	// Decode subject if MIME encoded
	if decoded, err := decodeRFC2047(subject); err == nil {
		subject = decoded
	}

	// Extract body
	textBody, htmlBody := extractBodies(msg)

	// Store for each recipient
	for _, to := range s.to {
		if err := s.storeEmail(subject, textBody, htmlBody, to, rawMessage); err != nil {
			log.Warn("failed to store email", "error", err, "to", to)
		}
	}

	log.Info("captured email", "from", s.from, "to", s.to, "subject", subject)
	return nil
}

func (s *smtpSession) Reset() {
	s.from = ""
	s.to = nil
}

func (s *smtpSession) Logout() error {
	return nil
}

func (s *smtpSession) storeEmail(subject, textBody, htmlBody, to string, rawMessage []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := s.database.Connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	_, err = conn.Exec(ctx, `
		INSERT INTO public.captured_emails
			(from_addr, to_addr, subject, text_body, html_body, raw_message)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, s.from, to, subject, textBody, htmlBody, rawMessage)

	return err
}

// decodeRFC2047 decodes MIME encoded-word strings
func decodeRFC2047(s string) (string, error) {
	dec := new(mime.WordDecoder)
	return dec.DecodeHeader(s)
}

// extractBodies extracts text and HTML bodies from an email
func extractBodies(msg *mail.Message) (textBody, htmlBody string) {
	contentType := msg.Header.Get("Content-Type")

	if strings.HasPrefix(contentType, "multipart/") {
		// Handle multipart message
		mediaType, params, err := mime.ParseMediaType(contentType)
		if err != nil {
			return readBody(msg.Body), ""
		}

		if strings.HasPrefix(mediaType, "multipart/") {
			boundary := params["boundary"]
			mr := multipart.NewReader(msg.Body, boundary)

			for {
				part, err := mr.NextPart()
				if err == io.EOF {
					break
				}
				if err != nil {
					break
				}

				partContentType := part.Header.Get("Content-Type")
				body, _ := io.ReadAll(part)

				if strings.HasPrefix(partContentType, "text/plain") {
					textBody = string(body)
				} else if strings.HasPrefix(partContentType, "text/html") {
					htmlBody = string(body)
				}
			}
		}
	} else if strings.HasPrefix(contentType, "text/html") {
		htmlBody = readBody(msg.Body)
	} else {
		textBody = readBody(msg.Body)
	}

	return textBody, htmlBody
}

func readBody(r io.Reader) string {
	body, _ := io.ReadAll(r)
	return string(body)
}
```

**Step 5: Write the test**

Create `internal/mailcapture/server_test.go`:

```go
package mailcapture

import (
	"context"
	"net/smtp"
	"testing"
	"time"

	"github.com/markb/supalite/internal/pg"
)

func TestMailCaptureServer_CapturesEmail(t *testing.T) {
	// Start embedded postgres
	db := pg.NewEmbeddedDatabase(pg.Config{
		Port:        15434,
		Username:    "test",
		Password:    "test",
		Database:    "testdb",
		RuntimePath: "/tmp/supalite-test-mailcapture",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := db.Start(ctx); err != nil {
		t.Fatalf("Failed to start database: %v", err)
	}
	defer db.Stop()

	// Create the captured_emails table
	conn, err := db.Connect(ctx)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	_, err = conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS public.captured_emails (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			from_addr TEXT NOT NULL,
			to_addr TEXT NOT NULL,
			subject TEXT,
			text_body TEXT,
			html_body TEXT,
			raw_message BYTEA
		)
	`)
	conn.Close(ctx)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Start mail capture server
	srv := NewServer(Config{
		Port:     2525,
		Host:     "localhost",
		Database: db,
	})

	if err := srv.Start(ctx); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer srv.Stop()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Send a test email via SMTP
	err = smtp.SendMail(
		"localhost:2525",
		nil, // no auth
		"sender@example.com",
		[]string{"recipient@example.com"},
		[]byte("Subject: Test Email\r\n\r\nThis is the body."),
	)
	if err != nil {
		t.Fatalf("Failed to send email: %v", err)
	}

	// Give it time to process
	time.Sleep(100 * time.Millisecond)

	// Verify email was captured
	conn, err = db.Connect(ctx)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close(ctx)

	var count int
	err = conn.QueryRow(ctx, "SELECT COUNT(*) FROM public.captured_emails").Scan(&count)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 captured email, got %d", count)
	}

	// Verify email content
	var fromAddr, toAddr, subject, textBody string
	err = conn.QueryRow(ctx, `
		SELECT from_addr, to_addr, subject, text_body
		FROM public.captured_emails LIMIT 1
	`).Scan(&fromAddr, &toAddr, &subject, &textBody)
	if err != nil {
		t.Fatalf("Failed to read captured email: %v", err)
	}

	if fromAddr != "sender@example.com" {
		t.Errorf("from_addr = %q, want sender@example.com", fromAddr)
	}
	if toAddr != "recipient@example.com" {
		t.Errorf("to_addr = %q, want recipient@example.com", toAddr)
	}
	if subject != "Test Email" {
		t.Errorf("subject = %q, want 'Test Email'", subject)
	}
}
```

**Step 6: Run test**

Run: `go test ./internal/mailcapture/... -v`
Expected: PASS

**Step 7: Commit**

```bash
git add internal/mailcapture/
git commit -m "$(cat <<'EOF'
feat(mailcapture): add SMTP server for capturing emails

Implements a minimal SMTP server using go-smtp that:
- Accepts SMTP connections from GoTrue
- Parses email headers and body (text/html)
- Stores captured emails in PostgreSQL

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: Integrate Mail Capture Server into Server Orchestration

**Files:**
- Modify: `internal/server/server.go:27-36` (Server struct)
- Modify: `internal/server/server.go:175-189` (Start function - GoTrue section)
- Modify: `internal/server/server.go:1641-1674` (waitForShutdown)
- Modify: `internal/auth/config.go:5-22` (add CaptureMode fields)

**Step 1: Update auth.EmailConfig**

In `internal/auth/config.go`, add capture mode fields:

```go
type EmailConfig struct {
	// SMTP configuration
	SMTPHost   string
	SMTPPort   int
	SMTPUser   string
	SMTPPass   string
	AdminEmail string

	// Mailer URL paths (for email templates)
	URLPathsInvite       string
	URLPathsConfirmation string
	URLPathsRecovery     string
	URLPathsEmailChange  string

	// Autoconfirm skips email confirmation when true
	Autoconfirm bool

	// Capture mode configuration
	CaptureMode bool
	CapturePort int
}
```

**Step 2: Update Server struct**

In `internal/server/server.go`, add to Server struct (around line 35):

```go
type Server struct {
	config     Config
	router     *chi.Mux
	httpServer *http.Server

	pgDatabase     *pg.EmbeddedDatabase
	prestServer    *prest.Server
	authServer     *auth.Server
	keyManager     *keys.Manager
	captureServer  *mailcapture.Server  // Add this line
}
```

Add import at top:
```go
import (
	// ... existing imports ...
	"github.com/markb/supalite/internal/mailcapture"
)
```

**Step 3: Start mail capture server conditionally**

In `internal/server/server.go` `Start()` function, add after pREST starts and before GoTrue starts (around line 174):

```go
	// 3.5. Start mail capture server if configured
	if s.config.Email != nil && s.config.Email.CaptureMode {
		capturePort := s.config.Email.CapturePort
		if capturePort == 0 {
			capturePort = 1025
		}

		log.Info("starting mail capture server...")
		s.captureServer = mailcapture.NewServer(mailcapture.Config{
			Port:     capturePort,
			Host:     "localhost",
			Database: s.pgDatabase,
		})

		if err := s.captureServer.Start(ctx); err != nil {
			log.Warn("failed to start mail capture server", "error", err)
		} else {
			log.Info("mail capture server started", "port", capturePort)
		}
	}
```

**Step 4: Override GoTrue SMTP config when capture mode is enabled**

Modify the GoTrue configuration section in `Start()` (around line 182):

```go
	// 4. Start GoTrue auth server
	log.Info("starting GoTrue auth server...")
	authCfg := auth.DefaultConfig()
	authCfg.ConnString = connString + "?search_path=auth"
	authCfg.JWTSecret = jwtSecret
	authCfg.SiteURL = s.config.SiteURL

	// Handle email configuration
	if s.config.Email != nil {
		if s.config.Email.CaptureMode && s.captureServer != nil && s.captureServer.IsRunning() {
			// Override SMTP settings to point to local capture server
			log.Info("configuring GoTrue to use mail capture server")
			authCfg.Email = &auth.EmailConfig{
				SMTPHost:   "localhost",
				SMTPPort:   s.captureServer.Port(),
				SMTPUser:   "capture",  // Any value works
				SMTPPass:   "capture",  // Any value works
				AdminEmail: s.config.Email.AdminEmail,
				// Preserve other settings
				URLPathsInvite:       s.config.Email.URLPathsInvite,
				URLPathsConfirmation: s.config.Email.URLPathsConfirmation,
				URLPathsRecovery:     s.config.Email.URLPathsRecovery,
				URLPathsEmailChange:  s.config.Email.URLPathsEmailChange,
				Autoconfirm:          s.config.Email.Autoconfirm,
			}
		} else {
			authCfg.Email = s.config.Email
		}
	}

	s.authServer = auth.NewServer(authCfg)
```

**Step 5: Stop mail capture server on shutdown**

In `waitForShutdown()` (around line 1661), add after authServer stop:

```go
	if s.captureServer != nil {
		_ = s.captureServer.Stop()
	}
```

**Step 6: Update cmd/serve.go to pass capture config**

In `cmd/serve.go`, update the email config conversion (around line 70):

```go
	if cfg.Email != nil && hasEmailConfig(cfg.Email) {
		emailCfg = &auth.EmailConfig{
			SMTPHost:            cfg.Email.SMTPHost,
			SMTPPort:            cfg.Email.SMTPPort,
			SMTPUser:            cfg.Email.SMTPUser,
			SMTPPass:            cfg.Email.SMTPPass,
			AdminEmail:          cfg.Email.SMTPAdminEmail,
			Autoconfirm:         cfg.Email.MailerAutoconfirm,
			URLPathsInvite:      cfg.Email.MailerURLPathsInvite,
			URLPathsConfirmation: cfg.Email.MailerURLPathsConfirmation,
			URLPathsRecovery:    cfg.Email.MailerURLPathsRecovery,
			URLPathsEmailChange: cfg.Email.MailerURLPathsEmailChange,
			CaptureMode:         cfg.Email.CaptureMode,
			CapturePort:         cfg.Email.CapturePort,
		}
	}
```

Also update `hasEmailConfig()`:

```go
func hasEmailConfig(e *config.EmailConfig) bool {
	return e.SMTPHost != "" || e.SMTPPort != 0 || e.SMTPUser != "" ||
		e.SMTPPass != "" || e.SMTPAdminEmail != "" ||
		e.MailerURLPathsInvite != "" || e.MailerURLPathsConfirmation != "" ||
		e.MailerURLPathsRecovery != "" || e.MailerURLPathsEmailChange != "" ||
		e.MailerAutoconfirm || e.CaptureMode
}
```

**Step 7: Run tests**

Run: `go build ./... && go test ./...`
Expected: PASS

**Step 8: Commit**

```bash
git add internal/server/server.go internal/auth/config.go cmd/serve.go
git commit -m "$(cat <<'EOF'
feat(server): integrate mail capture server into orchestration

- Start mail capture server only when capture_mode is enabled
- Override GoTrue SMTP config to use local capture server
- Graceful shutdown of capture server

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: Update Example Config and Documentation

**Files:**
- Modify: `supalite.example.json`
- Modify: `CLAUDE.md`

**Step 1: Update example config**

Add to `supalite.example.json` email section:

```json
{
  "email": {
    "// capture": "Enable capture mode for development - emails are stored in database instead of sent",
    "capture_mode": false,
    "capture_port": 1025,

    "// smtp": "SMTP configuration (ignored when capture_mode is true)",
    "smtp_host": "smtp.gmail.com",
    "smtp_port": 587,
    ...
  }
}
```

**Step 2: Update CLAUDE.md**

Add a new section after Email Configuration:

```markdown
### Email Capture Mode (Development)

For development and testing, you can capture emails to the database instead of sending them:

| Flag | Env Var | Default | Purpose |
|------|---------|---------|---------|
| `--capture-mode` | `SUPALITE_CAPTURE_MODE` | `false` | Enable email capture mode |
| `--capture-port` | `SUPALITE_CAPTURE_PORT` | `1025` | Port for local SMTP server |

**Usage:**

1. Enable capture mode in `supalite.json`:
   ```json
   {
     "email": {
       "capture_mode": true,
       "capture_port": 1025
     }
   }
   ```

2. Or via command line:
   ```bash
   ./supalite serve --capture-mode --capture-port 1025
   ```

3. Query captured emails via the REST API:
   ```bash
   curl http://localhost:8080/rest/v1/captured_emails?select=*&order=created_at.desc
   ```

**Captured emails table schema:**
- `id` (UUID): Primary key
- `created_at` (timestamp): When the email was captured
- `from_addr` (text): Sender email address
- `to_addr` (text): Recipient email address
- `subject` (text): Email subject
- `text_body` (text): Plain text body
- `html_body` (text): HTML body
- `raw_message` (bytea): Raw SMTP message
```

**Step 3: Commit**

```bash
git add supalite.example.json CLAUDE.md
git commit -m "$(cat <<'EOF'
docs: add mail capture mode documentation

Document the new capture_mode feature for development email testing.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: Add Interactive Config Command for Capture Mode

**Files:**
- Modify: `cmd/email.go` (add capture mode to interactive wizard)

**Step 1: Read existing email.go**

First, understand the current wizard implementation.

**Step 2: Add capture mode prompts**

Add prompts for capture mode in the interactive wizard:

```go
// Add at the beginning of the wizard
fmt.Println("\n=== Email Mode ===")
fmt.Println("Choose email mode:")
fmt.Println("1. SMTP - Send real emails via SMTP server")
fmt.Println("2. Capture - Store emails in database (for development)")

var modeChoice string
fmt.Print("Enter choice (1 or 2) [1]: ")
scanner.Scan()
modeChoice = strings.TrimSpace(scanner.Text())

if modeChoice == "2" {
	currentEmail.CaptureMode = true

	// Prompt for capture port
	fmt.Printf("Mail capture port [%d]: ", defaultPort(currentEmail.CapturePort, 1025))
	scanner.Scan()
	if port := strings.TrimSpace(scanner.Text()); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			currentEmail.CapturePort = p
		}
	}

	// Skip SMTP configuration
	fmt.Println("\nCapture mode enabled. Emails will be stored in the database.")
	fmt.Println("Query captured emails: GET /rest/v1/captured_emails")
} else {
	currentEmail.CaptureMode = false
	// Continue with existing SMTP prompts...
}
```

**Step 3: Update validation warnings**

Add warnings specific to capture mode:

```go
if currentEmail.CaptureMode {
	fmt.Println("  Mode: Capture (development)")
	fmt.Printf("  Capture Port: %d\n", currentEmail.CapturePort)
	fmt.Println("\n  ⚠️  Emails will be stored in database, not sent!")
}
```

**Step 4: Commit**

```bash
git add cmd/email.go
git commit -m "$(cat <<'EOF'
feat(cmd): add capture mode to email config wizard

Add option to configure email capture mode in interactive wizard.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Summary

This plan implements email capture mode with the following components:

1. **Configuration** (`internal/config/config.go`): New `CaptureMode` and `CapturePort` fields
2. **CLI Flags** (`cmd/serve.go`): `--capture-mode` and `--capture-port` flags
3. **Database Schema** (`internal/server/server.go`): `captured_emails` table
4. **Mail Capture Server** (`internal/mailcapture/`): SMTP server using `go-smtp`
5. **Server Integration** (`internal/server/server.go`): Conditional startup and GoTrue configuration override
6. **Documentation**: Updated CLAUDE.md and example config

**Key Design Decisions:**

- The local SMTP server **only starts** when `capture_mode: true` is explicitly set
- GoTrue's SMTP configuration is automatically overridden to point to the local capture server
- Captured emails are stored in `public.captured_emails` and accessible via pREST
- The feature is transparent to GoTrue - no modifications needed to the auth service
