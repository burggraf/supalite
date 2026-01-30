package dashboard

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
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

// columnInfo represents information about a table column.
type columnInfo struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
	Key      string `json:"key,omitempty"`
}

// tableSchemaResponse represents the response for /api/tables/{name}/schema endpoint.
type tableSchemaResponse struct {
	TableName string       `json:"table_name"`
	Schema    string       `json:"schema"`
	Columns   []columnInfo `json:"columns"`
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
	// Remove leading slash for fs.FS
	requestPath := strings.TrimPrefix(r.URL.Path, "/")

	// For root or client-side routes, serve index.html
	// Check if it's a static asset (has a file extension)
	if requestPath == "" || requestPath == "/" || !strings.Contains(requestPath, ".") {
		// This is a client-side route or root - serve index.html
		requestPath = "index.html"
	}

	log.Info("handleStatic", "original_path", r.URL.Path, "serving", requestPath)

	// Read file from embedded filesystem
	data, err := fs.ReadFile(s.embedFS, requestPath)
	if err != nil {
		log.Error("failed to read file", "path", requestPath, "error", err)
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	// Determine content type
	contentType := "text/html; charset=utf-8"
	if strings.HasSuffix(requestPath, ".js") {
		contentType = "application/javascript; charset=utf-8"
	} else if strings.HasSuffix(requestPath, ".css") {
		contentType = "text/css; charset=utf-8"
	}

	w.Header().Set("Content-Type", contentType)
	w.Write(data)
}

// handleGetTableSchema returns the schema for a specific table.
//
// GET /api/tables/{name}/schema
//
// Requires valid JWT token in Authorization header.
//
// Returns detailed information about the table's columns including:
//   - Column name
//   - Data type
//   - Nullable status
//   - Key information (PRIMARY KEY, FOREIGN KEY, etc.)
//
// Response (200 OK):
//   {
//     "table_name": "users",
//     "schema": "public",
//     "columns": [
//       {
//         "name": "id",
//         "type": "uuid",
//         "nullable": false,
//         "key": "PRIMARY KEY"
//       }
//     ]
//   }
//
// Returns 401 if not authenticated, 404 if table not found,
// or 500 for server errors.
func (s *Server) handleGetTableSchema(w http.ResponseWriter, r *http.Request) {
	// Extract table name from URL path
	// URL format: /api/tables/{tableName}/schema
	tableName := chi.URLParam(r, "tableName")
	if tableName == "" {
		http.Error(w, "table name is required", http.StatusBadRequest)
		return
	}

	// Connect to database
	ctx := r.Context()
	conn, err := s.pgConnector.Connect(ctx)
	if err != nil {
		log.Error("dashboard table schema: database connection failed", "error", err)
		http.Error(w, "database connection failed", http.StatusInternalServerError)
		return
	}
	defer conn.Close(ctx)

	// Query column information from information_schema
	query := `
		SELECT
			column_name,
			data_type,
			is_nullable,
			column_default
		FROM information_schema.columns
		WHERE table_name = $1
		AND table_schema IN ('public', 'admin', 'auth', 'storage')
		ORDER BY ordinal_position
	`

	rows, err := conn.Query(ctx, query, tableName)
	if err != nil {
		log.Error("dashboard table schema: query failed", "error", err)
		http.Error(w, "database query failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var columns []columnInfo
	var schemaName string

	for rows.Next() {
		var col columnInfo
		var nullable string
		var defaultValue sql.NullString

		if err := rows.Scan(&col.Name, &col.Type, &nullable, &defaultValue); err != nil {
			log.Error("dashboard table schema: row scan failed", "error", err)
			continue
		}

		col.Nullable = (nullable == "YES")

		// Set key information (simplified - would need additional queries for full key info)
		if col.Name == "id" {
			col.Key = "PRIMARY KEY"
		}

		columns = append(columns, col)
	}

	// Get schema name by checking which schema has this table
	schemaQuery := `
		SELECT table_schema
		FROM information_schema.tables
		WHERE table_name = $1
		AND table_schema IN ('public', 'admin', 'auth', 'storage')
		LIMIT 1
	`
	err = conn.QueryRow(ctx, schemaQuery, tableName).Scan(&schemaName)
	if err != nil {
		log.Error("dashboard table schema: schema lookup failed", "error", err)
		schemaName = "public" // fallback
	}

	// Check if we found any columns
	if len(columns) == 0 {
		http.Error(w, "table not found", http.StatusNotFound)
		return
	}

	response := tableSchemaResponse{
		TableName: tableName,
		Schema:    schemaName,
		Columns:   columns,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
