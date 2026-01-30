package admin

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// getTestConnection creates a test database connection
func getTestConnection(t *testing.T) *pgx.Conn {
	// Try to connect to the test database
	connStr := os.Getenv("TEST_DATABASE_URL")
	if connStr == "" {
		connStr = "postgres://postgres:postgres@localhost:5432/postgres"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, connStr)
	if err != nil {
		t.Skipf("Cannot connect to test database: %v", err)
	}

	// Create test table - drop and recreate to ensure fresh state with constraints
	initQuery := `
		DROP TABLE IF EXISTS admin_users CASCADE;
		CREATE TABLE admin_users (
			id TEXT PRIMARY KEY,
			username TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			email TEXT,
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

func cleanupTestUser(t *testing.T, conn *pgx.Conn, username string) {
	ctx := context.Background()
	_, err := conn.Exec(ctx, "DELETE FROM admin_users WHERE username = $1", username)
	if err != nil {
		t.Errorf("Failed to cleanup test user: %v", err)
	}
}

func TestNewUserRepository(t *testing.T) {
	conn := getTestConnection(t)
	defer conn.Close(context.Background())

	repo := NewUserRepository(conn)
	if repo == nil {
		t.Fatal("NewUserRepository() returned nil")
	}
	if repo.conn != conn {
		t.Error("NewUserRepository() did not store connection")
	}
}

func TestCreateUser(t *testing.T) {
	conn := getTestConnection(t)
	defer conn.Close(context.Background())
	repo := NewUserRepository(conn)
	ctx := context.Background()

	t.Run("valid user", func(t *testing.T) {
		username := "test-user1"
		user, err := repo.CreateUser(ctx, username, "securePassword123!", "test1@example.com")

		if err != nil {
			t.Fatalf("CreateUser() unexpected error: %v", err)
		}

		// Verify user fields
		if user.ID == "" {
			t.Error("CreateUser() returned user with empty ID")
		}
		if user.Username != username {
			t.Errorf("CreateUser() username = %s, want %s", user.Username, username)
		}
		if user.Email != "test1@example.com" {
			t.Errorf("CreateUser() email = %s, want %s", user.Email, "test1@example.com")
		}
		if user.PasswordHash == "" {
			t.Error("CreateUser() returned user with empty PasswordHash")
		}
		if user.PasswordHash == "securePassword123!" {
			t.Error("CreateUser() stored plain-text password (not hashed)")
		}
		if user.CreatedAt.IsZero() {
			t.Error("CreateUser() returned user with zero CreatedAt")
		}
		if user.UpdatedAt.IsZero() {
			t.Error("CreateUser() returned user with zero UpdatedAt")
		}

		// Cleanup
		cleanupTestUser(t, conn, username)
	})

	t.Run("user without email", func(t *testing.T) {
		username := "test-user2"
		user, err := repo.CreateUser(ctx, username, "securePassword123!", "")

		if err != nil {
			t.Fatalf("CreateUser() unexpected error: %v", err)
		}

		if user.Email != "" {
			t.Errorf("CreateUser() email = %s, want empty", user.Email)
		}

		// Cleanup
		cleanupTestUser(t, conn, username)
	})

	t.Run("duplicate username", func(t *testing.T) {
		username := "test-user3"

		// Create first user
		_, err := repo.CreateUser(ctx, username, "password123", "test@example.com")
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}
		defer cleanupTestUser(t, conn, username)

		// Try to create duplicate
		_, err = repo.CreateUser(ctx, username, "anotherPassword", "test2@example.com")
		if err == nil {
			t.Error("CreateUser() expected error for duplicate username but got nil")
			return
		}

		errStr := err.Error()
		expectedMsg := "already exists"
		found := false
		for i := 0; i <= len(errStr)-len(expectedMsg); i++ {
			if errStr[i:i+len(expectedMsg)] == expectedMsg {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("CreateUser() error = %v, want error containing %q", err, expectedMsg)
		}
	})

	t.Run("weak password (still allowed)", func(t *testing.T) {
		username := "test-user4"
		user, err := repo.CreateUser(ctx, username, "123", "test@example.com")

		// bcrypt accepts any password
		if err != nil {
			t.Fatalf("CreateUser() unexpected error: %v", err)
		}

		if user.PasswordHash == "" {
			t.Error("CreateUser() returned user with empty PasswordHash")
		}

		// Cleanup
		cleanupTestUser(t, conn, username)
	})
}

func TestGetUserByUsername(t *testing.T) {
	conn := getTestConnection(t)
	defer conn.Close(context.Background())
	repo := NewUserRepository(conn)
	ctx := context.Background()

	// Create test user
	testUser, err := repo.CreateUser(ctx, "test-getuser", "password123", "getuser@example.com")
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer cleanupTestUser(t, conn, "test-getuser")

	tests := []struct {
		name     string
		username string
		wantErr  bool
	}{
		{
			name:     "existing user",
			username: "test-getuser",
			wantErr:  false,
		},
		{
			name:     "non-existent user",
			username: "nonexistent-user",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := repo.GetUserByUsername(ctx, tt.username)

			if tt.wantErr {
				if err == nil {
					t.Error("GetUserByUsername() expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("GetUserByUsername() unexpected error: %v", err)
			}

			// Verify user fields
			if user.ID != testUser.ID {
				t.Errorf("GetUserByUsername() ID = %s, want %s", user.ID, testUser.ID)
			}
			if user.Username != testUser.Username {
				t.Errorf("GetUserByUsername() username = %s, want %s", user.Username, testUser.Username)
			}
			if user.PasswordHash != testUser.PasswordHash {
				t.Error("GetUserByUsername() PasswordHash mismatch")
			}
			if user.Email != testUser.Email {
				t.Errorf("GetUserByUsername() email = %s, want %s", user.Email, testUser.Email)
			}
		})
	}
}

func TestGetUserByID(t *testing.T) {
	conn := getTestConnection(t)
	defer conn.Close(context.Background())
	repo := NewUserRepository(conn)
	ctx := context.Background()

	// Create test user
	testUser, err := repo.CreateUser(ctx, "test-getbyid", "password123", "getbyid@example.com")
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer cleanupTestUser(t, conn, "test-getbyid")

	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{
			name:    "existing user",
			id:      testUser.ID,
			wantErr: false,
		},
		{
			name:    "non-existent user",
			id:      "00000000-0000-0000-0000-000000000000",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := repo.GetUserByID(ctx, tt.id)

			if tt.wantErr {
				if err == nil {
					t.Error("GetUserByID() expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("GetUserByID() unexpected error: %v", err)
			}

			if user.Username != testUser.Username {
				t.Errorf("GetUserByID() username = %s, want %s", user.Username, testUser.Username)
			}
		})
	}
}

func TestListUsers(t *testing.T) {
	conn := getTestConnection(t)
	defer conn.Close(context.Background())
	repo := NewUserRepository(conn)
	ctx := context.Background()

	// Create test users
	_, err := repo.CreateUser(ctx, "test-list1", "password123", "list1@example.com")
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer cleanupTestUser(t, conn, "test-list1")

	_, err = repo.CreateUser(ctx, "test-list2", "password123", "list2@example.com")
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer cleanupTestUser(t, conn, "test-list2")

	// Get all users
	users, err := repo.ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers() error: %v", err)
	}

	// Verify we have at least our test users
	found := 0
	for _, user := range users {
		if user.Username == "test-list1" || user.Username == "test-list2" {
			found++
		}
	}

	if found < 2 {
		t.Errorf("ListUsers() did not return all test users, found %d, want at least 2", found)
	}
}

func TestUpdatePassword(t *testing.T) {
	conn := getTestConnection(t)
	defer conn.Close(context.Background())
	repo := NewUserRepository(conn)
	ctx := context.Background()

	// Create test user
	oldPassword := "oldPassword123!"
	_, err := repo.CreateUser(ctx, "test-updatepass", oldPassword, "updatepass@example.com")
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer cleanupTestUser(t, conn, "test-updatepass")

	// Update password
	newPassword := "newPassword456!"
	err = repo.UpdatePassword(ctx, "test-updatepass", newPassword)
	if err != nil {
		t.Fatalf("UpdatePassword() error: %v", err)
	}

	// Verify password changed
	user, err := repo.GetUserByUsername(ctx, "test-updatepass")
	if err != nil {
		t.Fatalf("GetUserByUsername() error: %v", err)
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

func TestUpdatePassword_NonExistentUser(t *testing.T) {
	conn := getTestConnection(t)
	defer conn.Close(context.Background())
	repo := NewUserRepository(conn)
	ctx := context.Background()

	err := repo.UpdatePassword(ctx, "nonexistent-user", "newPassword")
	if err == nil {
		t.Error("UpdatePassword() expected error for non-existent user")
	}
}

func TestUpdateEmail(t *testing.T) {
	conn := getTestConnection(t)
	defer conn.Close(context.Background())
	repo := NewUserRepository(conn)
	ctx := context.Background()

	// Create test user
	_, err := repo.CreateUser(ctx, "test-updateemail", "password123", "old@example.com")
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer cleanupTestUser(t, conn, "test-updateemail")

	// Update email
	newEmail := "new@example.com"
	err = repo.UpdateEmail(ctx, "test-updateemail", newEmail)
	if err != nil {
		t.Fatalf("UpdateEmail() error: %v", err)
	}

	// Verify email changed
	user, err := repo.GetUserByUsername(ctx, "test-updateemail")
	if err != nil {
		t.Fatalf("GetUserByUsername() error: %v", err)
	}

	if user.Email != newEmail {
		t.Errorf("UpdateEmail() email = %s, want %s", user.Email, newEmail)
	}
}

func TestUpdateEmail_NonExistentUser(t *testing.T) {
	conn := getTestConnection(t)
	defer conn.Close(context.Background())
	repo := NewUserRepository(conn)
	ctx := context.Background()

	err := repo.UpdateEmail(ctx, "nonexistent-user", "new@example.com")
	if err == nil {
		t.Error("UpdateEmail() expected error for non-existent user")
	}
}

func TestDeleteUser(t *testing.T) {
	conn := getTestConnection(t)
	defer conn.Close(context.Background())
	repo := NewUserRepository(conn)
	ctx := context.Background()

	// Create test user
	_, err := repo.CreateUser(ctx, "test-delete", "password123", "delete@example.com")
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Delete user
	err = repo.DeleteUser(ctx, "test-delete")
	if err != nil {
		t.Fatalf("DeleteUser() error: %v", err)
	}

	// Verify user is gone
	_, err = repo.GetUserByUsername(ctx, "test-delete")
	if err == nil {
		t.Error("DeleteUser() user still exists after deletion")
	}
}

func TestDeleteUser_NonExistentUser(t *testing.T) {
	conn := getTestConnection(t)
	defer conn.Close(context.Background())
	repo := NewUserRepository(conn)
	ctx := context.Background()

	err := repo.DeleteUser(ctx, "nonexistent-user")
	if err == nil {
		t.Error("DeleteUser() expected error for non-existent user")
	}
}

func TestVerifyCredentials(t *testing.T) {
	conn := getTestConnection(t)
	defer conn.Close(context.Background())
	repo := NewUserRepository(conn)
	ctx := context.Background()

	// Create test user
	username := "test-verify"
	password := "correctPassword123!"
	_, err := repo.CreateUser(ctx, username, password, "verify@example.com")
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer cleanupTestUser(t, conn, username)

	tests := []struct {
		name     string
		username string
		password string
		wantErr  bool
	}{
		{
			name:     "correct credentials",
			username: username,
			password: password,
			wantErr:  false,
		},
		{
			name:     "wrong password",
			username: username,
			password: "wrongPassword",
			wantErr:  true,
		},
		{
			name:     "non-existent user",
			username: "nonexistent-user",
			password: password,
			wantErr:  true,
		},
		{
			name:     "empty password",
			username: username,
			password: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := repo.VerifyCredentials(ctx, tt.username, tt.password)

			if tt.wantErr {
				if err == nil {
					t.Error("VerifyCredentials() expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("VerifyCredentials() unexpected error: %v", err)
			}

			if user.Username != tt.username {
				t.Errorf("VerifyCredentials() username = %s, want %s", user.Username, tt.username)
			}

			// Verify password hash is not exposed in error messages
			if user.PasswordHash == "" {
				t.Error("VerifyCredentials() password hash should be populated for internal use")
			}
		})
	}
}

func TestGenerateUUID(t *testing.T) {
	// Test that generateUUID produces valid UUIDs
	uuids := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		uuid := generateUUID()
		if uuid == "" {
			t.Error("generateUUID() returned empty string")
		}
		if len(uuid) != 36 { // Standard UUID length
			t.Errorf("generateUUID() returned UUID of length %d, want 36", len(uuid))
		}
		if uuids[uuid] {
			t.Error("generateUUID() produced duplicate UUID")
		}
		uuids[uuid] = true
	}
}
