package mailcapture

import (
	"context"
	"net/smtp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/markb/supalite/internal/pg"
)

// createCapturedEmailsTable creates the captured_emails table for testing
func createCapturedEmailsTable(ctx context.Context, conn *pgx.Conn) error {
	_, err := conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS public.captured_emails (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			from_addr TEXT NOT NULL,
			to_addr TEXT NOT NULL,
			subject TEXT,
			text_body TEXT,
			html_body TEXT,
			raw_message BYTEA
		)
	`)
	return err
}

func TestMailCaptureServer_CapturesEmail(t *testing.T) {
	// Start embedded postgres
	db := pg.NewEmbeddedDatabase(pg.Config{
		Port:        15434,
		Username:    "test",
		Password:    "test",
		Database:    "testdb",
		RuntimePath: "/tmp/supalite-test-mailcapture",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := db.Start(ctx); err != nil {
		t.Fatalf("Failed to start database: %v", err)
	}
	defer db.Stop()

	// Create the captured_emails table
	conn, err := db.Connect(ctx)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	if err := createCapturedEmailsTable(ctx, conn); err != nil {
		conn.Close(ctx)
		t.Fatalf("Failed to create table: %v", err)
	}
	conn.Close(ctx)

	// Start mail capture server
	srv, err := NewServer(Config{
		Port:     2525,
		Host:     "localhost",
		Database: db,
	})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	if err := srv.Start(ctx); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer srv.Stop()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Send a test email via SMTP
	err = smtp.SendMail(
		"localhost:2525",
		nil, // no auth
		"sender@example.com",
		[]string{"recipient@example.com"},
		[]byte("Subject: Test Email\r\n\r\nThis is the body."),
	)
	if err != nil {
		t.Fatalf("Failed to send email: %v", err)
	}

	// Give it time to process
	time.Sleep(100 * time.Millisecond)

	// Verify email was captured
	conn, err = db.Connect(ctx)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close(ctx)

	var count int
	err = conn.QueryRow(ctx, "SELECT COUNT(*) FROM public.captured_emails").Scan(&count)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 captured email, got %d", count)
	}

	// Verify email content
	var fromAddr, toAddr, subject, textBody string
	err = conn.QueryRow(ctx, `
		SELECT from_addr, to_addr, subject, text_body
		FROM public.captured_emails LIMIT 1
	`).Scan(&fromAddr, &toAddr, &subject, &textBody)
	if err != nil {
		t.Fatalf("Failed to read captured email: %v", err)
	}

	if fromAddr != "sender@example.com" {
		t.Errorf("from_addr = %q, want sender@example.com", fromAddr)
	}
	if toAddr != "recipient@example.com" {
		t.Errorf("to_addr = %q, want recipient@example.com", toAddr)
	}
	if subject != "Test Email" {
		t.Errorf("subject = %q, want 'Test Email'", subject)
	}
}

func TestMailCaptureServer_CapturesMultipartEmail(t *testing.T) {
	// Start embedded postgres
	db := pg.NewEmbeddedDatabase(pg.Config{
		Port:        15435,
		Username:    "test",
		Password:    "test",
		Database:    "testdb",
		RuntimePath: "/tmp/supalite-test-mailcapture-multipart",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := db.Start(ctx); err != nil {
		t.Fatalf("Failed to start database: %v", err)
	}
	defer db.Stop()

	// Create the captured_emails table
	conn, err := db.Connect(ctx)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	if err := createCapturedEmailsTable(ctx, conn); err != nil {
		conn.Close(ctx)
		t.Fatalf("Failed to create table: %v", err)
	}
	conn.Close(ctx)

	// Start mail capture server
	srv, err := NewServer(Config{
		Port:     2526,
		Host:     "localhost",
		Database: db,
	})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	if err := srv.Start(ctx); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer srv.Stop()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Send a multipart email via SMTP
	multipartMsg := `Subject: Multipart Test
MIME-Version: 1.0
Content-Type: multipart/alternative; boundary="boundary123"

--boundary123
Content-Type: text/plain

This is the plain text body.
--boundary123
Content-Type: text/html

<html><body>This is the <b>HTML</b> body.</body></html>
--boundary123--
`

	err = smtp.SendMail(
		"localhost:2526",
		nil,
		"sender@example.com",
		[]string{"recipient@example.com"},
		[]byte(multipartMsg),
	)
	if err != nil {
		t.Fatalf("Failed to send email: %v", err)
	}

	// Give it time to process
	time.Sleep(100 * time.Millisecond)

	// Verify email was captured
	conn, err = db.Connect(ctx)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close(ctx)

	var textBody, htmlBody string
	err = conn.QueryRow(ctx, `
		SELECT text_body, html_body
		FROM public.captured_emails LIMIT 1
	`).Scan(&textBody, &htmlBody)
	if err != nil {
		t.Fatalf("Failed to read captured email: %v", err)
	}

	// The multipart parser may trim whitespace, so check if the content is present
	if !strings.Contains(textBody, "This is the plain text body") {
		t.Errorf("text_body = %q, want it to contain 'This is the plain text body'", textBody)
	}
	if !strings.Contains(htmlBody, "This is the <b>HTML</b> body") {
		t.Errorf("html_body = %q, want it to contain 'This is the <b>HTML</b> body'", htmlBody)
	}
}

func TestMailCaptureServer_MultipleRecipients(t *testing.T) {
	// Start embedded postgres
	db := pg.NewEmbeddedDatabase(pg.Config{
		Port:        15436,
		Username:    "test",
		Password:    "test",
		Database:    "testdb",
		RuntimePath: "/tmp/supalite-test-mailcapture-multiple",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := db.Start(ctx); err != nil {
		t.Fatalf("Failed to start database: %v", err)
	}
	defer db.Stop()

	// Create the captured_emails table
	conn, err := db.Connect(ctx)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	if err := createCapturedEmailsTable(ctx, conn); err != nil {
		conn.Close(ctx)
		t.Fatalf("Failed to create table: %v", err)
	}
	conn.Close(ctx)

	// Start mail capture server
	srv, err := NewServer(Config{
		Port:     2527,
		Host:     "localhost",
		Database: db,
	})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	if err := srv.Start(ctx); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer srv.Stop()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Send an email to multiple recipients
	err = smtp.SendMail(
		"localhost:2527",
		nil,
		"sender@example.com",
		[]string{"recipient1@example.com", "recipient2@example.com", "recipient3@example.com"},
		[]byte("Subject: Multiple Recipients\r\n\r\nThis is the body."),
	)
	if err != nil {
		t.Fatalf("Failed to send email: %v", err)
	}

	// Give it time to process
	time.Sleep(100 * time.Millisecond)

	// Verify all recipients got a copy
	conn, err = db.Connect(ctx)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close(ctx)

	var count int
	err = conn.QueryRow(ctx, "SELECT COUNT(*) FROM public.captured_emails").Scan(&count)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if count != 3 {
		t.Errorf("Expected 3 captured emails (one per recipient), got %d", count)
	}

	// Verify each recipient has their own copy
	rows, _ := conn.Query(ctx, "SELECT to_addr FROM public.captured_emails ORDER BY to_addr")
	defer rows.Close()

	recipients := []string{}
	for rows.Next() {
		var toAddr string
		rows.Scan(&toAddr)
		recipients = append(recipients, toAddr)
	}

	expected := []string{"recipient1@example.com", "recipient2@example.com", "recipient3@example.com"}
	if len(recipients) != 3 {
		t.Errorf("Got %d recipients, want 3", len(recipients))
	}
	for i := range expected {
		if i >= len(recipients) || recipients[i] != expected[i] {
			t.Errorf("recipients[%d] = %q, want %q", i, recipients[i], expected[i])
		}
	}
}
