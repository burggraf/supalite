package dashboard

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/markb/supalite/internal/log"
)

// Server represents the dashboard HTTP server.
//
// The server provides JWT-authenticated endpoints for dashboard functionality
// and serves static files for the web UI.
type Server struct {
	router       *chi.Mux
	jwtManager   *JWTManager
	pgConnector  PostgresConnector
	staticFS     http.FileSystem
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

	return &Server{
		router:      chi.NewRouter(),
		jwtManager:  jwtManager,
		pgConnector: cfg.PGDatabase,
		staticFS:    http.Dir("dashboard"), // Placeholder for static files
	}
}

// setupRoutes configures all HTTP routes for the dashboard.
//
// Routes:
//   - POST /api/login - Public endpoint for admin login
//   - GET  /api/me - Protected: returns current user info
//   - GET  /api/status - Protected: returns server status
//   - GET  /api/tables - Protected: lists database tables
//   - /* - Static file serving (placeholder)
func (s *Server) setupRoutes() {
	// Public routes
	s.router.Post("/api/login", s.handleLogin)

	// Protected routes (require authentication)
	s.router.Group(func(r chi.Router) {
		r.Use(s.authMiddleware)
		r.Get("/api/me", s.handleMe)
		r.Get("/api/status", s.handleStatus)
		r.Get("/api/tables", s.handleListTables)
	})

	// Static file serving (placeholder for now)
	s.router.Get("/*", s.handleStatic)
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
// This method initializes routes if not already done and returns
// the Chi router for use with http.ServeMux or direct mounting.
//
// Returns the main HTTP handler for the dashboard.
func (s *Server) Handler() http.Handler {
	if s.router == nil {
		s.router = chi.NewRouter()
		s.setupRoutes()
	}
	return s.router
}
