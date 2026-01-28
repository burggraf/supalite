package auth

import "time"

// Config holds the configuration for the GoTrue auth server
type Config struct {
	// ConnString is the PostgreSQL connection string
	ConnString string

	// Port is the port to run the GoTrue server on
	Port int

	// JWTSecret is the secret used to sign JWT tokens
	JWTSecret string

	// SiteURL is the base URL of the application (for callbacks, etc.)
	SiteURL string

	// URI is the base URI for the auth API (default: /auth/v1)
	URI string

	// OperatorToken is the token for admin operations
	OperatorToken string

	// LogLevel is the logging level (debug, info, warn, error)
	LogLevel string

	// DBStartupAttempts is the number of attempts to connect to the database
	DBStartupAttempts int

	// DBStartupDelay is the delay between connection attempts
	DBStartupDelay time.Duration
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() Config {
	return Config{
		Port:              9999,
		URI:               "/auth/v1",
		LogLevel:          "info",
		DBStartupAttempts: 10,
		DBStartupDelay:    2 * time.Second,
	}
}
