package dashboard

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/markb/supalite/internal/log"
	"golang.org/x/crypto/bcrypt"
)

// loginRequest represents the JSON body for login requests.
//
// The login endpoint accepts email and password credentials.
type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// loginResponse represents the JSON response for successful login.
//
// Returns the JWT access token and user information.
type loginResponse struct {
	AccessToken string `json:"access_token"`
	User        user   `json:"user"`
}

// user represents a user in the response.
type user struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

// meResponse represents the response for /api/me endpoint.
//
// Returns information about the currently authenticated user.
type meResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

// statusResponse represents the server status response.
//
// Provides information about the server state and services.
type statusResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Uptime    string    `json:"uptime"`
	Version   string    `json:"version"`
}

// tableInfo represents information about a database table.
//
// Used in the /api/tables response to list available tables.
type tableInfo struct {
	Name      string `json:"name"`
	Schema    string `json:"schema"`
	Rows      int64  `json:"rows,omitempty"`
	SizeBytes string `json:"size_bytes,omitempty"`
}

// tablesResponse represents the response for /api/tables endpoint.
//
// Returns a list of tables in the database.
type tablesResponse struct {
	Tables []tableInfo `json:"tables"`
}

// handleLogin processes admin login requests.
//
// POST /api/login
//
// Accepts JSON body with email and password. Validates credentials
// against the admin.users table and returns a JWT token on success.
//
// Request body:
//   {
//     "email": "admin@example.com",
//     "password": "password123"
//   }
//
// Response (200 OK):
//   {
//     "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
//     "user": {
//       "id": "uuid",
//       "email": "admin@example.com"
//     }
//   }
//
// Returns 400 for invalid JSON, 401 for invalid credentials,
// or 500 for server errors.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	// Parse request body
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate input
	if req.Email == "" || req.Password == "" {
		http.Error(w, "email and password are required", http.StatusBadRequest)
		return
	}

	// Connect to database
	ctx := r.Context()
	conn, err := s.pgConnector.Connect(ctx)
	if err != nil {
		log.Error("dashboard login: database connection failed", "error", err)
		http.Error(w, "database connection failed", http.StatusInternalServerError)
		return
	}
	defer conn.Close(ctx)

	// Query user from admin.users table
	var userID string
	var passwordHash string
	query := `SELECT id, password_hash FROM admin.users WHERE email = $1`
	err = conn.QueryRow(ctx, query, req.Email).Scan(&userID, &passwordHash)
	if err != nil {
		if err == pgx.ErrNoRows {
			http.Error(w, "invalid email or password", http.StatusUnauthorized)
			return
		}
		log.Error("dashboard login: user query failed", "error", err)
		http.Error(w, "database query failed", http.StatusInternalServerError)
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		http.Error(w, "invalid email or password", http.StatusUnauthorized)
		return
	}

	// Generate JWT token
	token, err := s.jwtManager.GenerateToken(req.Email)
	if err != nil {
		log.Error("dashboard login: token generation failed", "error", err)
		http.Error(w, "token generation failed", http.StatusInternalServerError)
		return
	}

	// Return token and user info
	response := loginResponse{
		AccessToken: token,
		User: user{
			ID:    userID,
			Email: req.Email,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
	log.Info("dashboard login successful", "email", req.Email)
}

// handleMe returns information about the currently authenticated user.
//
// GET /api/me
//
// Requires valid JWT token in Authorization header.
//
// Response (200 OK):
//   {
//     "id": "uuid",
//     "email": "admin@example.com"
//   }
//
// Returns 401 if not authenticated or 500 for server errors.
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	// Get user email from context (set by authMiddleware)
	userEmail, ok := r.Context().Value("user_email").(string)
	if !ok || userEmail == "" {
		http.Error(w, "user not found in context", http.StatusInternalServerError)
		return
	}

	// Connect to database
	ctx := r.Context()
	conn, err := s.pgConnector.Connect(ctx)
	if err != nil {
		log.Error("dashboard me: database connection failed", "error", err)
		http.Error(w, "database connection failed", http.StatusInternalServerError)
		return
	}
	defer conn.Close(ctx)

	// Query user from admin.users table
	var userID string
	query := `SELECT id FROM admin.users WHERE email = $1`
	err = conn.QueryRow(ctx, query, userEmail).Scan(&userID)
	if err != nil {
		if err == pgx.ErrNoRows {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}
		log.Error("dashboard me: user query failed", "error", err)
		http.Error(w, "database query failed", http.StatusInternalServerError)
		return
	}

	// Return user info
	response := meResponse{
		ID:    userID,
		Email: userEmail,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleStatus returns the current server status.
//
// GET /api/status
//
// Requires valid JWT token in Authorization header.
//
// Response (200 OK):
//   {
//     "status": "healthy",
//     "timestamp": "2026-01-29T12:00:00Z",
//     "uptime": "2h30m45s",
//     "version": "dev"
//   }
//
// Returns 401 if not authenticated.
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	// Get server uptime (placeholder - would need start time in Server struct)
	uptime := time.Since(time.Now().Add(-time.Hour)) // Placeholder

	response := statusResponse{
		Status:    "healthy",
		Timestamp: time.Now(),
		Uptime:    uptime.String(),
		Version:   "dev", // Would be injected from build vars
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleListTables lists all tables in the database.
//
// GET /api/tables
//
// Requires valid JWT token in Authorization header.
//
// Returns a list of tables with metadata including row counts
// and sizes where available.
//
// Response (200 OK):
//   {
//     "tables": [
//       {
//         "name": "users",
//         "schema": "public",
//         "rows": 42,
//         "size_bytes": "8192"
//       }
//     ]
//   }
//
// Returns 401 if not authenticated or 500 for server errors.
func (s *Server) handleListTables(w http.ResponseWriter, r *http.Request) {
	// Connect to database
	ctx := r.Context()
	conn, err := s.pgConnector.Connect(ctx)
	if err != nil {
		log.Error("dashboard tables: database connection failed", "error", err)
		http.Error(w, "database connection failed", http.StatusInternalServerError)
		return
	}
	defer conn.Close(ctx)

	// Query all tables in public schema
	query := `
		SELECT
			table_name,
			table_schema
		FROM information_schema.tables
		WHERE table_schema IN ('public', 'admin')
		AND table_type = 'BASE TABLE'
		ORDER BY table_schema, table_name
	`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		log.Error("dashboard tables: query failed", "error", err)
		http.Error(w, "database query failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Collect table names
	var tables []tableInfo
	var tableNames []struct {
		Name   string
		Schema string
	}

	for rows.Next() {
		var t struct {
			Name   string
			Schema string
		}
		if err := rows.Scan(&t.Name, &t.Schema); err != nil {
			log.Error("dashboard tables: row scan failed", "error", err)
			continue
		}
		tableNames = append(tableNames, t)
	}

	// Get row counts and sizes for each table
	for _, t := range tableNames {
		tableInfo := tableInfo{
			Name:   t.Name,
			Schema: t.Schema,
		}

		// Get row count
		var rowCount sql.NullInt64
		countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s.%s", t.Schema, t.Name)
		err = conn.QueryRow(ctx, countQuery).Scan(&rowCount)
		if err == nil && rowCount.Valid {
			tableInfo.Rows = rowCount.Int64
		}

		// Get table size (optional, may fail due to permissions)
		var size sql.NullString
		sizeQuery := `
			SELECT pg_size_pretty(pg_total_relation_size(quote_ident($1) || '.' || quote_ident($2)))
		`
		err = conn.QueryRow(ctx, sizeQuery, t.Schema, t.Name).Scan(&size)
		if err == nil && size.Valid {
			tableInfo.SizeBytes = size.String
		}

		tables = append(tables, tableInfo)
	}

	response := tablesResponse{
		Tables: tables,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleStatic serves static files for the dashboard web UI.
//
// GET /*
//
// Serves the embedded React dashboard files using http.FileServer.
// For requests to the root path, serves index.html to support client-side routing.
//
// The dashboard is embedded in the binary using Go's embed.FS directive,
// which packages the entire dashboard/dist directory at compile time.
func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	// Serve static files using the embedded filesystem
	fileServer := http.FileServer(s.staticFS)

	// If the path is root or a directory, serve index.html
	// This supports client-side routing in React
	if r.URL.Path == "/" || r.URL.Path == "" {
		r.URL.Path = "/index.html"
	}

	fileServer.ServeHTTP(w, r)
}
