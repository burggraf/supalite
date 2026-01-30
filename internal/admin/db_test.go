package admin

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/markb/supalite/internal/pg"
)

// TestConnectToDatabase_WhenRunning tests that ConnectToDatabase
// can connect to an already-running database instead of failing.
func TestConnectToDatabase_WhenRunning(t *testing.T) {
	// Start a test database
	dbCfg := pg.Config{
		Port:        15432,
		Username:    "test",
		Password:    "test",
		Database:    "testdb",
		RuntimePath: "/tmp/supalite-test-admin-db",
	}

	embeddedDB := pg.NewEmbeddedDatabase(dbCfg)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := embeddedDB.Start(ctx); err != nil {
		t.Fatalf("Failed to start test database: %v", err)
	}
	defer embeddedDB.Stop()

	// Create admin schema and users table for testing
	conn, err := pgx.Connect(ctx, "postgres://test:test@localhost:15432/testdb")
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}
	defer conn.Close(ctx)

	// Create schema and table
	_, err = conn.Exec(ctx, "CREATE SCHEMA IF NOT EXISTS admin")
	if err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}
	_, err = conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS admin.users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			email TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create users table: %v", err)
	}
	conn.Close(ctx)

	// Now test that ConnectToDatabase can connect to the running database
	conn2, cleanup, err := ConnectToDatabase(15432, "test", "test", "testdb", dbCfg.RuntimePath)
	if err != nil {
		t.Fatalf("ConnectToDatabase() failed: %v", err)
	}
	defer cleanup()

	// Verify we can query the database
	var count int
	err = conn2.QueryRow(ctx, "SELECT COUNT(*) FROM admin.users").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query database: %v", err)
	}

	// Should succeed without starting a new database
}

// TestConnectToDatabase_WhenNotRunning tests that ConnectToDatabase
// starts a new database when one is not already running.
func TestConnectToDatabase_WhenNotRunning(t *testing.T) {
	// Use a port that should be free
	runtimePath := "/tmp/supalite-test-admin-db-new"

	conn, cleanup, err := ConnectToDatabase(15433, "test", "test", "testdb", runtimePath)
	if err != nil {
		t.Fatalf("ConnectToDatabase() failed: %v", err)
	}
	defer cleanup()

	// Verify we can query the database
	var result int
	ctx := context.Background()
	err = conn.QueryRow(ctx, "SELECT 1").Scan(&result)
	if err != nil {
		t.Fatalf("Failed to query database: %v", err)
	}
	if result != 1 {
		t.Errorf("Expected 1, got %d", result)
	}

	// Should have started a new database
}
