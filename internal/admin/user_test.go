package admin

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

const testTimeout = 5 * time.Second

// getTestConnection creates a test database connection
func getTestConnection(t *testing.T) *pgx.Conn {
	// Try to connect to the test database
	connStr := os.Getenv("TEST_DATABASE_URL")
	if connStr == "" {
		connStr = "postgres://postgres:postgres@localhost:5432/postgres"
	}

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	conn, err := pgx.Connect(ctx, connStr)
	if err != nil {
		t.Skipf("Cannot connect to test database: %v", err)
	}

	// Create test table - drop and recreate to ensure fresh state with constraints
	initQuery := `
		DROP TABLE IF EXISTS admin.users CASCADE;
		DROP SCHEMA IF EXISTS admin CASCADE;
		CREATE SCHEMA admin;
		CREATE TABLE admin.users (
			id UUID PRIMARY KEY,
			email TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		);
	`
	_, err = conn.Exec(ctx, initQuery)
	if err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}

	return conn
}

func cleanupTestUser(t *testing.T, conn *pgx.Conn, email string) {
	ctx := context.Background()
	_, err := conn.Exec(ctx, "DELETE FROM admin.users WHERE email = $1", email)
	if err != nil {
		t.Errorf("Failed to cleanup test user: %v", err)
	}
}

func TestCreate(t *testing.T) {
	conn := getTestConnection(t)
	defer conn.Close(context.Background())
	ctx := context.Background()

	t.Run("valid user", func(t *testing.T) {
		email := "test1@example.com"
		user, err := Create(ctx, conn, email, "securePassword123!")

		if err != nil {
			t.Fatalf("Create() unexpected error: %v", err)
		}

		// Verify user fields
		if user.ID.String() == "" {
			t.Error("Create() returned user with empty ID")
		}
		if user.Email != email {
			t.Errorf("Create() email = %s, want %s", user.Email, email)
		}
		if user.PasswordHash == "" {
			t.Error("Create() returned user with empty PasswordHash")
		}
		if user.PasswordHash == "securePassword123!" {
			t.Error("Create() stored plain-text password (not hashed)")
		}
		if user.CreatedAt.IsZero() {
			t.Error("Create() returned user with zero CreatedAt")
		}
		if user.UpdatedAt.IsZero() {
			t.Error("Create() returned user with zero UpdatedAt")
		}

		// Cleanup
		cleanupTestUser(t, conn, email)
	})

	t.Run("duplicate email", func(t *testing.T) {
		email := "test2@example.com"

		// Create first user
		_, err := Create(ctx, conn, email, "password123")
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}
		defer cleanupTestUser(t, conn, email)

		// Try to create duplicate
		_, err = Create(ctx, conn, email, "anotherPassword")
		if err == nil {
			t.Error("Create() expected error for duplicate email but got nil")
		}
	})
}

func TestFindByEmail(t *testing.T) {
	conn := getTestConnection(t)
	defer conn.Close(context.Background())
	ctx := context.Background()

	// Create test user
	testEmail := "test-find@example.com"
	testUser, err := Create(ctx, conn, testEmail, "password123")
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer cleanupTestUser(t, conn, testEmail)

	tests := []struct {
		name    string
		email   string
		wantErr bool
	}{
		{
			name:    "existing user",
			email:   testEmail,
			wantErr: false,
		},
		{
			name:    "non-existent user",
			email:   "nonexistent@example.com",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := FindByEmail(ctx, conn, tt.email)

			if tt.wantErr {
				if err == nil {
					t.Error("FindByEmail() expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("FindByEmail() unexpected error: %v", err)
			}

			// Verify user fields
			if user.ID != testUser.ID {
				t.Errorf("FindByEmail() ID = %s, want %s", user.ID, testUser.ID)
			}
			if user.Email != testUser.Email {
				t.Errorf("FindByEmail() email = %s, want %s", user.Email, testUser.Email)
			}
			if user.PasswordHash != testUser.PasswordHash {
				t.Error("FindByEmail() PasswordHash mismatch")
			}
		})
	}
}

func TestList(t *testing.T) {
	conn := getTestConnection(t)
	defer conn.Close(context.Background())
	ctx := context.Background()

	// Create test users
	_, err := Create(ctx, conn, "list1@example.com", "password123")
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer cleanupTestUser(t, conn, "list1@example.com")

	_, err = Create(ctx, conn, "list2@example.com", "password123")
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer cleanupTestUser(t, conn, "list2@example.com")

	// Get all users
	users, err := List(ctx, conn)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	// Verify we have at least our test users
	found := 0
	for _, user := range users {
		if user.Email == "list1@example.com" || user.Email == "list2@example.com" {
			found++
		}
	}

	if found < 2 {
		t.Errorf("List() did not return all test users, found %d, want at least 2", found)
	}
}

func TestUpdatePassword(t *testing.T) {
	conn := getTestConnection(t)
	defer conn.Close(context.Background())
	ctx := context.Background()

	// Create test user
	oldPassword := "oldPassword123!"
	testEmail := "updatepass@example.com"
	_, err := Create(ctx, conn, testEmail, oldPassword)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer cleanupTestUser(t, conn, testEmail)

	// Update password
	newPassword := "newPassword456!"
	err = UpdatePassword(ctx, conn, testEmail, newPassword)
	if err != nil {
		t.Fatalf("UpdatePassword() error: %v", err)
	}

	// Verify password changed
	user, err := FindByEmail(ctx, conn, testEmail)
	if err != nil {
		t.Fatalf("FindByEmail() error: %v", err)
	}

	// Old password should not work
	if err := VerifyPassword(oldPassword, user.PasswordHash); err == nil {
		t.Error("Old password still works after UpdatePassword()")
	}

	// New password should work
	if err := VerifyPassword(newPassword, user.PasswordHash); err != nil {
		t.Errorf("New password doesn't work after UpdatePassword(): %v", err)
	}
}

func TestDelete(t *testing.T) {
	conn := getTestConnection(t)
	defer conn.Close(context.Background())
	ctx := context.Background()

	// Create test user
	testEmail := "delete@example.com"
	_, err := Create(ctx, conn, testEmail, "password123")
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Delete user
	err = Delete(ctx, conn, testEmail)
	if err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	// Verify user is gone
	_, err = FindByEmail(ctx, conn, testEmail)
	if err == nil {
		t.Error("Delete() user still exists after deletion")
	}
}

func TestCount(t *testing.T) {
	conn := getTestConnection(t)
	defer conn.Close(context.Background())
	ctx := context.Background()

	// Initial count should be 0
	count, err := Count(ctx, conn)
	if err != nil {
		t.Fatalf("Count() error: %v", err)
	}
	if count != 0 {
		t.Errorf("Count() = %d, want 0", count)
	}

	// Add a user
	testEmail := "count@example.com"
	_, err = Create(ctx, conn, testEmail, "password123")
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer cleanupTestUser(t, conn, testEmail)

	// Count should be 1
	count, err = Count(ctx, conn)
	if err != nil {
		t.Fatalf("Count() error: %v", err)
	}
	if count != 1 {
		t.Errorf("Count() = %d, want 1", count)
	}
}
