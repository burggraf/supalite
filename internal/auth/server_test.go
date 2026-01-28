package auth

import (
	"context"
	"testing"
	"time"

	"github.com/markb/supalite/internal/pg"
)

func TestGoTrueServer_Start(t *testing.T) {
	// Start embedded Postgres
	pgDB := pg.NewEmbeddedDatabase(pg.Config{
		Port:     15434,
		Username: "gotrue",
		Password: "gotrue",
		Database: "auth_test",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := pgDB.Start(ctx); err != nil {
		t.Fatalf("Failed to start postgres: %v", err)
	}
	defer pgDB.Stop()

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
