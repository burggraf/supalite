package pg

// Config holds the configuration for the embedded PostgreSQL database
type Config struct {
	Port     uint16
	Username string
	Password string
	Database string
	DataDir  string
	Version  string
}
