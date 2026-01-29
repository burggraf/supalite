package server

import (
	"context"
	"testing"
	"time"

	"github.com/markb/supalite/internal/pg"
)

func TestCapturedEmailsTableCreation(t *testing.T) {
	// Start embedded postgres
	db := pg.NewEmbeddedDatabase(pg.Config{
		Port:        15433,
		Username:    "test",
		Password:    "test",
		Database:    "testdb",
		RuntimePath: "/tmp/supalite-test-schema",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := db.Start(ctx); err != nil {
		t.Fatalf("Failed to start database: %v", err)
	}
	defer db.Stop()

	// Create server
	srv := &Server{
		pgDatabase: db,
	}

	if err := srv.initSchema(ctx); err != nil {
		t.Fatalf("initSchema() failed: %v", err)
	}

	// Verify table exists
	conn, err := db.Connect(ctx)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close(ctx)

	var exists bool
	err = conn.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables
			WHERE table_schema = 'public'
			AND table_name = 'captured_emails'
		)
	`).Scan(&exists)

	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if !exists {
		t.Error("captured_emails table should exist")
	}

	// Verify table structure
	var columnCount int
	err = conn.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM information_schema.columns
		WHERE table_schema = 'public'
		AND table_name = 'captured_emails'
	`).Scan(&columnCount)

	if err != nil {
		t.Fatalf("Failed to query columns: %v", err)
	}
	if columnCount != 8 {
		t.Errorf("Expected 8 columns in captured_emails, got %d", columnCount)
	}

	// Verify INSERT works (this would fail if RLS was enabled without policies)
	var insertedID string
	err = conn.QueryRow(ctx, `
		INSERT INTO public.captured_emails (from_addr, to_addr, subject, text_body)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, "test@example.com", "recipient@example.com", "Test Subject", "Test body").Scan(&insertedID)

	if err != nil {
		t.Fatalf("Failed to insert test email: %v", err)
	}
	if insertedID == "" {
		t.Error("Expected inserted ID to be returned")
	}

	// Verify we can read the inserted data
	var fromAddr, subject string
	err = conn.QueryRow(ctx, `
		SELECT from_addr, subject
		FROM public.captured_emails
		WHERE id = $1
	`, insertedID).Scan(&fromAddr, &subject)

	if err != nil {
		t.Fatalf("Failed to query inserted email: %v", err)
	}
	if fromAddr != "test@example.com" {
		t.Errorf("Expected from_addr 'test@example.com', got '%s'", fromAddr)
	}
	if subject != "Test Subject" {
		t.Errorf("Expected subject 'Test Subject', got '%s'", subject)
	}

	// Verify indexes exist
	var indexCount int
	err = conn.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM pg_indexes
		WHERE schemaname = 'public'
		AND tablename = 'captured_emails'
	`).Scan(&indexCount)

	if err != nil {
		t.Fatalf("Failed to query indexes: %v", err)
	}
	if indexCount != 3 {
		t.Errorf("Expected 3 indexes on captured_emails (primary key + 2 custom), got %d", indexCount)
	}
}
