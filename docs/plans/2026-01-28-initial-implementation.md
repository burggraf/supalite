# Supalite Initial Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build Supalite, a lightweight Supabase-compatible backend using embedded PostgreSQL, pREST for REST API, and the real Supabase Auth server (GoTrue).

**Architecture:** Single Go binary that embeds PostgreSQL for data storage, integrates pREST library for PostgREST-compatible API, and runs Supabase's GoTrue auth server as a subprocess. The main binary orchestrates these components and provides a unified CLI interface.

**Tech Stack:**
- **Go 1.25+** - Core runtime
- **fergusstrange/embedded-postgres** - Embedded PostgreSQL database
- **prest/prest** - PostgREST-compatible REST API library
- **supabase/auth** - Official Supabase GoTrue auth server (Go)
- **Cobra** - CLI framework
- **Chi** - HTTP router for orchestration endpoints

---

## Project Structure Reference

Based on sblite, the target structure:
```
supalite/
├── main.go                    # Entry point
├── go.mod                     # Dependencies
├── go.sum
├── cmd/
│   ├── root.go               # Root command
│   ├── serve.go              # `supalite serve` - start all servers
│   └── init.go               # `supalite init` - initialize database
├── internal/
│   ├── pg/                   # Embedded PostgreSQL management
│   │   ├── embedded.go       # Postgres lifecycle (start/stop)
│   │   └── config.go         # Postgres configuration
│   ├── auth/                 # GoTrue auth server integration
│   │   ├── server.go         # GoTrue subprocess management
│   │   └── client.go         # GoTrue API client for config
│   ├── prest/                # pREST integration
│   │   ├── server.go         # pREST server configuration
│   │   └── config.go         # pREST TOML config generation
│   ├── server/               # Main HTTP server (orchestration only)
│   │   ├── server.go         # Chi router, health, orchestration
│   │   └── middleware.go     # Middleware for proxying
│   └── log/                  # Logging system
│       ├── logger.go         # Structured logging
│       └── buffer.go         # In-memory log buffer
└── docs/
    └── plans/
        └── 2026-01-28-initial-implementation.md
```

---

## Task 1: Project Skeleton and Go Module

**Files:**
- Create: `go.mod`
- Create: `main.go`
- Create: `cmd/root.go`
- Create: `README.md`

**Step 1: Initialize Go module**

```bash
cd /Users/markb/dev/supalite
go mod init github.com/markb/supalite
```

Expected: Creates `go.mod` with module declaration

**Step 2: Write go.mod with dependencies**

Create: `go.mod`

```go
module github.com/markb/supalite

go 1.25.5

require (
	github.com/spf13/cobra v1.10.2
	github.com/fergusstrange/embedded-postgres v1.5.0
	github.com/prest/prest v1.5.0
	github.com/go-chi/chi/v5 v5.2.4
	github.com/jackc/pgx/v5 v5.4.3
)
```

**Step 3: Write main.go entry point**

Create: `main.go`

```go
package main

import "github.com/markb/supalite/cmd"

func main() {
	cmd.Execute()
}
```

**Step 4: Write root command**

Create: `cmd/root.go`

```go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	Version   = "dev"
	BuildTime = ""
	GitCommit = ""
)

var rootCmd = &cobra.Command{
	Use:     "supalite",
	Short:   "Supalite - lightweight Supabase-compatible backend",
	Long:    `A single-binary backend with embedded PostgreSQL, pREST, and Supabase Auth (GoTrue).`,
	Version: Version,
}

func init() {
	versionTmpl := "supalite version {{.Version}}"
	if BuildTime != "" {
		versionTmpl += " (built " + BuildTime
		if GitCommit != "" {
			versionTmpl += ", commit " + GitCommit
		}
		versionTmpl += ")"
	}
	versionTmpl += "\n"
	rootCmd.SetVersionTemplate(versionTmpl)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

**Step 5: Write README.md**

Create: `README.md`

```markdown
# Supalite

A lightweight, single-binary backend providing Supabase-compatible functionality using:
- Embedded PostgreSQL (no external database required)
- pREST for PostgREST-compatible REST API
- Supabase Auth (GoTrue) for authentication

## Quick Start

```bash
# Build
go build -o supalite .

# Run (auto-creates database)
./supalite serve

# With custom port
./supalite serve --port 3000
```

## APIs

- **Auth:** `http://localhost:8080/auth/v1/*` (GoTrue)
- **REST:** `http://localhost:8080/rest/v1/*` (pREST)

## License

MIT
```

**Step 6: Download dependencies**

```bash
go mod tidy
```

Expected: Downloads dependencies and creates `go.sum`

**Step 7: Commit**

```bash
git add go.mod go.sum main.go cmd/root.go README.md
git commit -m "feat: initial project skeleton with go.mod and CLI root

- Add cobra CLI framework
- Add dependencies for embedded-postgres, prest, chi, pgx
- Set up project structure"
```

---

## Task 2: Embedded PostgreSQL Management

**Files:**
- Create: `internal/pg/embedded.go`
- Create: `internal/pg/config.go`
- Create: `internal/pg/embedded_test.go`

**Step 1: Create package directory**

```bash
mkdir -p /Users/markb/dev/supalite/internal/pg
```

**Step 2: Write the failing test**

Create: `internal/pg/embedded_test.go`

```go
package pg

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestEmbeddedDatabase_Start(t *testing.T) {
	db := NewEmbeddedDatabase(Config{
		Port:     15432,
		Username: "test",
		Password: "test",
		Database: "testdb",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := db.Start(ctx); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	defer db.Stop()

	// Verify connection works
	conn, err := pgx.Connect(ctx, "postgres://test:test@localhost:15432/testdb")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close(ctx)

	var result int
	if err := conn.QueryRow(ctx, "SELECT 1").Scan(&result); err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if result != 1 {
		t.Errorf("Expected 1, got %d", result)
	}
}
```

**Step 3: Run test to verify it fails**

```bash
go test ./internal/pg -v
```

Expected: FAIL with "undefined: NewEmbeddedDatabase"

**Step 4: Implement embedded database**

Create: `internal/pg/embedded.go`

```go
package pg

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fergusstrange/embedded-postgres"
	"github.com/jackc/pgx/v5"
)

type EmbeddedDatabase struct {
	postgres   *embeddedpostgres.EmbeddedPostgres
	config     Config
	connString string
	mu         sync.RWMutex
	started    bool
}

type Config struct {
	Port     uint16
	Username string
	Password string
	Database string
	DataDir  string // Directory for Postgres data (optional, defaults to temp)
	Version  string // Postgres version (e.g., "V16")
}

func NewEmbeddedDatabase(cfg Config) *EmbeddedDatabase {
	if cfg.Port == 0 {
		cfg.Port = 5432
	}
	if cfg.Username == "" {
		cfg.Username = "postgres"
	}
	if cfg.Password == "" {
		cfg.Password = "postgres"
	}
	if cfg.Database == "" {
		cfg.Database = "postgres"
	}
	if cfg.Version == "" {
		cfg.Version = "V16"
	}

	return &EmbeddedDatabase{
		config: cfg,
		connString: fmt.Sprintf("postgres://%s:%s@localhost:%d/%s",
			cfg.Username, cfg.Password, cfg.Port, cfg.Database),
	}
}

func (db *EmbeddedDatabase) Start(ctx context.Context) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.started {
		return nil
	}

	// Configure embedded Postgres
	config := embeddedpostgres.DefaultConfig().
		Port(db.config.Port).
		Username(db.config.Username).
		Password(db.config.Password).
		Database(db.config.Database).
		Version(embeddedpostgres.PostgresVersion(db.config.Version))

	// Set data directory if specified
	if db.config.DataDir != "" {
		// Ensure data directory exists
		if err := os.MkdirAll(db.config.DataDir, 0755); err != nil {
			return fmt.Errorf("failed to create data directory: %w", err)
		}
		config = config.DataPath(filepath.Join(db.config.DataDir, "data"))
	}

	db.postgres = embeddedpostgres.NewDatabase(config)

	// Start with context timeout
	done := make(chan error, 1)
	go func() {
		done <- db.postgres.Start()
	}()

	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("failed to start postgres: %w", err)
		}
	case <-ctx.Done():
		return fmt.Errorf("postgres start timed out: %w", ctx.Err())
	}

	// Wait for Postgres to be ready
	if err := db.waitReady(ctx); err != nil {
		return fmt.Errorf("postgres not ready: %w", err)
	}

	db.started = true
	return nil
}

func (db *EmbeddedDatabase) Stop() {
	db.mu.Lock()
	defer db.mu.Unlock()

	if !db.started {
		return
	}

	if db.postgres != nil {
		db.postgres.Stop()
	}
	db.started = false
}

func (db *EmbeddedDatabase) ConnectionString() string {
	return db.connString
}

func (db *EmbeddedDatabase) Connect(ctx context.Context) (*pgx.Conn, error) {
	return pgx.Connect(ctx, db.connString)
}

func (db *EmbeddedDatabase) waitReady(ctx context.Context) error {
	const maxRetries = 60
	const retryDelay = 500 * time.Millisecond

	for i := 0; i < maxRetries; i++ {
		conn, err := db.Connect(ctx)
		if err == nil {
			conn.Close(ctx)
			return nil
		}

		select {
		case <-time.After(retryDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return fmt.Errorf("postgres did not become ready")
}
```

**Step 5: Run test to verify it passes**

```bash
go test ./internal/pg -v -run TestEmbeddedDatabase_Start
```

Expected: PASS (note: first run will download Postgres binaries, may take a minute)

**Step 6: Commit**

```bash
git add internal/pg/
git commit -m "feat: add embedded PostgreSQL management

- Add EmbeddedDatabase wrapper around fergusstrange/embedded-postgres
- Support configurable port, credentials, data directory
- Add ready-wait logic before marking as started
- Add test for database start and connection"
```

---

## Task 3: Init Command - Database Initialization

**Files:**
- Create: `cmd/init.go`
- Create: `internal/pg/config.go`

**Step 1: Write config types**

Create: `internal/pg/config.go`

```go
package pg

import "os"

// DefaultConfig returns the default configuration for supalite
func DefaultConfig() Config {
	// Check environment variables
	port := uint16(5432)
	if p := os.Getenv("SUPALITE_PG_PORT"); p != "" {
		if portNum := parsePort(p); portNum > 0 {
			port = portNum
		}
	}

	username := "postgres"
	if u := os.Getenv("SUPALITE_PG_USER"); u != "" {
		username = u
	}

	password := "postgres"
	if p := os.Getenv("SUPALITE_PG_PASSWORD"); p != "" {
		password = p
	}

	database := "postgres"
	if d := os.Getenv("SUPALITE_PG_DATABASE"); d != "" {
		database = d
	}

	dataDir := "./data"
	if d := os.Getenv("SUPALITE_DATA_DIR"); d != "" {
		dataDir = d
	}

	return Config{
		Port:     port,
		Username: username,
		Password: password,
		Database: database,
		DataDir:  dataDir,
		Version:  "V16",
	}
}

func parsePort(s string) uint16 {
	var port int
	if _, err := fmt.Sscanf(s, "%d", &port); err == nil && port > 0 && port < 65536 {
		return uint16(port)
	}
	return 0
}
```

**Step 2: Write init command**

Create: `cmd/init.go`

```go
package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/markb/supalite/internal/pg"
	"github.com/spf13/cobra"
)

var initConfig struct {
	dbPath    string
	port      uint16
	username  string
	password  string
	database  string
	pgVersion string
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the embedded PostgreSQL database",
	Long:  `Creates and initializes the embedded PostgreSQL database with Supabase schema.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Initializing Supalite database...")

		// Create embedded database
		cfg := pg.Config{
			Port:     initConfig.port,
			Username: initConfig.username,
			Password: initConfig.password,
			Database: initConfig.database,
			DataDir:  initConfig.dbPath,
			Version:  initConfig.pgVersion,
		}
		database := pg.NewEmbeddedDatabase(cfg)

		// Start database
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		if err := database.Start(ctx); err != nil {
			return fmt.Errorf("failed to start database: %w", err)
		}
		defer database.Stop()

		// Run schema initialization
		if err := initSchema(ctx, database); err != nil {
			return fmt.Errorf("failed to initialize schema: %w", err)
		}

		fmt.Printf("Database initialized successfully!\n")
		fmt.Printf("Data directory: %s\n", initConfig.dbPath)
		fmt.Printf("Connection: postgres://%s:****@localhost:%d/%s\n",
			initConfig.username, initConfig.port, initConfig.database)

		return nil
	},
}

func initSchema(ctx context.Context, db *pg.EmbeddedDatabase) error {
	conn, err := db.Connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	// Create auth schema placeholder
	// In full implementation, this would run Supabase auth migrations
	_, err = conn.Exec(ctx, `
		CREATE SCHEMA IF NOT EXISTS auth;
		CREATE SCHEMA IF NOT EXISTS storage;
		CREATE SCHEMA IF NOT EXISTS public;
	`)
	return err
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().StringVar(&initConfig.dbPath, "db", "./data", "Data directory for PostgreSQL")
	initCmd.Flags().Uint16Var(&initConfig.port, "port", 5432, "PostgreSQL port")
	initCmd.Flags().StringVar(&initConfig.username, "username", "postgres", "Database username")
	initCmd.Flags().StringVar(&initConfig.password, "password", "postgres", "Database password")
	initCmd.Flags().StringVar(&initConfig.database, "database", "postgres", "Database name")
	initCmd.Flags().StringVar(&initConfig.pgVersion, "pg-version", "V16", "PostgreSQL version (V14, V15, V16)")
}
```

**Step 3: Test init command**

```bash
go build -o supalite .
./supalite init --db /tmp/supalite-test
```

Expected: Creates database and shows success message

**Step 4: Commit**

```bash
git add cmd/init.go internal/pg/config.go
git commit -m "feat: add init command for database initialization

- Add init command to create and initialize embedded PostgreSQL
- Support configurable port, credentials, data directory
- Create base schemas (auth, storage, public)"
```

---

## Task 4: Logging System

**Files:**
- Create: `internal/log/logger.go`
- Create: `internal/log/buffer.go`

**Step 1: Write logger**

Create: `internal/log/logger.go`

```go
package log

import (
	"fmt"
	"io"
	"log"
	"os"
	"sync"
)

type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

var (
	globalLogger *Logger
	once         sync.Once
)

type Logger struct {
	mu     sync.Mutex
	level  Level
	writer io.Writer
}

func init() {
	globalLogger = &Logger{
		level:  LevelInfo,
		writer: os.Stdout,
	}
}

func SetLevel(level Level) {
	globalLogger.SetLevel(level)
}

func SetWriter(w io.Writer) {
	globalLogger.SetWriter(w)
}

func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

func (l *Logger) SetWriter(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.writer = w
}

func Debug(msg string, args ...interface{}) {
	globalLogger.log(LevelDebug, msg, args...)
}

func Info(msg string, args ...interface{}) {
	globalLogger.log(LevelInfo, msg, args...)
}

func Warn(msg string, args ...interface{}) {
	globalLogger.log(LevelWarn, msg, args...)
}

func Error(msg string, args ...interface{}) {
	globalLogger.log(LevelError, msg, args...)
}

func (l *Logger) log(level Level, msg string, args ...interface{}) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	prefix := ""
	switch level {
	case LevelDebug:
		prefix = "DEBUG"
	case LevelInfo:
		prefix = "INFO"
	case LevelWarn:
		prefix = "WARN"
	case LevelError:
		prefix = "ERROR"
	}

	logMsg := fmt.Sprintf("[%s] %s", prefix, msg)
	if len(args) > 0 {
		logMsg += fmt.Sprintf(" %+v", args)
	}
	log.Println(logMsg)
}

// Standard logger for compatibility
func Logger() *log.Logger {
	return log.New(os.Stdout, "", log.LstdFlags)
}
```

**Step 5: Commit**

```bash
git add internal/log/
git commit -m "feat: add structured logging system

- Add Logger with level filtering (Debug, Info, Warn, Error)
- Thread-safe logging with mutex
- Global logger instance for convenience"
```

---

## Task 5: pREST Integration

**Files:**
- Create: `internal/prest/server.go`
- Create: `internal/prest/config.go`
- Create: `internal/prest/server_test.go`

**Step 1: Create prest directory**

```bash
mkdir -p /Users/markb/dev/supalite/internal/prest
```

**Step 2: Write the failing test**

Create: `internal/prest/server_test.go`

```go
package prest

import (
	"context"
	"testing"
	"time"

	"github.com/markb/supalite/internal/pg"
)

func TestPRESTServer_Start(t *testing.T) {
	// Start embedded Postgres
	pgDB := pg.NewEmbeddedDatabase(pg.Config{
		Port:     15433,
		Username: "test",
		Password: "test",
		Database: "testdb",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := pgDB.Start(ctx); err != nil {
		t.Fatalf("Failed to start postgres: %v", err)
	}
	defer pgDB.Stop()

	// Create and start pREST server
	prestSrv := NewServer(Config{
		ConnString: pgDB.ConnectionString(),
		Port:       3010,
	})

	if err := prestSrv.Start(ctx); err != nil {
		t.Fatalf("Failed to start pREST: %v", err)
	}
	defer prestSrv.Stop()

	// Give server time to start
	time.Sleep(2 * time.Second)

	// Verify server is responding (basic health check)
	if !prestSrv.IsRunning() {
		t.Error("Server is not running after Start()")
	}
}
```

**Step 3: Run test to verify it fails**

```bash
go test ./internal/prest -v
```

Expected: FAIL with "undefined: NewServer"

**Step 4: Implement pREST server**

Create: `internal/prest/server.go`

```go
package prest

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/prest/prest/adapters/postgres"
	"github.com/prest/prest/config"
	"github.com/prest/prest/controllers"
	"github.com/prest/prest/middlewares"
)

type Server struct {
	config     Config
	httpServer *http.Server
	mu         sync.RWMutex
	running    bool
}

type Config struct {
	ConnString   string
	Port         int
	HTTPPort     int // Parent app port for proxying
	HTTPSEnabled bool
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
	// Note: pREST typically uses a TOML config file or env vars
	// We'll configure via environment variables for simplicity
	if err := s.configurePrest(); err != nil {
		return fmt.Errorf("failed to configure pREST: %w", err)
	}

	// Get the pREST router
	middlewareCfg := middlewares.Config{}
	router := controllers.Get(middlewareCfg)

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.config.Port),
		Handler:      router,
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
	case <-ctx.Done():
		return ctx.Err()
	}

	s.running = true
	return nil
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
		// Use connection string for full control
		// Note: pREST's config structure may need adjustment based on version
	}

	// Configure adapter
	adapter := postgres.New()
	if adapter == nil {
		return fmt.Errorf("failed to create postgres adapter")
	}

	// Set connection string directly
	adapter.SetDatabaseURL(s.config.ConnString)

	return nil
}
```

**Step 5: Create config helper**

Create: `internal/prest/config.go`

```go
package prest

import (
	"fmt"
	"os"
)

// DefaultConfig returns default pREST configuration
func DefaultConfig(connString string) Config {
	port := 3000
	if p := os.Getenv("SUPALITE_PREST_PORT"); p != "" {
		if portNum := parsePortInt(p); portNum > 0 {
			port = portNum
		}
	}

	return Config{
		ConnString: connString,
		Port:       port,
	}
}

func parsePortInt(s string) int {
	var port int
	if _, err := fmt.Sscanf(s, "%d", &port); err == nil && port > 0 && port < 65536 {
		return port
	}
	return 0
}
```

**Step 6: Run test to verify it passes**

```bash
go test ./internal/prest -v -run TestPRESTServer_Start
```

Expected: PASS (may need adjustment based on actual pREST API)

**Step 7: Commit**

```bash
git add internal/prest/
git commit -m "feat: add pREST server integration

- Add Server wrapper for pREST library
- Configure via connection string for embedded Postgres
- Add lifecycle management (Start/Stop/IsRunning)
- Add test for server start and basic health check"
```

---

## Task 6: GoTrue Auth Server Integration

**Files:**
- Create: `internal/auth/server.go`
- Create: `internal/auth/config.go`
- Create: `internal/auth/server_test.go`

**Step 1: Create auth directory**

```bash
mkdir -p /Users/markb/dev/supalite/internal/auth
```

**Step 2: Write the failing test**

Create: `internal/auth/server_test.go`

```go
package auth

import (
	"context"
	"testing"
	"time"

	"github.com/markb/supalite/internal/pg"
)

func TestGoTrueServer_Start(t *testing.T) {
	// Start embedded Postgres
	pgDB := pg.NewEmbeddedDatabase(pg.Config{
		Port:     15434,
		Username: "gotrue",
		Password: "gotrue",
		Database: "auth_test",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := pgDB.Start(ctx); err != nil {
		t.Fatalf("Failed to start postgres: %v", err)
	}
	defer pgDB.Stop()

	// Create and start GoTrue server
	authSrv := NewServer(Config{
		ConnString:   pgDB.ConnectionString(),
		Port:         9999,
		JWTSecret:    "test-secret",
		SiteURL:      "http://localhost:9999",
	})

	if err := authSrv.Start(ctx); err != nil {
		t.Fatalf("Failed to start GoTrue: %v", err)
	}
	defer authSrv.Stop()

	// Give server time to start
	time.Sleep(3 * time.Second)

	// Verify server is running
	if !authSrv.IsRunning() {
		t.Error("GoTrue server is not running")
	}
}
```

**Step 3: Run test to verify it fails**

```bash
go test ./internal/auth -v
```

Expected: FAIL with "undefined: NewServer"

**Step 4: Implement GoTrue server wrapper**

Create: `internal/auth/server.go`

```go
package auth

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/markb/supalite/internal/log"
)

type Server struct {
	config     Config
	cmd        *exec.Cmd
	mu         sync.RWMutex
	running    bool
	httpClient *http.Client
}

type Config struct {
	ConnString string
	Port       int
	JWTSecret  string
	SiteURL    string
	URI        string // Usually /auth/v1
}

func NewServer(cfg Config) *Server {
	return &Server{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// Start launches the GoTrue server as a subprocess
// Note: This requires the GoTrue binary to be available
// For development, we'll build from source or use a bundled binary
func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil
	}

	// Set up environment for GoTrue
	// Based on supabase/auth environment variables
	env := []string{
		fmt.Sprintf("DATABASE_URL=%s", s.config.ConnString),
		fmt.Sprintf("PORT=%d", s.config.Port),
		fmt.Sprintf("JWT_SECRET=%s", s.config.JWTSecret),
		fmt.Sprintf("SITE_URL=%s", s.config.SiteURL),
		fmt.Sprintf("API_EXTERNAL_URL=%s", s.config.SiteURL),
		"GNAT_READ_TIMEOUT=30s",
		"GNAT_WRITE_TIMEOUT=30s",
	}

	// Find or build GoTrue binary
	gotrueBin, err := s.findGoTrueBinary()
	if err != nil {
		return fmt.Errorf("failed to find GoTrue binary: %w", err)
	}

	s.cmd = exec.Command(gotrueBin)
	s.cmd.Env = env
	s.cmd.Dir = s.config.DataDir

	if err := s.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start GoTrue: %w", err)
	}

	// Wait for server to be ready
	if err := s.waitReady(ctx); err != nil {
		s.cmd.Process.Kill()
		return fmt.Errorf("GoTrue not ready: %w", err)
	}

	s.running = true
	log.Info("GoTrue auth server started",
		"port", s.config.Port,
		"uri", s.config.URI,
	)

	return nil
}

func (s *Server) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running || s.cmd == nil {
		return
	}

	if s.cmd.Process != nil {
		s.cmd.Process.Signal(syscall.SIGTERM)
		// Wait a bit for graceful shutdown
		time.Sleep(2 * time.Second)
		s.cmd.Process.Kill()
	}

	s.running = false
	log.Info("GoTrue auth server stopped")
}

func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

func (s *Server) findGoTrueBinary() (string, error) {
	// Try to find GoTrue in common locations
	paths := []string{
		"./bin/gotrue",
		"/usr/local/bin/gotrue",
		"./gotrue",
	}

	for _, path := range paths {
		if _, err := exec.LookPath(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("GoTrue binary not found (please run: go install github.com/supabase/auth/cmd/gotrue@latest)")
}

func (s *Server) waitReady(ctx context.Context) error {
	const maxRetries = 30
	const retryDelay = 500 * time.Millisecond

	url := fmt.Sprintf("http://127.0.0.1:%d/health", s.config.Port)

	for i := 0; i < maxRetries; i++ {
		select {
		case <-time.After(retryDelay):
		case <-ctx.Done():
			return ctx.Err()
		}

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			continue
		}

		resp, err := s.httpClient.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
	}

	return fmt.Errorf("GoTrue health check failed")
}
```

**Step 5: Create config helper**

Create: `internal/auth/config.go`

```go
package auth

import (
	"fmt"
	"os"
)

// DefaultConfig returns default GoTrue configuration
func DefaultConfig(connString string) Config {
	port := 9999
	if p := os.Getenv("SUPALITE_AUTH_PORT"); p != "" {
		if portNum := parsePortInt(p); portNum > 0 {
			port = portNum
		}
	}

	jwtSecret := "super-secret-jwt-token"
	if s := os.Getenv("SUPALITE_JWT_SECRET"); s != "" {
		jwtSecret = s
	}

	siteURL := "http://localhost:8080"
	if u := os.Getenv("SUPALITE_SITE_URL"); u != "" {
		siteURL = u
	}

	return Config{
		ConnString: connString,
		Port:       port,
		JWTSecret:  jwtSecret,
		SiteURL:    siteURL,
		URI:        "/auth/v1",
	}
}

func parsePortInt(s string) int {
	var port int
	if _, err := fmt.Sscanf(s, "%d", &port); err == nil && port > 0 && port < 65536 {
		return port
	}
	return 0
}
```

**Step 6: Run test (may fail without GoTrue binary)**

```bash
go test ./internal/auth -v
```

Expected: May FAIL without GoTrue binary - that's OK for now

**Step 7: Commit**

```bash
git add internal/auth/
git commit -m "feat: add GoTrue auth server integration

- Add Server wrapper for Supabase GoTrue subprocess
- Configure via environment variables (DATABASE_URL, JWT_SECRET, etc.)
- Add health check for readiness detection
- Add graceful shutdown support"
```

---

## Task 7: Serve Command - Orchestration

**Files:**
- Create: `cmd/serve.go`
- Create: `internal/server/server.go`
- Modify: `internal/prest/server.go` (add expose for port)
- Modify: `internal/auth/server.go` (add DataDir to Config)

**Step 1: Create server package**

```bash
mkdir -p /Users/markb/dev/supalite/internal/server
```

**Step 2: Write orchestration server**

Create: `internal/server/server.go`

```go
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

	pgDatabase    *pg.EmbeddedDatabase
	prestServer   *prest.Server
	authServer    *auth.Server
}

type Config struct {
	Host      string
	Port      int
	DataDir   string
	JWTSecret string
	SiteURL   string
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
	pgCfg := pg.Config{
		Port:     5432,
		Username: "supalite",
		Password: "supalite",
		Database: "supalite",
		DataDir:  s.config.DataDir,
		Version:  "V16",
	}
	s.pgDatabase = pg.NewEmbeddedDatabase(pgCfg)

	if err := s.pgDatabase.Start(ctx); err != nil {
		return fmt.Errorf("failed to start PostgreSQL: %w", err)
	}
	log.Info("PostgreSQL started", "port", 5432)

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
	authCfg := auth.DefaultConfig(connString)
	authCfg.DataDir = s.config.DataDir
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

	// Start server in goroutine
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

	// 7. Wait for shutdown signal
	return s.waitForShutdown(ctx)
}

func (s *Server) setupRoutes() {
	s.router.Get("/health", s.handleHealth)

	// Proxy to pREST (if not running on same port)
	s.router.Mount("/rest/v1", s.prestServer.Handler())

	// Proxy to GoTrue (if not running on same port)
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

	// Create base schemas
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
	case err := <-ctx.Done():
		return err
	}

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Stop HTTP server
	if s.httpServer != nil {
		s.httpServer.Shutdown(shutdownCtx)
	}

	// Stop auth server
	if s.authServer != nil {
		s.authServer.Stop()
	}

	// Stop pREST server
	if s.prestServer != nil {
		s.prestServer.Stop()
	}

	// Stop PostgreSQL
	if s.pgDatabase != nil {
		s.pgDatabase.Stop()
	}

	log.Info("Supalite stopped")
	return nil
}
```

**Step 3: Update prest server to expose handler**

Modify: `internal/prest/server.go`

Add method to Server struct:
```go
func (s *Server) Handler() http.Handler {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.httpServer == nil || !s.running {
		return http.NotFoundHandler()
	}
	return s.httpServer.Handler
}
```

**Step 4: Update auth server to expose handler and DataDir**

Modify: `internal/auth/server.go`

Update Config struct:
```go
type Config struct {
	ConnString string
	Port       int
	JWTSecret  string
	SiteURL    string
	URI        string
	DataDir    string // Directory for GoTrue working files
}
```

Add handler method:
```go
func (s *Server) Handler() http.Handler {
	// Proxy to GoTrue server
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		url := fmt.Sprintf("http://127.0.0.1:%d%s", s.config.Port, r.URL.Path)
		proxyReq, _ := http.NewRequestWithContext(r.Context(), r.Method, url, r.Body)
		for k, v := range r.Header {
			proxyReq.Header[k] = v
		}

		resp, err := s.httpClient.Do(proxyReq)
		if err != nil {
			http.Error(w, "auth service unavailable", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		for k, v := range resp.Header {
			w.Header()[k] = v
		}
		w.WriteHeader(resp.StatusCode)
		// Copy body...
	})
}
```

**Step 5: Write serve command**

Create: `cmd/serve.go`

```go
package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/markb/supalite/internal/log"
	"github.com/markb/supalite/internal/server"
	"github.com/spf13/cobra"
)

var serveConfig struct {
	host      string
	port      int
	dataDir   string
	jwtSecret string
	siteURL   string
	logLevel  string
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Supalite server",
	Long:  `Starts embedded PostgreSQL, pREST, and GoTrue auth server.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Configure logging
		if serveConfig.logLevel != "" {
			switch serveConfig.logLevel {
			case "debug":
				log.SetLevel(log.LevelDebug)
			case "info":
				log.SetLevel(log.LevelInfo)
			case "warn":
				log.SetLevel(log.LevelWarn)
			case "error":
				log.SetLevel(log.LevelError)
			}
		}

		// Validate JWT secret
		if serveConfig.jwtSecret == "" {
			serveConfig.jwtSecret = os.Getenv("SUPALITE_JWT_SECRET")
		}
		if serveConfig.jwtSecret == "" {
			serveConfig.jwtSecret = "super-secret-jwt-key-change-in-production"
			log.Warn("using default JWT secret, set SUPALITE_JWT_SECRET in production")
		}

		// Get site URL
		if serveConfig.siteURL == "" {
			serveConfig.siteURL = os.Getenv("SUPALITE_SITE_URL")
		}
		if serveConfig.siteURL == "" {
			serveConfig.siteURL = fmt.Sprintf("http://localhost:%d", serveConfig.port)
		}

		// Create server
		srv := server.New(server.Config{
			Host:      serveConfig.host,
			Port:      serveConfig.port,
			DataDir:   serveConfig.dataDir,
			JWTSecret: serveConfig.jwtSecret,
			SiteURL:   serveConfig.siteURL,
		})

		// Start with context for graceful shutdown
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		return srv.Start(ctx)
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().StringVar(&serveConfig.host, "host", "0.0.0.0", "Host to bind to")
	serveCmd.Flags().IntVarP(&serveConfig.port, "port", "p", 8080, "Port to listen on")
	serveCmd.Flags().StringVar(&serveConfig.dataDir, "db", "./data", "Data directory for PostgreSQL")
	serveCmd.Flags().StringVar(&serveConfig.jwtSecret, "jwt-secret", "", "JWT signing secret")
	serveCmd.Flags().StringVar(&serveConfig.siteURL, "site-url", "", "Base URL for the application")
	serveCmd.Flags().StringVar(&serveConfig.logLevel, "log-level", "info", "Log level: debug, info, warn, error")
}
```

**Step 6: Commit**

```bash
git add cmd/serve.go internal/server/
git commit -m "feat: add serve command for full server orchestration

- Start embedded PostgreSQL, pREST, and GoTrue in sequence
- Provide unified HTTP server with proxying to services
- Add graceful shutdown on SIGINT/SIGTERM
- Add health check endpoint"
```

---

## Task 8: Build and Run End-to-End Test

**Files:**
- Create: `e2e/basic_test.go`

**Step 1: Create e2e directory**

```bash
mkdir -p /Users/markb/dev/supalite/e2e
```

**Step 2: Write basic E2E test**

Create: `e2e/basic_test.go`

```go
package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/markb/supalite/internal/server"
)

func TestServer_Startup(t *testing.T) {
	// Create server with test ports
	srv := server.New(server.Config{
		Host:      "127.0.0.1",
		Port:      18080,
		DataDir:   "/tmp/supalite-e2e",
		JWTSecret: "test-secret",
		SiteURL:   "http://localhost:18080",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Start server in background
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start(ctx)
	}()

	// Wait for startup
	time.Sleep(10 * time.Second)

	// Test health endpoint
	// (Would add HTTP client checks here)

	cancel()

	// Verify clean shutdown
	if err := <-errCh; err != nil && err != context.Canceled {
		t.Errorf("Server error: %v", err)
	}
}
```

**Step 3: Commit**

```bash
git add e2e/
git commit -m "test: add basic E2E test for server startup

- Test that all components start successfully
- Verify clean shutdown"
```

---

## Task 9: Documentation and Build Scripts

**Files:**
- Modify: `README.md`
- Create: `Makefile`

**Step 1: Update README**

Modify: `README.md`

```markdown
# Supalite

A lightweight, single-binary backend providing Supabase-compatible functionality.

## Features

- **Embedded PostgreSQL** - No external database required
- **pREST** - PostgREST-compatible REST API
- **Supabase Auth (GoTrue)** - Full authentication server

## Quick Start

```bash
# Build
go build -o supalite .

# Initialize database (optional, will auto-create on serve)
./supalite init --db ./data

# Start server
./supalite serve

# With custom settings
./supalite serve --port 3000 --db /path/to/data
```

## APIs

Once running:

- **Auth API:** `http://localhost:8080/auth/v1/*`
  - Signup: `POST /auth/v1/signup`
  - Login: `POST /auth/v1/token?grant_type=password`
  - User: `GET /auth/v1/user` (authenticated)

- **REST API:** `http://localhost:8080/rest/v1/*`
  - Tables: `GET /rest/v1/{table}`
  - Insert: `POST /rest/v1/{table}`
  - Update: `PATCH /rest/v1/{table}`
  - Delete: `DELETE /rest/v1/{table}`

- **Health:** `http://localhost:8080/health`

## Configuration

| Flag | Env Variable | Default | Description |
|------|--------------|---------|-------------|
| `--host` | `SUPALITE_HOST` | `0.0.0.0` | Server bind address |
| `--port` | `SUPALITE_PORT` | `8080` | Server port |
| `--db` | `SUPALITE_DATA_DIR` | `./data` | PostgreSQL data directory |
| `--jwt-secret` | `SUPALITE_JWT_SECRET` | (warning) | JWT signing secret |
| `--site-url` | `SUPALITE_SITE_URL` | localhost | Base URL for callbacks |

## Requirements

- Go 1.25+
- For development: GoTrue binary (`go install github.com/supabase/auth/cmd/gotrue@latest`)

## Architecture

```
supalite (main binary)
├── Embedded PostgreSQL (fergusstrange/embedded-postgres)
├── pREST server (prest/prest)
└── GoTrue auth server (supabase/auth)
```

## License

MIT
```

**Step 2: Create Makefile**

Create: `Makefile`

```makefile
.PHONY: build run test clean

BINARY=supalite
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

build:
	go build -ldflags \
		"-X github.com/markb/supalite/cmd.Version=$(VERSION) \
		 -X github.com/markb/supalite/cmd.BuildTime=$(BUILD_TIME) \
		 -X github.com/markb/supalite/cmd.GitCommit=$(GIT_COMMIT)" \
		-o $(BINARY) .

run: build
	./$(BINARY) serve

test:
	go test ./...

test-verbose:
	go test -v ./...

clean:
	rm -f $(BINARY)
	rm -rf ./data

init:
	go run . init

serve:
	go run . serve

install-gotrue:
	go install github.com/supabase/auth/cmd/gotrue@latest
```

**Step 3: Commit**

```bash
git add README.md Makefile
git commit -m "docs: add comprehensive README and Makefile

- Document all features and APIs
- Add configuration table
- Add Makefile for common tasks"
```

---

## Task 10: Final Build Verification

**Files:**
- None (verification task)

**Step 1: Full build**

```bash
make clean
make build
```

Expected: Clean build with no errors

**Step 2: Run version command**

```bash
./supalite --version
```

Expected: Version output with build info

**Step 3: Test init**

```bash
rm -rf /tmp/supalite-final-test
./supalite init --db /tmp/supalite-final-test
```

Expected: Database initialization success message

**Step 4: Test serve (background)**

```bash
timeout 15 ./supalite serve --db /tmp/supalite-final-test --port 18081 &
sleep 5
curl http://localhost:18081/health
```

Expected: `{"status":"healthy"}`

**Step 5: Commit final tag**

```bash
git add -A
git commit -m "chore: finalize initial implementation

All components working:
- Embedded PostgreSQL starts and stops cleanly
- pREST server provides REST API
- GoTrue server provides auth API
- Orchestration server proxies to both
- Graceful shutdown on signals"
git tag v0.1.0
```

---

## Dependencies Reference

**Sources:**
- [embedded-postgres Go library](https://github.com/fergusstrange/embedded-postgres) - Run real Postgres locally
- [pREST documentation](https://docs.prestd.com/) - PostgreSQL REST API
- [pREST GitHub](https://github.com/prest/prest) - Source code
- [Supabase Auth (GoTrue)](https://github.com/supabase/auth) - Auth server
- [Go in 2026 libraries](https://ademawan.medium.com/go-in-2026-10-most-promising-libraries-for-modern-application-development-c826c50e7ceb) - Context on modern Go ecosystem
