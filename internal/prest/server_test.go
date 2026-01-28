package prest

import (
	"context"
	"testing"
	"time"

	"github.com/markb/supalite/internal/pg"
)

func TestPRESTServer_Start(t *testing.T) {
	// Start embedded Postgres
	pgDB := pg.NewEmbeddedDatabase(pg.Config{
		Port:     15433,
		Username: "test",
		Password: "test",
		Database: "testdb",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := pgDB.Start(ctx); err != nil {
		t.Fatalf("Failed to start postgres: %v", err)
	}
	defer pgDB.Stop()

	// Create and start pREST server
	prestSrv := NewServer(Config{
		ConnString: pgDB.ConnectionString(),
		Port:       3010,
	})

	if err := prestSrv.Start(ctx); err != nil {
		t.Fatalf("Failed to start pREST: %v", err)
	}
	defer prestSrv.Stop()

	// Give server time to start
	time.Sleep(2 * time.Second)

	// Verify server is running
	if !prestSrv.IsRunning() {
		t.Error("Server is not running after Start()")
	}
}
