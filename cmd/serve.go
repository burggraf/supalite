package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/markb/supalite/internal/log"
	"github.com/markb/supalite/internal/server"
	"github.com/spf13/cobra"
)

var serveConfig struct {
	host      string
	port      int
	pgPort    uint16
	dataDir   string
	jwtSecret string
	siteURL   string
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Supalite server",
	Long: `Start the Supalite server with embedded PostgreSQL, pREST, and GoTrue auth.

The server orchestrates all components and provides a unified API endpoint.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Set log level
		log.SetLevel(log.LevelInfo)

		// Validate configuration
		if serveConfig.jwtSecret == "" {
			// Generate a warning but don't fail
			log.Warn("no JWT secret provided, using default (not recommended for production)")
			serveConfig.jwtSecret = "super-secret-jwt-token-change-in-production"
		}

		if serveConfig.siteURL == "" {
			serveConfig.siteURL = fmt.Sprintf("http://localhost:%d", serveConfig.port)
		}

		// Create server configuration
		cfg := server.Config{
			Host:      serveConfig.host,
			Port:      serveConfig.port,
			PGPort:    serveConfig.pgPort,
			DataDir:   serveConfig.dataDir,
			JWTSecret: serveConfig.jwtSecret,
			SiteURL:   serveConfig.siteURL,
		}

		// Create and start server
		srv := server.New(cfg)

		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		return srv.Start(ctx)
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)

	// Server configuration
	serveCmd.Flags().StringVar(&serveConfig.host, "host", "0.0.0.0", "Host to bind to")
	serveCmd.Flags().IntVar(&serveConfig.port, "port", 8080, "Port to listen on")

	// Database configuration
	serveCmd.Flags().StringVar(&serveConfig.dataDir, "data-dir", "./data", "Data directory for PostgreSQL")
	serveCmd.Flags().Uint16Var(&serveConfig.pgPort, "pg-port", 5432, "PostgreSQL port")

	// Auth configuration
	serveCmd.Flags().StringVar(&serveConfig.jwtSecret, "jwt-secret", os.Getenv("SUPALITE_JWT_SECRET"), "JWT secret for signing tokens")
	serveCmd.Flags().StringVar(&serveConfig.siteURL, "site-url", os.Getenv("SUPALITE_SITE_URL"), "Site URL for auth callbacks")
}
