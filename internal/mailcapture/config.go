package mailcapture

import (
	"github.com/markb/supalite/internal/pg"
)

// Config holds configuration for the mail capture server
type Config struct {
	// Port is the port to listen on for SMTP connections
	Port int

	// Host is the hostname to listen on (default: localhost)
	Host string

	// Database is the PostgreSQL connection for storing emails
	Database *pg.EmbeddedDatabase
}

// DefaultConfig returns configuration with sensible defaults
func DefaultConfig() Config {
	return Config{
		Port: 1025,
		Host: "localhost",
	}
}
