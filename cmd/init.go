package cmd

import (
	"context"
	"fmt"
	"time"

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

		fmt.Printf("Database initialized successfully!\n")
		fmt.Printf("Data directory: %s\n", initConfig.dbPath)
		fmt.Printf("Connection: postgres://%s:****@localhost:%d/%s\n",
			initConfig.username, initConfig.port, initConfig.database)

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

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().StringVar(&initConfig.dbPath, "db", "./data", "Data directory for PostgreSQL")
	initCmd.Flags().Uint16Var(&initConfig.port, "port", 5432, "PostgreSQL port")
	initCmd.Flags().StringVar(&initConfig.username, "username", "postgres", "Database username")
	initCmd.Flags().StringVar(&initConfig.password, "password", "postgres", "Database password")
	initCmd.Flags().StringVar(&initConfig.database, "database", "postgres", "Database name")
	initCmd.Flags().StringVar(&initConfig.pgVersion, "pg-version", "16.9.0", "PostgreSQL version (e.g., 16.9.0, 15.8.0, 14.13.0)")
}
