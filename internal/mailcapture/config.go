package mailcapture

import (
	"github.com/jackc/pgx/v5"
	"github.com/markb/supalite/internal/pg"
)

// Database interface for storing captured emails
type Database interface {
	Connect(ctx any) (*pgx.Conn, error)
}

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
