package pg

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestEmbeddedDatabase_Start(t *testing.T) {
	db := NewEmbeddedDatabase(Config{
		Port:     15432,
		Username: "test",
		Password: "test",
		Database: "testdb",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := db.Start(ctx); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	defer db.Stop()

	// Verify connection works
	conn, err := pgx.Connect(ctx, "postgres://test:test@localhost:15432/testdb")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close(ctx)

	var result int
	if err := conn.QueryRow(ctx, "SELECT 1").Scan(&result); err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if result != 1 {
		t.Errorf("Expected 1, got %d", result)
	}
}
