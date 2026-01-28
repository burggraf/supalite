package prest

import (
	"fmt"
	"strconv"
)

// Config holds the configuration for the pREST server
type Config struct {
	ConnString string // PostgreSQL connection string
	Port       int    // Port to run pREST on (default: 3000)
}

// DefaultConfig returns default pREST configuration
func DefaultConfig(connString string) Config {
	return Config{
		ConnString: connString,
		Port:       3000,
	}
}

// parsePortInt parses a port string to an integer
func parsePortInt(portStr string) (int, error) {
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, fmt.Errorf("invalid port number: %s", portStr)
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("port out of range: %d", port)
	}
	return port, nil
}
