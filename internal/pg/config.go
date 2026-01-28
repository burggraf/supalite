package pg

import (
	"fmt"
	"os"
)

// Config holds the configuration for the embedded PostgreSQL database
type Config struct {
	Port        uint16
	Username    string
	Password    string
	Database    string
	DataDir     string
	Version     string
	RuntimePath string // Optional: unique runtime path to avoid conflicts
}

// DefaultConfig returns the default configuration for supalite
func DefaultConfig() Config {
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
		Version:  "16.9.0",
	}
}

func parsePort(s string) uint16 {
	var port int
	if _, err := fmt.Sscanf(s, "%d", &port); err == nil && port > 0 && port < 65536 {
		return uint16(port)
	}
	return 0
}
