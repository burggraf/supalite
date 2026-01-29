package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// EmailConfig holds email configuration for GoTrue
type EmailConfig struct {
	SMTPHost            string `json:"smtp_host,omitempty"`
	SMTPPort            int    `json:"smtp_port,omitempty"`
	SMTPUser            string `json:"smtp_user,omitempty"`
	SMTPPass            string `json:"smtp_pass,omitempty"`
	SMTPAdminEmail      string `json:"smtp_admin_email,omitempty"`
	MailerAutoconfirm   bool   `json:"mailer_autoconfirm,omitempty"`
	MailerURLPathsInvite     string `json:"mailer_urlpaths_invite,omitempty"`
	MailerURLPathsConfirmation string `json:"mailer_urlpaths_confirmation,omitempty"`
	MailerURLPathsRecovery    string `json:"mailer_urlpaths_recovery,omitempty"`
	MailerURLPathsEmailChange string `json:"mailer_urlpaths_email_change,omitempty"`

	// Capture mode configuration
	CaptureMode bool `json:"capture_mode,omitempty"`
	CapturePort int  `json:"capture_port,omitempty"`
}

// Config holds the complete Supalite configuration
type Config struct {
	// Server settings
	Host     string `json:"host,omitempty"`
	Port     int    `json:"port,omitempty"`
	DataDir  string `json:"data_dir,omitempty"`
	SiteURL  string `json:"site_url,omitempty"`

	// PostgreSQL settings
	PGPort     uint16 `json:"pg_port,omitempty"`
	PGUsername string `json:"pg_username,omitempty"`
	PGPassword string `json:"pg_password,omitempty"`
	PGDatabase string `json:"pg_database,omitempty"`

	// JWT settings
	JWTSecret      string `json:"jwt_secret,omitempty"`
	AnonKey        string `json:"anon_key,omitempty"`
	ServiceRoleKey string `json:"service_role_key,omitempty"`

	// Email settings (for GoTrue)
	Email *EmailConfig `json:"email,omitempty"`
}

// Load loads configuration from supalite.json (if exists) with fallback to environment variables
// The JSON file takes precedence over environment variables for any fields that are set
func Load() (*Config, error) {
	cfg := &Config{}

	// Try to load from supalite.json
	if data, err := os.ReadFile("supalite.json"); err == nil {
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("failed to parse supalite.json: %w", err)
		}
	} else if !os.IsNotExist(err) {
		// File exists but failed to read
		return nil, fmt.Errorf("failed to read supalite.json: %w", err)
	}

	// Apply environment variable fallbacks for any unset values
	applyEnvFallbacks(cfg)

	// Set defaults for values that are still empty
	setDefaults(cfg)

	return cfg, nil
}

// applyEnvFallbacks applies environment variable values to any unset config fields
func applyEnvFallbacks(cfg *Config) {
	// Server settings
	if cfg.Host == "" {
		cfg.Host = getEnv("SUPALITE_HOST", "")
	}
	if cfg.Port == 0 {
		cfg.Port = getEnvInt("SUPALITE_PORT", 0)
	}
	if cfg.DataDir == "" {
		cfg.DataDir = getEnv("SUPALITE_DATA_DIR", "")
	}
	if cfg.SiteURL == "" {
		cfg.SiteURL = getEnv("SUPALITE_SITE_URL", "")
	}

	// PostgreSQL settings
	if cfg.PGPort == 0 {
		cfg.PGPort = uint16(getEnvInt("SUPALITE_PG_PORT", 0))
	}
	if cfg.PGUsername == "" {
		cfg.PGUsername = getEnv("SUPALITE_PG_USERNAME", "")
	}
	if cfg.PGPassword == "" {
		cfg.PGPassword = getEnv("SUPALITE_PG_PASSWORD", "")
	}
	if cfg.PGDatabase == "" {
		cfg.PGDatabase = getEnv("SUPALITE_PG_DATABASE", "")
	}

	// JWT settings
	if cfg.JWTSecret == "" {
		cfg.JWTSecret = getEnv("SUPALITE_JWT_SECRET", "")
	}
	if cfg.AnonKey == "" {
		cfg.AnonKey = getEnv("SUPALITE_ANON_KEY", "")
	}
	if cfg.ServiceRoleKey == "" {
		cfg.ServiceRoleKey = getEnv("SUPALITE_SERVICE_ROLE_KEY", "")
	}

	// Email settings - initialize Email config if needed
	if cfg.Email == nil {
		cfg.Email = &EmailConfig{}
	}

	if cfg.Email.SMTPHost == "" {
		cfg.Email.SMTPHost = getEnv("SUPALITE_SMTP_HOST", "")
	}
	if cfg.Email.SMTPPort == 0 {
		cfg.Email.SMTPPort = getEnvInt("SUPALITE_SMTP_PORT", 0)
	}
	if cfg.Email.SMTPUser == "" {
		cfg.Email.SMTPUser = getEnv("SUPALITE_SMTP_USER", "")
	}
	if cfg.Email.SMTPPass == "" {
		cfg.Email.SMTPPass = getEnv("SUPALITE_SMTP_PASS", "")
	}
	if cfg.Email.SMTPAdminEmail == "" {
		cfg.Email.SMTPAdminEmail = getEnv("SUPALITE_SMTP_ADMIN_EMAIL", "")
	}
	if cfg.Email.MailerURLPathsInvite == "" {
		cfg.Email.MailerURLPathsInvite = getEnv("SUPALITE_MAILER_URLPATHS_INVITE", "")
	}
	if cfg.Email.MailerURLPathsConfirmation == "" {
		cfg.Email.MailerURLPathsConfirmation = getEnv("SUPALITE_MAILER_URLPATHS_CONFIRMATION", "")
	}
	if cfg.Email.MailerURLPathsRecovery == "" {
		cfg.Email.MailerURLPathsRecovery = getEnv("SUPALITE_MAILER_URLPATHS_RECOVERY", "")
	}
	if cfg.Email.MailerURLPathsEmailChange == "" {
		cfg.Email.MailerURLPathsEmailChange = getEnv("SUPALITE_MAILER_URLPATHS_EMAIL_CHANGE", "")
	}
	// Autoconfirm is a boolean - check for "true" string
	if !cfg.Email.MailerAutoconfirm {
		cfg.Email.MailerAutoconfirm = strings.ToLower(getEnv("SUPALITE_MAILER_AUTOCONFIRM", "")) == "true"
	}
	// Capture mode settings
	if !cfg.Email.CaptureMode {
		cfg.Email.CaptureMode = strings.ToLower(getEnv("SUPALITE_CAPTURE_MODE", "")) == "true"
	}
	if cfg.Email.CapturePort == 0 {
		cfg.Email.CapturePort = getEnvInt("SUPALITE_CAPTURE_PORT", 0)
	}
}

// setDefaults sets default values for any empty fields
func setDefaults(cfg *Config) {
	if cfg.Host == "" {
		cfg.Host = "0.0.0.0"
	}
	if cfg.Port == 0 {
		cfg.Port = 8080
	}
	if cfg.DataDir == "" {
		cfg.DataDir = "./data"
	}
	if cfg.PGPort == 0 {
		cfg.PGPort = 5432
	}
}

// getEnv gets an environment variable or returns the default value
func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// getEnvInt gets an environment variable as an integer or returns the default value
func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		var intVal int
		if _, err := fmt.Sscanf(val, "%d", &intVal); err == nil {
			return intVal
		}
	}
	return defaultVal
}
