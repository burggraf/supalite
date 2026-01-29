package auth

import (
	"context"
	"testing"
	"time"

	"github.com/markb/supalite/internal/pg"
)

func TestGoTrueServer_Start(t *testing.T) {
	// Skip test if GoTrue is not installed
	if _, err := findGoTrueBinary(); err != nil {
		t.Skip("GoTrue binary not found, skipping test. Install with: go install github.com/supabase/auth/cmd/gotrue@latest")
	}
	// Start embedded Postgres
	pgDB := pg.NewEmbeddedDatabase(pg.Config{
		Port:        15434,
		Username:    "gotrue",
		Password:    "gotrue",
		Database:    "auth_test",
		RuntimePath: "/tmp/supalite-test-auth",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := pgDB.Start(ctx); err != nil {
		t.Fatalf("Failed to start postgres: %v", err)
	}
	defer pgDB.Stop()

	// Create the auth schema required by GoTrue
	conn, err := pgDB.Connect(ctx)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer conn.Close(ctx)

	_, err = conn.Exec(ctx, `
		CREATE SCHEMA IF NOT EXISTS auth;
		CREATE SCHEMA IF NOT EXISTS storage;
		CREATE SCHEMA IF NOT EXISTS public;

		-- Create postgres role that GoTrue expects
		CREATE ROLE postgres WITH LOGIN SUPERUSER;
		GRANT ALL PRIVILEGES ON DATABASE auth_test TO postgres;
		GRANT ALL ON SCHEMA auth TO postgres;
		GRANT ALL ON SCHEMA storage TO postgres;
		GRANT ALL ON SCHEMA public TO postgres;
	`)
	if err != nil {
		t.Fatalf("Failed to create schemas and roles: %v", err)
	}

	// Create and start GoTrue server
	authSrv := NewServer(Config{
		ConnString:   pgDB.ConnectionString(),
		Port:         9999,
		JWTSecret:    "test-secret",
		SiteURL:      "http://localhost:9999",
	})

	if err := authSrv.Start(ctx); err != nil {
		t.Fatalf("Failed to start GoTrue: %v", err)
	}
	defer authSrv.Stop()

	// Give server time to start
	time.Sleep(3 * time.Second)

	// Verify server is running
	if !authSrv.IsRunning() {
		t.Error("GoTrue server is not running")
	}
}
