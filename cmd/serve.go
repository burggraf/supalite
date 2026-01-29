package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/markb/supalite/internal/auth"
	"github.com/markb/supalite/internal/config"
	"github.com/markb/supalite/internal/log"
	"github.com/markb/supalite/internal/server"
	"github.com/spf13/cobra"
)

var (
	// Config file path
	configFile string

	// Flags that override config file/env vars
	flagHost           string
	flagPort           int
	flagPgPort         uint16
	flagDataDir        string
	flagJwtSecret      string
	flagSiteURL        string
	flagPgUsername     string
	flagPgPassword     string
	flagPgDatabase     string
	flagAnonKey        string
	flagServiceRoleKey string

	// Email flags
	flagSmtpHost            string
	flagSmtpPort            int
	flagSmtpUser            string
	flagSmtpPass            string
	flagSmtpAdminEmail      string
	flagMailerAutoconfirm   bool
	flagMailerUrlpathsInvite       string
	flagMailerUrlpathsConfirmation string
	flagMailerUrlpathsRecovery     string
	flagMailerUrlpathsEmailChange  string

	// Email capture mode flags
	flagCaptureMode bool
	flagCapturePort int
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Supalite server",
	Long: `Start the Supalite server with embedded PostgreSQL, pREST, and GoTrue auth.

The server orchestrates all components and provides a unified API endpoint.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Set log level
		log.SetLevel(log.LevelInfo)

		// Load configuration (file + env vars)
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Apply flag overrides (flags take precedence over file and env vars)
		applyFlagOverrides(cfg)

		// Set default site URL if not provided
		if cfg.SiteURL == "" {
			cfg.SiteURL = fmt.Sprintf("http://localhost:%d", cfg.Port)
		}

		// Convert config.Email to auth.EmailConfig
		var emailCfg *auth.EmailConfig
		if cfg.Email != nil && hasEmailConfig(cfg.Email) {
			emailCfg = &auth.EmailConfig{
				SMTPHost:            cfg.Email.SMTPHost,
				SMTPPort:            cfg.Email.SMTPPort,
				SMTPUser:            cfg.Email.SMTPUser,
				SMTPPass:            cfg.Email.SMTPPass,
				AdminEmail:          cfg.Email.SMTPAdminEmail,
				Autoconfirm:         cfg.Email.MailerAutoconfirm,
				URLPathsInvite:      cfg.Email.MailerURLPathsInvite,
				URLPathsConfirmation: cfg.Email.MailerURLPathsConfirmation,
				URLPathsRecovery:    cfg.Email.MailerURLPathsRecovery,
				URLPathsEmailChange: cfg.Email.MailerURLPathsEmailChange,
				CaptureMode:         cfg.Email.CaptureMode,
				CapturePort:         cfg.Email.CapturePort,
			}
		}

		// Create server configuration
		srvCfg := server.Config{
			Host:           cfg.Host,
			Port:           cfg.Port,
			PGPort:         cfg.PGPort,
			DataDir:        cfg.DataDir,
			JWTSecret:      cfg.JWTSecret,
			SiteURL:        cfg.SiteURL,
			PGUsername:     cfg.PGUsername,
			PGPassword:     cfg.PGPassword,
			PGDatabase:     cfg.PGDatabase,
			AnonKey:        cfg.AnonKey,
			ServiceRoleKey: cfg.ServiceRoleKey,
			Email:          emailCfg,
		}

		// Create and start server
		srv := server.New(srvCfg)

		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		return srv.Start(ctx)
	},
}

// applyFlagOverrides applies command-line flag values to the config
// Flags take precedence over both file and environment variable values
func applyFlagOverrides(cfg *config.Config) {
	if flagHost != "" {
		cfg.Host = flagHost
	}
	if flagPort != 0 {
		cfg.Port = flagPort
	}
	if flagPgPort != 0 {
		cfg.PGPort = flagPgPort
	}
	if flagDataDir != "" {
		cfg.DataDir = flagDataDir
	}
	if flagJwtSecret != "" {
		cfg.JWTSecret = flagJwtSecret
	}
	if flagSiteURL != "" {
		cfg.SiteURL = flagSiteURL
	}
	if flagPgUsername != "" {
		cfg.PGUsername = flagPgUsername
	}
	if flagPgPassword != "" {
		cfg.PGPassword = flagPgPassword
	}
	if flagPgDatabase != "" {
		cfg.PGDatabase = flagPgDatabase
	}
	if flagAnonKey != "" {
		cfg.AnonKey = flagAnonKey
	}
	if flagServiceRoleKey != "" {
		cfg.ServiceRoleKey = flagServiceRoleKey
	}

	// Email overrides
	if cfg.Email == nil {
		cfg.Email = &config.EmailConfig{}
	}
	if flagSmtpHost != "" {
		cfg.Email.SMTPHost = flagSmtpHost
	}
	if flagSmtpPort != 0 {
		cfg.Email.SMTPPort = flagSmtpPort
	}
	if flagSmtpUser != "" {
		cfg.Email.SMTPUser = flagSmtpUser
	}
	if flagSmtpPass != "" {
		cfg.Email.SMTPPass = flagSmtpPass
	}
	if flagSmtpAdminEmail != "" {
		cfg.Email.SMTPAdminEmail = flagSmtpAdminEmail
	}
	if flagMailerAutoconfirm {
		cfg.Email.MailerAutoconfirm = true
	}
	if flagMailerUrlpathsInvite != "" {
		cfg.Email.MailerURLPathsInvite = flagMailerUrlpathsInvite
	}
	if flagMailerUrlpathsConfirmation != "" {
		cfg.Email.MailerURLPathsConfirmation = flagMailerUrlpathsConfirmation
	}
	if flagMailerUrlpathsRecovery != "" {
		cfg.Email.MailerURLPathsRecovery = flagMailerUrlpathsRecovery
	}
	if flagMailerUrlpathsEmailChange != "" {
		cfg.Email.MailerURLPathsEmailChange = flagMailerUrlpathsEmailChange
	}

	// Capture mode overrides
	if flagCaptureMode {
		cfg.Email.CaptureMode = true
	}
	if flagCapturePort != 0 {
		cfg.Email.CapturePort = flagCapturePort
	}
}

// hasEmailConfig checks if any email configuration is set
func hasEmailConfig(e *config.EmailConfig) bool {
	return e.SMTPHost != "" || e.SMTPPort != 0 || e.SMTPUser != "" ||
		e.SMTPPass != "" || e.SMTPAdminEmail != "" ||
		e.MailerURLPathsInvite != "" || e.MailerURLPathsConfirmation != "" ||
		e.MailerURLPathsRecovery != "" || e.MailerURLPathsEmailChange != "" ||
		e.MailerAutoconfirm || e.CaptureMode
}

func init() {
	rootCmd.AddCommand(serveCmd)

	// Server configuration
	serveCmd.Flags().StringVar(&flagHost, "host", "", "Host to bind to (overrides config file and env vars)")
	serveCmd.Flags().IntVar(&flagPort, "port", 0, "Port to listen on (overrides config file and env vars)")

	// Database configuration
	serveCmd.Flags().StringVar(&flagDataDir, "data-dir", "", "Data directory for PostgreSQL (overrides config file and env vars)")
	serveCmd.Flags().Uint16Var(&flagPgPort, "pg-port", 0, "PostgreSQL port (overrides config file and env vars)")
	serveCmd.Flags().StringVar(&flagPgUsername, "pg-username", "", "PostgreSQL username (overrides config file and env vars)")
	serveCmd.Flags().StringVar(&flagPgPassword, "pg-password", "", "PostgreSQL password (overrides config file and env vars)")
	serveCmd.Flags().StringVar(&flagPgDatabase, "pg-database", "", "PostgreSQL database name (overrides config file and env vars)")

	// Auth configuration
	serveCmd.Flags().StringVar(&flagJwtSecret, "jwt-secret", "", "JWT secret for signing tokens - legacy mode (overrides config file and env vars)")
	serveCmd.Flags().StringVar(&flagSiteURL, "site-url", "", "Site URL for auth callbacks (overrides config file and env vars)")
	serveCmd.Flags().StringVar(&flagAnonKey, "anon-key", "", "Anonymous/public key (overrides config file and env vars)")
	serveCmd.Flags().StringVar(&flagServiceRoleKey, "service-role-key", "", "Service role key (overrides config file and env vars)")

	// Email configuration (all optional - overrides config file and env vars)
	serveCmd.Flags().StringVar(&flagSmtpHost, "smtp-host", "", "SMTP server hostname")
	serveCmd.Flags().IntVar(&flagSmtpPort, "smtp-port", 0, "SMTP server port")
	serveCmd.Flags().StringVar(&flagSmtpUser, "smtp-user", "", "SMTP username")
	serveCmd.Flags().StringVar(&flagSmtpPass, "smtp-pass", "", "SMTP password")
	serveCmd.Flags().StringVar(&flagSmtpAdminEmail, "smtp-admin-email", "", "Admin email for sending password reset emails")
	serveCmd.Flags().BoolVar(&flagMailerAutoconfirm, "mailer-autoconfirm", false, "Skip email confirmation for new users")
	serveCmd.Flags().StringVar(&flagMailerUrlpathsInvite, "mailer-urlpaths-invite", "", "Invite email URL path")
	serveCmd.Flags().StringVar(&flagMailerUrlpathsConfirmation, "mailer-urlpaths-confirmation", "", "Confirmation email URL path")
	serveCmd.Flags().StringVar(&flagMailerUrlpathsRecovery, "mailer-urlpaths-recovery", "", "Recovery email URL path")
	serveCmd.Flags().StringVar(&flagMailerUrlpathsEmailChange, "mailer-urlpaths-email-change", "", "Email change confirmation URL path")

	// Email capture mode (for development)
	serveCmd.Flags().BoolVar(&flagCaptureMode, "capture-mode", false, "Enable email capture mode (captures emails to database instead of sending)")
	serveCmd.Flags().IntVar(&flagCapturePort, "capture-port", 0, "Port for mail capture SMTP server (default: 1025)")
}
