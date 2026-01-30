# Admin Dashboard Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Create an embedded admin dashboard for Supalite at `/_/` with React+Vite frontend, admin authentication system, and core Supabase-like features.

**Architecture:** Vite+React+ShadCN UI frontend built to static files, embedded in Go binary via `//go:embed`, served from `/_/` path. Admin users stored in `admin.users` table with bcrypt password hashing. JWT tokens for session management.

**Tech Stack:** React 18, TypeScript, Vite 5, Tailwind CSS, ShadCN UI, Go 1.21+, Chi router, bcrypt, pgx

---

## Task 1: Create Admin Package Foundation

**Files:**
- Create: `internal/admin/password.go`
- Create: `internal/admin/user.go`
- Modify: `go.mod` (add bcrypt dependency)

**Step 1: Add bcrypt dependency**

Run:
```bash
go get golang.org/x/crypto/bcrypt
```

Expected: Dependency added to `go.mod` and `go.sum`

**Step 2: Write password hashing utilities**

Create `internal/admin/password.go`:
```go
package admin

import "golang.org/x/crypto/bcrypt"

// HashPassword creates a bcrypt hash from a plain-text password
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// VerifyPassword checks if a plain-text password matches a bcrypt hash
func VerifyPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
```

**Step 3: Write user CRUD operations**

Create `internal/admin/user.go`:
```go
package admin

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// User represents an admin user
type User struct {
	ID           uuid.UUID `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Create inserts a new admin user into the database
func Create(ctx context.Context, conn *pgx.Conn, email, password string) (*User, error) {
	hash, err := HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user := &User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: hash,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	query := `
		INSERT INTO admin.users (id, email, password_hash, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, email, password_hash, created_at, updated_at
	`

	row := conn.QueryRow(ctx, query, user.ID, user.Email, user.PasswordHash, user.CreatedAt, user.UpdatedAt)
	err = row.Scan(&user.ID, &user.Email, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

// FindByEmail looks up a user by email address
func FindByEmail(ctx context.Context, conn *pgx.Conn, email string) (*User, error) {
	query := `
		SELECT id, email, password_hash, created_at, updated_at
		FROM admin.users
		WHERE email = $1
	`

	var user User
	err := conn.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &user, nil
}

// List returns all admin users
func List(ctx context.Context, conn *pgx.Conn) ([]User, error) {
	query := `
		SELECT id, email, created_at, updated_at
		FROM admin.users
		ORDER BY created_at DESC
	`

	rows, _ := conn.Query(ctx, query)
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		err := rows.Scan(&user.ID, &user.Email, &user.CreatedAt, &user.UpdatedAt)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	return users, rows.Err()
}

// Delete removes a user by email
func Delete(ctx context.Context, conn *pgx.Conn, email string) error {
	query := `DELETE FROM admin.users WHERE email = $1`
	_, err := conn.Exec(ctx, query, email)
	return err
}

// UpdatePassword changes a user's password
func UpdatePassword(ctx context.Context, conn *pgx.Conn, email, newPassword string) error {
	hash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}

	query := `
		UPDATE admin.users
		SET password_hash = $1, updated_at = $2
		WHERE email = $3
	`
	_, err = conn.Exec(ctx, query, hash, time.Now(), email)
	return err
}

// Count returns the total number of admin users
func Count(ctx context.Context, conn *pgx.Conn) (int, error) {
	var count int
	err := conn.QueryRow(ctx, "SELECT COUNT(*) FROM admin.users").Scan(&count)
	return count, err
}
```

**Step 4: Commit**

```bash
git add internal/admin/ go.mod go.sum
git commit -m "feat(admin): add admin user package with password hashing and CRUD

- Add bcrypt-based password hashing utilities
- Add User struct with Create, FindByEmail, List, Delete, UpdatePassword, Count
- Uses admin.users table schema

Co-Authored-By: Claude (GLM-4.7) <noreply@anthropic.com>"
```

---

## Task 2: Update Database Schema

**Files:**
- Modify: `internal/server/server.go` (initSchema function)

**Step 1: Update initSchema to create admin schema and users table**

Modify `internal/server/server.go`, find the `initSchema` function (around line 1677) and add the admin schema:

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
		CREATE SCHEMA IF NOT EXISTS admin;

		-- Admin users table for dashboard authentication
		CREATE TABLE IF NOT EXISTS admin.users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			email TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS admin_users_email_idx
			ON admin.users(email);

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

		-- Enable Row Level Security (mail capture server connects as superuser, bypasses RLS)
		ALTER TABLE public.captured_emails ENABLE ROW LEVEL SECURITY;
	`)
	return err
}
```

**Step 2: Commit**

```bash
git add internal/server/server.go
git commit -m "feat(admin): create admin schema and users table in database init

- Add admin.users table for dashboard authentication
- Table stores email, password_hash (bcrypt), created_at, updated_at
- Add index on email for lookups

Co-Authored-By: Claude (GLM-4.7) <noreply@anthropic.com>"
```

---

## Task 3: Add CLI Commands for Admin Management

**Files:**
- Create: `cmd/admin.go`
- Modify: `cmd/root.go` (to wire in admin command)
- Create: `internal/prompt/prompt.go` (for password input)

**Step 1: Create password prompt utility**

Create `internal/prompt/prompt.go`:
```go
package prompt

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"golang.org/x/term"
)

// Email prompts user for an email address
func Email(prompt string) (string, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(prompt)
	email, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(email), nil
}

// Password prompts user for a password (hidden input)
func Password(prompt string) (string, error) {
	fmt.Print(prompt)
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return "", err
	}
	return string(bytePassword), nil
}

// ConfirmPassword prompts twice and verifies they match
func ConfirmPassword() (string, error) {
	for {
		password, err := Password("Enter password: ")
		if err != nil {
			return "", err
		}

		confirm, err := Password("Confirm password: ")
		if err != nil {
			return "", err
		}

		if password == confirm {
			return password, nil
		}

		fmt.Println("Passwords do not match. Please try again.")
	}
}
```

**Step 2: Create admin CLI command file**

Create `cmd/admin.go`:
```go
package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/markb/supalite/internal/admin"
	"github.com/markb/supalite/internal/config"
	"github.com/markb/supalite/internal/pg"
	"github.com/markb/supalite/internal/prompt"
	"github.com/spf13/cobra"
)

var adminCmd = &cobra.Command{
	Use:   "admin",
	Short: "Manage dashboard admin users",
}

var adminAddCmd = &cobra.Command{
	Use:   "add <email>",
	Short: "Add a new admin user",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		email := args[0]
		fmt.Printf("Creating admin user: %s\n", email)

		password, err := prompt.ConfirmPassword()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading password: %v\n", err)
			os.Exit(1)
		}

		cfg := config.Load()
		pgCfg := pg.Config{
			Port:     cfg.PGPort,
			Username: cfg.PGUsername,
			Password: cfg.PGPassword,
			Database: cfg.PGDatabase,
			DataDir:  cfg.DataDir,
			Version:  "16.9.0",
		}
		db := pg.NewEmbeddedDatabase(pgCfg)

		ctx := context.Background()
		if err := db.Start(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to start database: %v\n", err)
			os.Exit(1)
		}
		defer db.Stop()

		conn, err := db.Connect(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to connect to database: %v\n", err)
			os.Exit(1)
		}
		defer conn.Close(ctx)

		user, err := admin.Create(ctx, conn, email, password)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create user: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Admin user created: %s (ID: %s)\n", user.Email, user.ID)
	},
}

var adminChangePasswordCmd = &cobra.Command{
	Use:   "change-password <email>",
	Short: "Change an admin user's password",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		email := args[0]
		fmt.Printf("Changing password for: %s\n", email)

		password, err := prompt.ConfirmPassword()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading password: %v\n", err)
			os.Exit(1)
		}

		cfg := config.Load()
		pgCfg := pg.Config{
			Port:     cfg.PGPort,
			Username: cfg.PGUsername,
			Password: cfg.PGPassword,
			Database: cfg.PGDatabase,
			DataDir:  cfg.DataDir,
			Version:  "16.9.0",
		}
		db := pg.NewEmbeddedDatabase(pgCfg)

		ctx := context.Background()
		if err := db.Start(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to start database: %v\n", err)
			os.Exit(1)
		}
		defer db.Stop()

		conn, err := db.Connect(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to connect to database: %v\n", err)
			os.Exit(1)
		}
		defer conn.Close(ctx)

		if err := admin.UpdatePassword(ctx, conn, email, password); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to update password: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Password updated successfully")
	},
}

var adminDeleteCmd = &cobra.Command{
	Use:   "delete <email>",
	Short: "Delete an admin user",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		email := args[0]
		fmt.Printf("Are you sure you want to delete admin user '%s'? [y/N] ", email)

		var confirm string
		fmt.Scanln(&confirm)
		if confirm != "y" && confirm != "Y" {
			fmt.Println("Cancelled")
			return
		}

		cfg := config.Load()
		pgCfg := pg.Config{
			Port:     cfg.PGPort,
			Username: cfg.PGUsername,
			Password: cfg.PGPassword,
			Database: cfg.PGDatabase,
			DataDir:  cfg.DataDir,
			Version:  "16.9.0",
		}
		db := pg.NewEmbeddedDatabase(pgCfg)

		ctx := context.Background()
		if err := db.Start(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to start database: %v\n", err)
			os.Exit(1)
		}
		defer db.Stop()

		conn, err := db.Connect(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to connect to database: %v\n", err)
			os.Exit(1)
		}
		defer conn.Close(ctx)

		if err := admin.Delete(ctx, conn, email); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to delete user: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Admin user deleted: %s\n", email)
	},
}

var adminListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all admin users",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.Load()
		pgCfg := pg.Config{
			Port:     cfg.PGPort,
			Username: cfg.PGUsername,
			Password: cfg.PGPassword,
			Database: cfg.PGDatabase,
			DataDir:  cfg.DataDir,
			Version:  "16.9.0",
		}
		db := pg.NewEmbeddedDatabase(pgCfg)

		ctx := context.Background()
		if err := db.Start(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to start database: %v\n", err)
			os.Exit(1)
		}
		defer db.Stop()

		conn, err := db.Connect(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to connect to database: %v\n", err)
			os.Exit(1)
		}
		defer conn.Close(ctx)

		users, err := admin.List(ctx, conn)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to list users: %v\n", err)
			os.Exit(1)
		}

		if len(users) == 0 {
			fmt.Println("No admin users found")
			return
		}

		fmt.Println("\nAdmin Users:")
		fmt.Println("─────────────────────────────────────────────────────────")
		for _, u := range users {
			fmt.Printf("  %s (%s)\n", u.Email, u.ID)
			fmt.Printf("    Created: %s\n", u.CreatedAt.Format("2006-01-02 15:04:05"))
		}
		fmt.Println()
	},
}

func init() {
	rootCmd.AddCommand(adminCmd)
	adminCmd.AddCommand(adminAddCmd)
	adminCmd.AddCommand(adminChangePasswordCmd)
	adminCmd.AddCommand(adminDeleteCmd)
	adminCmd.AddCommand(adminListCmd)
}
```

**Step 3: Update init command to prompt for first admin user**

Modify `cmd/init.go`, add to the `runInit` function after schema initialization:

```go
	// After schema initialization, check for admin users
	count, err := admin.Count(ctx, conn)
	if err == nil && count == 0 {
		fmt.Println("\nNo admin users found. Let's create your first dashboard admin account.")

		email, err := prompt.Email("Enter admin email: ")
		if err != nil {
			log.Fatal("Failed to read email", err)
		}

		password, err := prompt.ConfirmPassword()
		if err != nil {
			log.Fatal("Failed to read password", err)
		}

		user, err := admin.Create(ctx, conn, email, password)
		if err != nil {
			log.Fatal("Failed to create admin user", err)
		}

		fmt.Println("\n✓ Admin user created successfully!")
		fmt.Printf("  Email: %s\n", user.Email)
		fmt.Println("  You can now login at: http://localhost:8080/_/\n")
	}
```

Also add the import at the top of `cmd/init.go`:
```go
import (
	"github.com/markb/supalite/internal/admin"
	"github.com/markb/supalite/internal/prompt"
)
```

**Step 4: Commit**

```bash
git add cmd/admin.go cmd/init.go internal/prompt/
git commit -m "feat(cli): add admin user management commands

- Add 'supalite admin add' to create new admin users
- Add 'supalite admin change-password' to update passwords
- Add 'supalite admin delete' to remove admin users
- Add 'supalite admin list' to list all admin users
- Update 'supalite init' to prompt for first admin user
- Add internal/prompt package for secure password input

Co-Authored-By: Claude (GLM-4.7) <noreply@anthropic.com>"
```

---

## Task 4: Create Dashboard Package with JWT Support

**Files:**
- Create: `internal/dashboard/jwt.go`
- Create: `internal/dashboard/server.go`
- Create: `internal/dashboard/api.go`
- Modify: `internal/server/server.go` (mount dashboard routes)

**Step 1: Create JWT utilities for dashboard auth**

Create `internal/dashboard/jwt.go`:
```go
package dashboard

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	Email string `json:"email"`
	Role  string `json:"role"`
	jwt.RegisteredClaims
}

// GenerateToken creates a JWT token for an admin user
func GenerateToken(email string, userID uuid.UUID, secret string) (string, error) {
	now := time.Now()
	expiry := now.Add(24 * time.Hour)

	claims := Claims{
		Email: email,
		Role:  "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiry),
			Issuer:    "supalite",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// VerifyToken validates a JWT token and returns the claims
func VerifyToken(tokenString, secret string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}
```

**Step 2: Add JWT dependency**

Run:
```bash
go get github.com/golang-jwt/jwt/v5
go get github.com/google/uuid
```

**Step 3: Create dashboard server with embedded filesystem**

Create `internal/dashboard/server.go`:
```go
package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/markb/supalite/internal/admin"
	"github.com/markb/supalite/internal/keys"
	"github.com/rs/cors"
)

type Server struct {
	router      *chi.Mux
	jwtSecret   string
	keyManager  *keys.Manager
	pgConnector func(context.Context) (*pgx.Conn, error)
}

type Config struct {
	JWTSecret   string
	KeyManager  *keys.Manager
	PGConnector func(context.Context) (*pgx.Conn, error)
}

func New(cfg Config) *Server {
	s := &Server{
		router:      chi.NewRouter(),
		jwtSecret:   cfg.JWTSecret,
		keyManager:  cfg.KeyManager,
		pgConnector: cfg.PGConnector,
	}
	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	// Public routes
	s.router.Post("/api/login", s.handleLogin)

	// Protected routes (require JWT)
	s.router.Group(func(r chi.Router) {
		r.Use(s.authMiddleware)
		s.router.Get("/api/me", s.handleMe)
		s.router.Get("/api/status", s.handleStatus)
		s.router.Get("/api/tables", s.handleListTables)
	})

	// Serve static files (SPA - handle client-side routing)
	s.router.Get("/*", s.handleStatic)
}

func (s *Server) Handler() http.Handler {
	return cors.AllowAll().Handler(s.router)
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "missing authorization header", http.StatusUnauthorized)
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		_, err := VerifyToken(tokenString, s.jwtSecret)
		if err != nil {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}
```

**Step 4: Create API handlers**

Create `internal/dashboard/api.go`:
```go
package dashboard

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/markb/supalite/internal/admin"
)

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string      `json:"token"`
	User   admin.User `json:"user"`
}

type StatusResponse struct {
	PostgreSQL string `json:"postgresql"`
	PREST      string `json:"prest"`
	GoTrue     string `json:"gotrue"`
	Healthy    bool   `json:"healthy"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	conn, err := s.pgConnector(ctx)
	if err != nil {
		http.Error(w, "database connection error", http.StatusInternalServerError)
		return
	}
	defer conn.Close(ctx)

	user, err := admin.FindByEmail(ctx, conn, req.Email)
	if err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	if !admin.VerifyPassword(req.Password, user.PasswordHash) {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	token, err := GenerateToken(user.Email, user.ID, s.jwtSecret)
	if err != nil {
		http.Error(w, "failed to generate token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(LoginResponse{
		Token: token,
		User:  *user,
	})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	claims, err := VerifyToken(tokenString, s.jwtSecret)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	ctx := r.Context()
	conn, err := s.pgConnector(ctx)
	if err != nil {
		http.Error(w, "database connection error", http.StatusInternalServerError)
		return
	}
	defer conn.Close(ctx)

	user, err := admin.FindByEmail(ctx, conn, claims.Email)
	if err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement actual status checks
	status := StatusResponse{
		PostgreSQL: "running",
		PREST:      "running",
		GoTrue:     "running",
		Healthy:    true,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (s *Server) handleListTables(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	conn, err := s.pgConnector(ctx)
	if err != nil {
		http.Error(w, "database connection error", http.StatusInternalServerError)
		return
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx, `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public'
		ORDER BY table_name
	`)
	if err != nil {
		http.Error(w, "query error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			continue
		}
		tables = append(tables, table)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tables)
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	// TODO: Serve embedded static files
	// For now, return a placeholder
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>Supalite Dashboard</title></head>
<body>
<h1>Supalite Dashboard</h1>
<p>Dashboard files will be embedded here once built.</p>
<p>Login API: POST /_/api/login with {email, password}</p>
</body>
</html>`))
}
```

Add missing imports to `internal/dashboard/server.go`:
```go
import (
	// ... existing imports ...
	"github.com/jackc/pgx/v5"
	"strings"
)
```

**Step 5: Integrate dashboard into main server**

Modify `internal/server/server.go`:

1. Add dashboard field to Server struct:
```go
type Server struct {
	config        Config
	router        *chi.Mux
	httpServer    *http.Server
	// ... existing fields ...
	dashboardServer *dashboard.Server  // Add this line
}
```

2. Add JWT secret generation in Start method (after key manager init):
```go
	// After key manager initialization (around line 164)

	// Generate JWT secret for dashboard auth (if not provided)
	jwtSecret := s.config.JWTSecret
	if jwtSecret == "" {
		// Use a random secret for dashboard auth
		jwtSecret = generateRandomSecret(32)
	}
```

3. Create and mount dashboard routes (add in setupRoutes function):
```go
func (s *Server) setupRoutes() {
	s.router.Get("/health", s.handleHealth)
	s.router.HandleFunc("/.well-known/jwks.json", s.handleJWKS)
	s.router.HandleFunc("/rest/v1", s.handleSupabaseREST)
	s.router.HandleFunc("/rest/v1/*", s.handleSupabaseREST)
	s.router.HandleFunc("/auth/v1/*", s.handleAuthRequest)

	// Mount dashboard at /_/
	s.dashboardServer = dashboard.New(dashboard.Config{
		JWTSecret:   jwtSecret,  // Pass from server scope
		KeyManager:  s.keyManager,
		PGConnector: s.pgDatabase.Connect,
	})
	s.router.Mount("/_/", s.dashboardServer.Handler())
}
```

**Step 6: Add import to main server**

Add to `internal/server/server.go` imports:
```go
import (
	// ... existing imports ...
	"github.com/markb/supalite/internal/dashboard"
)
```

**Step 7: Commit**

```bash
git add internal/dashboard/ internal/server/server.go go.mod go.sum
git commit -m "feat(dashboard): add dashboard server with JWT authentication

- Create dashboard package with embedded filesystem support
- Add JWT token generation and verification for admin auth
- Implement login API endpoint (POST /_/api/login)
- Add protected API endpoints (me, status, tables)
- Mount dashboard at /_/ in main server
- Add auth middleware for protected routes

Co-Authored-By: Claude (GLM-4.7) <noreply@anthropic.com>"
```

---

## Task 5: Create Frontend Project Foundation

**Files:**
- Create: `dashboard/` directory structure
- Create: `dashboard/package.json`
- Create: `dashboard/vite.config.ts`
- Create: `dashboard/tsconfig.json`
- Create: `dashboard/postcss.config.js`
- Create: `dashboard/tailwind.config.js`
- Create: `dashboard/index.html`
- Create: `dashboard/src/main.tsx`
- Create: `dashboard/src/App.tsx`
- Create: `dashboard/src/index.css`

**Step 1: Create dashboard directory and initialize**

Run:
```bash
mkdir -p dashboard/src/{components,pages,lib}
cd dashboard
npm init -y
```

**Step 2: Install dependencies**

Run:
```bash
cd dashboard
npm install react react-dom react-router-dom
npm install -D typescript @types/react @types/react-dom
npm install -D vite @vitejs/plugin-react
npm install -D tailwindcss postcss autoprefixer
npx tailwindcss init -p
```

**Step 3: Create package.json**

Create `dashboard/package.json`:
```json
{
  "name": "supalite-dashboard",
  "private": true,
  "version": "0.0.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc && vite build",
    "preview": "vite preview"
  },
  "dependencies": {
    "react": "^18.3.1",
    "react-dom": "^18.3.1",
    "react-router-dom": "^6.22.0"
  },
  "devDependencies": {
    "@types/react": "^18.3.0",
    "@types/react-dom": "^18.3.0",
    "@vitejs/plugin-react": "^4.2.1",
    "autoprefixer": "^10.4.18",
    "postcss": "^8.4.35",
    "tailwindcss": "^3.4.1",
    "typescript": "^5.3.3",
    "vite": "^5.1.4"
  }
}
```

**Step 4: Create vite.config.ts**

Create `dashboard/vite.config.ts`:
```ts
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/rest': 'http://localhost:8080',
      '/auth': 'http://localhost:8080',
      '/_/api': 'http://localhost:8080',
    }
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  }
})
```

**Step 5: Create tsconfig.json**

Create `dashboard/tsconfig.json`:
```json
{
  "compilerOptions": {
    "target": "ES2020",
    "useDefineForClassFields": true,
    "lib": ["ES2020", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "skipLibCheck": true,
    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,
    "resolveJsonModule": true,
    "isolatedModules": true,
    "noEmit": true,
    "jsx": "react-jsx",
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noFallthroughCasesInSwitch": true
  },
  "include": ["src"],
  "references": [{ "path": "./tsconfig.node.json" }]
}
```

**Step 6: Create tsconfig.node.json**

Create `dashboard/tsconfig.node.json`:
```json
{
  "compilerOptions": {
    "composite": true,
    "skipLibCheck": true,
    "module": "ESNext",
    "moduleResolution": "bundler",
    "allowSyntheticDefaultImports": true
  },
  "include": ["vite.config.ts"]
}
```

**Step 7: Create tailwind.config.js**

Create `dashboard/tailwind.config.js`:
```js
/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {},
  },
  plugins: [],
}
```

**Step 8: Create postcss.config.js**

Create `dashboard/postcss.config.js`:
```js
export default {
  plugins: {
    tailwindcss: {},
    autoprefixer: {},
  },
}
```

**Step 9: Create index.html**

Create `dashboard/index.html`:
```html
<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>Supalite Dashboard</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

**Step 10: Create src/index.css**

Create `dashboard/src/index.css`:
```css
@tailwind base;
@tailwind components;
@tailwind utilities;

body {
  margin: 0;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Roboto', 'Oxygen',
    'Ubuntu', 'Cantarell', 'Fira Sans', 'Droid Sans', 'Helvetica Neue',
    sans-serif;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}
```

**Step 11: Create src/main.tsx**

Create `dashboard/src/main.tsx`:
```tsx
import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App.tsx'
import './index.css'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
)
```

**Step 12: Create src/App.tsx**

Create `dashboard/src/App.tsx`:
```tsx
import { BrowserRouter as Router, Routes, Route } from 'react-router-dom'

function App() {
  return (
    <Router>
      <Routes>
        <Route path="/login" element={<div>Login Page</div>} />
        <Route path="/" element={<div>Dashboard Home</div>} />
      </Routes>
    </Router>
  )
}

export default App
```

**Step 13: Test the build**

Run:
```bash
cd dashboard
npm run build
```

Expected: Creates `dashboard/dist/` directory with built files

**Step 14: Commit**

```bash
git add dashboard/
git commit -m "feat(dashboard): initialize frontend project with Vite+React+Tailwind

- Create dashboard/ directory with Vite, React, TypeScript setup
- Configure Tailwind CSS for styling
- Set up Vite dev server with API proxy to Go backend
- Add basic routing with react-router-dom
- Configure build output for embedding in Go binary

Co-Authored-By: Claude (GLM-4.7) <noreply@anthropic.com>"
```

---

## Task 6: Create API Client and Login Page

**Files:**
- Create: `dashboard/src/lib/api.ts`
- Create: `dashboard/src/pages/LoginPage.tsx`
- Modify: `dashboard/src/App.tsx`

**Step 1: Create API client**

Create `dashboard/src/lib/api.ts`:
```ts
const API_BASE = '/_/api'

export function setToken(token: string) {
  sessionStorage.setItem('admin_token', token)
}

export function getToken(): string | null {
  return sessionStorage.getItem('admin_token')
}

export function clearToken() {
  sessionStorage.removeItem('admin_token')
}

async function authFetch(url: string, options: RequestInit = {}) {
  const token = getToken()
  return fetch(url, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...options.headers,
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
  })
}

export interface User {
  id: string
  email: string
  created_at: string
  updated_at: string
}

export const api = {
  login: (email: string, password: string) =>
    authFetch(`${API_BASE}/login`, {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    }),

  me: () => authFetch(`${API_BASE}/me`),

  logout: () => {
    clearToken()
    return Promise.resolve()
  },

  getStatus: () => authFetch(`${API_BASE}/status`),

  getTables: () => authFetch(`${API_BASE}/tables`),
}
```

**Step 2: Create login page**

Create `dashboard/src/pages/LoginPage.tsx`:
```tsx
import { useState, FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, setToken } from '../lib/api'

export function LoginPage() {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const navigate = useNavigate()

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)

    try {
      const res = await api.login(email, password)

      if (res.ok) {
        const data = await res.json()
        setToken(data.token)
        navigate('/')
      } else {
        const data = await res.json().catch(() => ({}))
        setError(data.error || 'Login failed')
        setLoading(false)
      }
    } catch (err) {
      setError('Network error. Please try again.')
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-50">
      <div className="max-w-md w-full">
        <div className="bg-white py-8 px-4 shadow sm:rounded-lg sm:px-10">
          <div className="sm:mx-auto sm:w-full sm:max-w-md mb-6">
            <h1 className="text-center text-3xl font-bold text-gray-900">
              Supalite Dashboard
            </h1>
          </div>

          <form onSubmit={handleSubmit} className="space-y-6">
            <div>
              <label htmlFor="email" className="block text-sm font-medium text-gray-700">
                Email
              </label>
              <input
                id="email"
                type="email"
                required
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500"
              />
            </div>

            <div>
              <label htmlFor="password" className="block text-sm font-medium text-gray-700">
                Password
              </label>
              <input
                id="password"
                type="password"
                required
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-blue-500"
              />
            </div>

            {error && (
              <div className="rounded-md bg-red-50 p-4">
                <p className="text-sm text-red-800">{error}</p>
              </div>
            )}

            <button
              type="submit"
              disabled={loading}
              className="w-full flex justify-center rounded-md border border-transparent bg-blue-600 py-2 px-4 text-sm font-medium text-white shadow-sm hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {loading ? 'Logging in...' : 'Login'}
            </button>
          </form>
        </div>
      </div>
    </div>
  )
}
```

**Step 3: Create protected route component**

Create `dashboard/src/components/ProtectedRoute.tsx`:
```tsx
import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../lib/api'

export function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const [verified, setVerified] = useState(false)
  const [valid, setValid] = useState(false)
  const navigate = useNavigate()

  useEffect(() => {
    api.me().then((res) => {
      if (res.ok) {
        setValid(true)
      } else {
        setValid(false)
        navigate('/login')
      }
      setVerified(true)
    }).catch(() => {
      setValid(false)
      navigate('/login')
      setVerified(true)
    })
  }, [navigate])

  if (!verified) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <p className="text-gray-600">Loading...</p>
      </div>
    )
  }

  if (!valid) {
    return null
  }

  return <>{children}</>
}
```

**Step 4: Update App.tsx with routing**

Replace `dashboard/src/App.tsx`:
```tsx
import { BrowserRouter as Router, Routes, Route, Navigate } from 'react-router-dom'
import { LoginPage } from './pages/LoginPage'
import { ProtectedRoute } from './components/ProtectedRoute'
import { api, getToken } from './lib/api'
import { useEffect, useState } from 'react'

function DashboardHome() {
  const [status, setStatus] = useState<any>(null)

  useEffect(() => {
    api.getStatus().then(res => res.json()).then(setStatus)
  }, [])

  return (
    <div className="min-h-screen bg-gray-50">
      <header className="bg-white shadow">
        <div className="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
          <h1 className="text-3xl font-bold text-gray-900">Supalite Dashboard</h1>
        </div>
      </header>
      <main className="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
        <div className="bg-white shadow rounded-lg p-6">
          <h2 className="text-xl font-semibold mb-4">System Status</h2>
          {status ? (
            <pre className="bg-gray-100 p-4 rounded">{JSON.stringify(status, null, 2)}</pre>
          ) : (
            <p className="text-gray-600">Loading...</p>
          )}
        </div>
      </main>
    </div>
  )
}

function App() {
  const [checkingAuth, setCheckingAuth] = useState(true)
  const [isAuthenticated, setIsAuthenticated] = useState(false)

  useEffect(() => {
    const token = getToken()
    if (token) {
      api.me().then(res => {
        setIsAuthenticated(res.ok)
        setCheckingAuth(false)
      }).catch(() => {
        setIsAuthenticated(false)
        setCheckingAuth(false)
      })
    } else {
      setCheckingAuth(false)
    }
  }, [])

  if (checkingAuth) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <p className="text-gray-600">Loading...</p>
      </div>
    )
  }

  return (
    <Router>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route
          path="/"
          element={
            <ProtectedRoute>
              <DashboardHome />
            </ProtectedRoute>
          }
        />
      </Routes>
    </Router>
  )
}

export default App
```

**Step 5: Test the build**

Run:
```bash
cd dashboard
npm run build
```

**Step 6: Commit**

```bash
git add dashboard/
git commit -m "feat(dashboard): add login page and API client

- Create API client with JWT token management
- Implement login page with email/password form
- Add protected route component for authenticated pages
- Create dashboard home page with system status display
- Add session storage for token persistence

Co-Authored-By: Claude (GLM-4.7) <noreply@anthropic.com>"
```

---

## Task 7: Embed Dashboard in Go Binary

**Files:**
- Modify: `internal/dashboard/server.go` (update static handler)
- Modify: `Makefile` (add build steps)

**Step 1: Update dashboard static handler to serve embedded files**

Modify `internal/dashboard/server.go`, replace the `handleStatic` function:
```go
package dashboard

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
	// ... other imports
)

//go:embed all:../../dashboard/dist
var distFS embed.FS

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	// Strip /_/ prefix to get path relative to dist
	requestPath := strings.TrimPrefix(r.URL.Path, "/_/")

	// Try to serve file
	fsys, _ := fs.Sub(distFS, "dashboard/dist")

	// If requesting root or a directory, serve index.html
	if requestPath == "" || requestPath == "/" {
		requestPath = "/index.html"
	}

	// Serve the file
	http.FileServer(http.FS(fsys)).ServeHTTP(w, &http.Request{
		Method: http.MethodGet,
		URL:    &url.URL{Path: requestPath},
	})
}
```

Add missing import:
```go
import (
	// ...
	"io/fs"
	"net/url"
)
```

**Step 2: Create empty dist directory for initial build**

Run:
```bash
cd dashboard
npm run build
```

**Step 3: Update Makefile**

Add to `Makefile`:
```makefile
# Build dashboard first, then Go binary
build: build-dashboard build-go

build-dashboard:
	@echo "Building dashboard..."
	cd dashboard && npm run build
	@echo "Dashboard built to dashboard/dist/"

build-go:
	@echo "Building Go binary..."
	go build -ldflags "$(LDFLAGS)" -o supalite ./cmd/main.go

# Override existing build target
.PHONY: build build-dashboard build-go
```

**Step 4: Test the full build**

Run:
```bash
make build
```

Expected: Dashboard built, then Go binary compiled with embedded files

**Step 5: Test running the server**

Run:
```bash
./supalite serve
```

Then visit: `http://localhost:8080/_/`

Expected: See login page

**Step 6: Commit**

```bash
git add internal/dashboard/server.go Makefile
git commit -m "feat(dashboard): embed built frontend in Go binary

- Add //go:embed directive for dashboard/dist directory
- Update static handler to serve embedded files
- Update Makefile to build dashboard before Go binary
- Serve index.html for SPA routing

Co-Authored-By: Claude (GLM-4.7) <noreply@anthropic.com>"
```

---

## Task 8: Create Overview Page

**Files:**
- Create: `dashboard/src/pages/OverviewPage.tsx`
- Create: `dashboard/src/components/StatusCard.tsx`
- Create: `dashboard/src/components/ApiKeyCard.tsx`
- Modify: `dashboard/src/App.tsx`

**Step 1: Create status card component**

Create `dashboard/src/components/StatusCard.tsx`:
```tsx
interface StatusCardProps {
  title: string
  status: 'healthy' | 'unhealthy' | 'unknown'
  details?: string
}

export function StatusCard({ title, status, details }: StatusCardProps) {
  const statusColors = {
    healthy: 'bg-green-100 text-green-800',
    unhealthy: 'bg-red-100 text-red-800',
    unknown: 'bg-gray-100 text-gray-800',
  }

  return (
    <div className="bg-white shadow rounded-lg p-6">
      <div className="flex items-center justify-between">
        <h3 className="text-lg font-medium text-gray-900">{title}</h3>
        <span className={`inline-flex rounded-full px-3 py-1 text-sm font-medium ${statusColors[status]}`}>
          {status}
        </span>
      </div>
      {details && <p className="mt-2 text-sm text-gray-600">{details}</p>}
    </div>
  )
}
```

**Step 2: Create API key card component**

Create `dashboard/src/components/ApiKeyCard.tsx`:
```tsx
interface ApiKeyCardProps {
  label: string
  value: string
}

export function ApiKeyCard({ label, value }: ApiKeyCardProps) {
  const [copied, setCopied] = useState(false)

  const copyToClipboard = () => {
    navigator.clipboard.writeText(value)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  const maskedValue = value.slice(0, 20) + '...'

  return (
    <div className="bg-white shadow rounded-lg p-6">
      <h3 className="text-sm font-medium text-gray-500">{label}</h3>
      <div className="mt-2 flex items-center justify-between">
        <code className="text-sm text-gray-900 font-mono">{maskedValue}</code>
        <button
          onClick={copyToClipboard}
          className="ml-4 text-blue-600 hover:text-blue-800 text-sm font-medium"
        >
          {copied ? 'Copied!' : 'Copy'}
        </button>
      </div>
    </div>
  )
}
```

Add useState import:
```tsx
import { useState } from 'react'
```

**Step 3: Create overview page**

Create `dashboard/src/pages/OverviewPage.tsx`:
```tsx
import { useEffect, useState } from 'react'
import { StatusCard } from '../components/StatusCard'
import { ApiKeyCard } from '../components/ApiKeyCard'
import { api } from '../lib/api'

interface Status {
  postgresql: string
  prest: string
  gotrue: string
  healthy: boolean
}

export function OverviewPage() {
  const [status, setStatus] = useState<Status | null>(null)
  const [anonKey, setAnonKey] = useState('')
  const [serviceKey, setServiceKey] = useState('')

  useEffect(() => {
    api.getStatus().then(res => res.json()).then(setStatus)

    // Get keys from environment or server
    // For now, use placeholder
    setAnonKey('eyJhbGciOiJFUzI1NiIsInR5cCI6IkpXVCJ9...')
    setServiceKey('eyJhbGciOiJFUzI1NiIsInR5cCI6IkpXVCJ9...')
  }, [])

  return (
    <div className="min-h-screen bg-gray-50">
      <header className="bg-white shadow">
        <div className="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
          <h1 className="text-3xl font-bold text-gray-900">Dashboard</h1>
        </div>
      </header>

      <main className="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
        <div className="mb-8">
          <h2 className="text-lg font-medium text-gray-900 mb-4">System Status</h2>
          <div className="grid grid-cols-1 gap-6 sm:grid-cols-3">
            <StatusCard
              title="PostgreSQL"
              status={status?.postgresql === 'running' ? 'healthy' : 'unknown'}
              details={status?.postgresql}
            />
            <StatusCard
              title="pREST"
              status={status?.prest === 'running' ? 'healthy' : 'unknown'}
              details={status?.prest}
            />
            <StatusCard
              title="GoTrue"
              status={status?.gotrue === 'running' ? 'healthy' : 'unknown'}
              details={status?.gotrue}
            />
          </div>
        </div>

        <div>
          <h2 className="text-lg font-medium text-gray-900 mb-4">API Keys</h2>
          <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
            <ApiKeyCard label="Anon Key (Public)" value={anonKey} />
            <ApiKeyCard label="Service Role Key (Secret)" value={serviceKey} />
          </div>
        </div>
      </main>
    </div>
  )
}
```

**Step 4: Update App.tsx to use overview page**

Update `dashboard/src/App.tsx`:
```tsx
function DashboardHome() {
  return <OverviewPage />
}
```

Add import:
```tsx
import { OverviewPage } from './pages/OverviewPage'
```

**Step 5: Build and test**

Run:
```bash
cd dashboard && npm run build
make build
./supalite serve
```

**Step 6: Commit**

```bash
git add dashboard/
git commit -m "feat(dashboard): add overview page with system status

- Create StatusCard component for service status display
- Create ApiKeyCard component for key display with copy function
- Create OverviewPage showing system status and API keys
- Update routing to use OverviewPage as dashboard home

Co-Authored-By: Claude (GLM-4.7) <noreply@anthropic.com>"
```

---

## Task 9: Add Logout Functionality

**Files:**
- Create: `dashboard/src/components/Header.tsx`
- Modify: `dashboard/src/pages/OverviewPage.tsx`
- Modify: `dashboard/src/App.tsx`

**Step 1: Create header component**

Create `dashboard/src/components/Header.tsx`:
```tsx
import { useNavigate } from 'react-router-dom'
import { api } from '../lib/api'

export function Header() {
  const navigate = useNavigate()

  const handleLogout = () => {
    api.logout()
    navigate('/login')
  }

  return (
    <header className="bg-white shadow">
      <div className="mx-auto max-w-7xl px-4 py-4 sm:px-6 lg:px-8 flex items-center justify-between">
        <h1 className="text-2xl font-bold text-gray-900">Supalite Dashboard</h1>
        <button
          onClick={handleLogout}
          className="text-gray-600 hover:text-gray-900 font-medium"
        >
          Logout
        </button>
      </div>
    </header>
  )
}
```

**Step 2: Update overview page to use header**

Modify `dashboard/src/pages/OverviewPage.tsx`:
```tsx
import { Header } from '../components/Header'

export function OverviewPage() {
  // ... existing code ...

  return (
    <div className="min-h-screen bg-gray-50">
      <Header />
      {/* ... rest of the page */}
    </div>
  )
}
```

Remove the old header from the return statement.

**Step 3: Build and test**

Run:
```bash
cd dashboard && npm run build
```

**Step 4: Commit**

```bash
git add dashboard/
git commit -m "feat(dashboard): add header with logout functionality

- Create Header component with logout button
- Add logout handler that clears token and redirects to login
- Update OverviewPage to use new Header component

Co-Authored-By: Claude (GLM-4.7) <noreply@anthropic.com>"
```

---

## Task 10: Add Navigation and Table Browser

**Files:**
- Create: `dashboard/src/components/Navigation.tsx`
- Create: `dashboard/src/pages/TablesPage.tsx`
- Modify: `dashboard/src/lib/api.ts`
- Modify: `dashboard/src/App.tsx`
- Modify: `internal/dashboard/api.go` (add table schema endpoint)

**Step 1: Add table schema API endpoint**

Modify `internal/dashboard/api.go`, add:
```go
func (s *Server) handleGetTableSchema(w http.ResponseWriter, r *http.Request) {
	tableName := r.URL.Query().Get("table")
	if tableName == "" {
		http.Error(w, "table parameter required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	conn, err := s.pgConnector(ctx)
	if err != nil {
		http.Error(w, "database connection error", http.StatusInternalServerError)
		return
	}
	defer conn.Close(ctx)

	query := `
		SELECT column_name, data_type, is_nullable, column_default
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = $1
		ORDER BY ordinal_position
	`

	rows, err := conn.Query(ctx, query, tableName)
	if err != nil {
		http.Error(w, "query error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var columns []map[string]interface{}
	for rows.Next() {
		var name, dataType, isNullable string
		var defaultVal *string
		if err := rows.Scan(&name, &dataType, &isNullable, &defaultVal); err != nil {
			continue
		}
		columns = append(columns, map[string]interface{}{
			"name":       name,
			"type":       dataType,
			"nullable":   isNullable == "YES",
			"default":    defaultVal,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(columns)
}
```

Add route in `setupRoutes`:
```go
s.router.Get("/api/tables/schema", s.handleGetTableSchema)
```

**Step 2: Update API client**

Add to `dashboard/src/lib/api.ts`:
```ts
getTableSchema: (table: string) =>
  authFetch(`${API_BASE}/tables/schema?table=${table}`),

getTableData: (table: string, query?: string) =>
  authFetch(`/rest/v1/${table}${query ? `?${query}` : ''}`),
```

**Step 3: Create navigation component**

Create `dashboard/src/components/Navigation.tsx`:
```tsx
import { NavLink } from 'react-router-dom'

const navItems = [
  { path: '/', label: 'Overview' },
  { path: '/tables', label: 'Tables' },
  { path: '/auth', label: 'Authentication' },
  { path: '/api-keys', label: 'API Keys' },
  { path: '/settings', label: 'Settings' },
]

export function Navigation() {
  return (
    <nav className="bg-white shadow-sm border-b">
      <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
        <div className="flex space-x-8">
          {navItems.map((item) => (
            <NavLink
              key={item.path}
              to={item.path}
              className={({ isActive }) =>
                `inline-flex items-center px-1 pt-1 border-b-2 text-sm font-medium ${
                  isActive
                    ? 'border-blue-500 text-gray-900'
                    : 'border-transparent text-gray-500 hover:border-gray-300 hover:text-gray-700'
                }`
              }
            >
              {item.label}
            </NavLink>
          ))}
        </div>
      </div>
    </nav>
  )
}
```

**Step 4: Create tables page**

Create `dashboard/src/pages/TablesPage.tsx`:
```tsx
import { useEffect, useState } from 'react'
import { api } from '../lib/api'
import { Header } from '../components/Header'
import { Navigation } from '../components/Navigation'

export function TablesPage() {
  const [tables, setTables] = useState<string[]>([])
  const [selectedTable, setSelectedTable] = useState<string | null>(null)
  const [schema, setSchema] = useState<any[]>([])
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    api.getTables().then(res => res.json()).then(setTables)
  }, [])

  useEffect(() => {
    if (selectedTable) {
      setLoading(true)
      api.getTableSchema(selectedTable).then(res => res.json()).then(data => {
        setSchema(data)
        setLoading(false)
      })
    }
  }, [selectedTable])

  return (
    <div className="min-h-screen bg-gray-50">
      <Header />
      <Navigation />
      <main className="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
        <div className="grid grid-cols-1 lg:grid-cols-4 gap-6">
          {/* Table list */}
          <div className="bg-white shadow rounded-lg p-6">
            <h2 className="text-lg font-medium text-gray-900 mb-4">Tables</h2>
            <ul className="space-y-2">
              {tables.map(table => (
                <li key={table}>
                  <button
                    onClick={() => setSelectedTable(table)}
                    className={`w-full text-left px-3 py-2 rounded-md text-sm ${
                      selectedTable === table
                        ? 'bg-blue-100 text-blue-700'
                        : 'text-gray-700 hover:bg-gray-100'
                    }`}
                  >
                    {table}
                  </button>
                </li>
              ))}
            </ul>
          </div>

          {/* Table details */}
          <div className="lg:col-span-3 bg-white shadow rounded-lg p-6">
            {selectedTable ? (
              <>
                <h2 className="text-lg font-medium text-gray-900 mb-4">
                  {selectedTable}
                </h2>
                {loading ? (
                  <p className="text-gray-600">Loading...</p>
                ) : (
                  <table className="min-w-full divide-y divide-gray-200">
                    <thead className="bg-gray-50">
                      <tr>
                        <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                          Column
                        </th>
                        <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                          Type
                        </th>
                        <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                          Nullable
                        </th>
                      </tr>
                    </thead>
                    <tbody className="bg-white divide-y divide-gray-200">
                      {schema.map((col, i) => (
                        <tr key={i}>
                          <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
                            {col.name}
                          </td>
                          <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                            {col.type}
                          </td>
                          <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                            {col.nullable ? 'Yes' : 'No'}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                )}
              </>
            ) : (
              <p className="text-gray-600">Select a table to view its schema</p>
            )}
          </div>
        </div>
      </main>
    </div>
  )
}
```

**Step 5: Update App.tsx with routes**

Update `dashboard/src/App.tsx`:
```tsx
import { TablesPage } from './pages/TablesPage'

function App() {
  // ... existing code ...

  return (
    <Router>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route
          path="/"
          element={
            <ProtectedRoute>
              <OverviewPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/tables"
          element={
            <ProtectedRoute>
              <TablesPage />
            </ProtectedRoute>
          }
        />
        {/* Add other routes later */}
      </Routes>
    </Router>
  )
}
```

**Step 6: Update OverviewPage to include Navigation**

Modify `dashboard/src/pages/OverviewPage.tsx`:
```tsx
import { Navigation } from '../components/Navigation'

return (
  <div className="min-h-screen bg-gray-50">
    <Header />
    <Navigation />
    {/* ... rest of page */}
  </div>
)
```

**Step 7: Build and test**

Run:
```bash
cd dashboard && npm run build
```

**Step 8: Commit**

```bash
git add dashboard/ internal/dashboard/api.go
git commit -m "feat(dashboard): add table browser with navigation

- Create Navigation component with tab-based routing
- Create TablesPage showing all tables and their schema
- Add /api/tables/schema endpoint for table metadata
- Add Navigation to OverviewPage
- Update routing in App.tsx for /tables route

Co-Authored-By: Claude (GLM-4.7) <noreply@anthropic.com>"
```

---

## Completion

All core features implemented. The dashboard now has:

1. ✅ Admin authentication (login/logout)
2. ✅ Overview page with system status
3. ✅ Table browser with schema viewer
4. ✅ Navigation between pages
5. ✅ Embedded in Go binary
6. ✅ Development mode with Vite

**Remaining tasks (future phases):**
- Auth management page (view auth.users)
- API keys page
- Settings page
- Table data viewer/editor
- SQL query editor

**Testing:**
```bash
# Test CLI commands
./supalite admin list
./supalite admin add test@example.com
./supalite admin delete test@example.com

# Test dashboard
make build
./supalite serve
# Visit http://localhost:8080/_/
```
