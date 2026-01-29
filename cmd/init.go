package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/markb/supalite/internal/config"
	"github.com/markb/supalite/internal/pg"
	"github.com/spf13/cobra"
)

var initConfig struct {
	dbPath    string
	port      uint16
	username  string
	password  string
	database  string
	pgVersion string
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the embedded PostgreSQL database",
	Long:  `Creates and initializes the embedded PostgreSQL database with Supabase schema.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Initializing Supalite database...")

		cfg := pg.Config{
			Port:     initConfig.port,
			Username: initConfig.username,
			Password: initConfig.password,
			Database: initConfig.database,
			DataDir:  initConfig.dbPath,
			Version:  initConfig.pgVersion,
		}
		database := pg.NewEmbeddedDatabase(cfg)

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		if err := database.Start(ctx); err != nil {
			return fmt.Errorf("failed to start database: %w", err)
		}
		defer database.Stop()

		if err := initSchema(ctx, database); err != nil {
			return fmt.Errorf("failed to initialize schema: %w", err)
		}

		// Create supalite.json with capture mode enabled by default
		if err := createDefaultConfig(initConfig.dbPath, initConfig.port, initConfig.username, initConfig.password, initConfig.database); err != nil {
			return fmt.Errorf("failed to create config file: %w", err)
		}

		fmt.Printf("Database initialized successfully!\n")
		fmt.Printf("Data directory: %s\n", initConfig.dbPath)
		fmt.Printf("Connection: postgres://%s:****@localhost:%d/%s\n",
			initConfig.username, initConfig.port, initConfig.database)
		fmt.Printf("\nMail capture mode enabled by default for development.\n")
		fmt.Printf("Configuration written to: %s\n", getConfigPath(initConfig.dbPath))

		return nil
	},
}

func initSchema(ctx context.Context, db *pg.EmbeddedDatabase) error {
	conn, err := db.Connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	_, err = conn.Exec(ctx, `
		CREATE SCHEMA IF NOT EXISTS auth;
		CREATE SCHEMA IF NOT EXISTS storage;
		CREATE SCHEMA IF NOT EXISTS public;
	`)
	return err
}

// createDefaultConfig creates a supalite.json file with capture mode enabled by default
func createDefaultConfig(dataDir string, pgPort uint16, username, password, database string) error {
	configPath := getConfigPath(dataDir)

	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil {
		// Config already exists, don't overwrite
		return nil
	}

	// Create default config with capture mode enabled
	cfg := &config.Config{
		DataDir:  dataDir,
		PGPort:   pgPort,
		PGUsername: username,
		PGPassword: password,
		PGDatabase: database,
		Email: &config.EmailConfig{
			CaptureMode: true,
			CapturePort: 1025,
		},
	}

	// Marshal to JSON with indentation
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// getConfigPath returns the path to supalite.json
func getConfigPath(dataDir string) string {
	// If dataDir is a relative path, use current directory
	if dataDir == "./data" || dataDir == "data" {
		return "supalite.json"
	}
	// Otherwise, place it in the data directory
	return dataDir + "/supalite.json"
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().StringVar(&initConfig.dbPath, "db", "./data", "Data directory for PostgreSQL")
	initCmd.Flags().Uint16Var(&initConfig.port, "port", 5432, "PostgreSQL port")
	initCmd.Flags().StringVar(&initConfig.username, "username", "postgres", "Database username")
	initCmd.Flags().StringVar(&initConfig.password, "password", "postgres", "Database password")
	initCmd.Flags().StringVar(&initConfig.database, "database", "postgres", "Database name")
	initCmd.Flags().StringVar(&initConfig.pgVersion, "pg-version", "16.9.0", "PostgreSQL version (e.g., 16.9.0, 15.8.0, 14.13.0)")
}
