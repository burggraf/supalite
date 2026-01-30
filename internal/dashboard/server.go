package dashboard

import (
	"embed"
	"io/fs"
	"net/http"
	"context"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/markb/supalite/internal/log"
)

//go:embed dist
var dashboardFS embed.FS

// Server represents the dashboard HTTP server.
//
// The server provides JWT-authenticated endpoints for dashboard functionality
// and serves static files for the web UI.
type Server struct {
	router       *chi.Mux
	jwtManager   *JWTManager
	pgConnector  PostgresConnector
	staticFS     http.FileSystem  // HTTP-compatible filesystem
	embedFS      fs.FS            // Original embedded filesystem for fs.ReadFile
}

// PostgresConnector defines the interface for connecting to PostgreSQL.
//
// This interface allows the dashboard to connect to the database without
// depending on the specific PostgreSQL implementation.
type PostgresConnector interface {
	Connect(ctx context.Context) (*pgx.Conn, error)
}

// Config holds the configuration for the dashboard server.
//
// Configuration includes the JWT secret for authentication and the
// PostgreSQL connector for database access.
type Config struct {
	JWTSecret  string             // Secret key for JWT signing (32+ bytes recommended)
	PGDatabase PostgresConnector  // Database connector for admin operations
}

// NewServer creates a new dashboard server.
//
// Parameters:
//   - cfg: Configuration including JWT secret and database connector
//
// Returns a configured server ready to start.
//
// Example:
//	server := dashboard.NewServer(dashboard.Config{
//	    JWTSecret: "your-secret-key-min-32-bytes",
//	    PGDatabase: pgDatabase,
//	})
func NewServer(cfg Config) *Server {
	jwtManager := NewJWTManager([]byte(cfg.JWTSecret))

	// Create sub FS that removes the dist prefix for serving
	distFS, err := fs.Sub(dashboardFS, "dist")
	if err != nil {
		log.Warn("failed to create embedded filesystem for dashboard", "error", err)
		// This should not happen in production builds
		distFS = dashboardFS
	}

	router := chi.NewRouter()
	s := &Server{
		router:      router,
		jwtManager:  jwtManager,
		pgConnector: cfg.PGDatabase,
		staticFS:    http.FS(distFS),
		embedFS:     distFS,  // Store the original fs.FS for fs.ReadFile
	}

	// Setup routes immediately after creating the server
	s.setupRoutes()

	return s
}

// setupRoutes configures all HTTP routes for the dashboard.
//
// Routes:
//   - POST /api/login - Public endpoint for admin login
//   - GET  /api/me - Protected: returns current user info
//   - GET  /api/status - Protected: returns server status
//   - GET  /api/tables - Protected: lists database tables
//   - GET  /api/tables/{name}/schema - Protected: returns table schema
//   - /* - Static file serving
func (s *Server) setupRoutes() {
	// Public routes
	s.router.Post("/api/login", s.handleLogin)

	// Protected routes (require authentication)
	s.router.Group(func(r chi.Router) {
		r.Use(s.authMiddleware)
		r.Get("/api/me", s.handleMe)
		r.Get("/api/status", s.handleStatus)
		r.Get("/api/tables", s.handleListTables)
		r.Get("/api/tables/{tableName}/schema", s.handleGetTableSchema)
	})

	// Static file serving - handle both root and all other paths
	// Use a middleware to catch all remaining requests
	s.router.HandleFunc("/*", func(w http.ResponseWriter, r *http.Request) {
		log.Info("dashboard static request", "path", r.URL.Path)
		s.handleStatic(w, r)
	})
}

// authMiddleware validates JWT tokens for protected routes.
//
// This middleware checks for the Authorization header in the format:
//   Authorization: Bearer <token>
//
// If the token is valid, the request proceeds to the next handler.
// If invalid or missing, returns 401 Unauthorized.
//
// The middleware extracts the token and verifies it using the JWT manager.
// Valid tokens include the user's email in the claims.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "missing authorization header", http.StatusUnauthorized)
			return
		}

		// Check Bearer format
		const bearerPrefix = "Bearer "
		if len(authHeader) < len(bearerPrefix) || authHeader[:len(bearerPrefix)] != bearerPrefix {
			http.Error(w, "invalid authorization header format", http.StatusUnauthorized)
			return
		}

		// Extract token
		tokenString := authHeader[len(bearerPrefix):]

		// Verify token
		claims, err := s.jwtManager.VerifyToken(tokenString)
		if err != nil {
			log.Warn("dashboard auth failed", "error", err)
			http.Error(w, "invalid or expired token", http.StatusUnauthorized)
			return
		}

		// Add user info to request context for handlers to use
		ctx := context.WithValue(r.Context(), "user_email", claims.Email)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Handler returns the HTTP handler for the dashboard server.
//
// Returns the Chi router for use with http.ServeMux or direct mounting.
// Routes are already initialized in NewServer.
func (s *Server) Handler() http.Handler {
	return s.router
}
