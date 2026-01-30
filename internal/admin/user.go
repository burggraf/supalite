package admin

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// User represents an admin user in the system.
//
// Admin users have access to the admin dashboard and can manage
// the Supalite instance. Passwords are stored as bcrypt hashes
// and should never be exposed in plain text.
type User struct {
	ID           string    `json:"id"`            // UUID primary key
	Username     string    `json:"username"`      // Unique username
	PasswordHash string    `json:"-"`             // bcrypt hash (never exposed in JSON)
	Email        string    `json:"email"`         // Optional email address
	CreatedAt    time.Time `json:"created_at"`    // Account creation timestamp
	UpdatedAt    time.Time `json:"updated_at"`    // Last update timestamp
}

// UserRepository handles database operations for admin users.
//
// The repository provides CRUD operations for admin users and manages
// database persistence using pgx.
type UserRepository struct {
	conn *pgx.Conn
}

// NewUserRepository creates a new user repository.
//
// Parameters:
//   - conn: Active pgx database connection
//
// Returns a configured UserRepository ready for use.
//
// Example:
//
//	conn, err := pgx.Connect(ctx, connString)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	repo := admin.NewUserRepository(conn)
func NewUserRepository(conn *pgx.Conn) *UserRepository {
	return &UserRepository{conn: conn}
}

// CreateUser creates a new admin user with a hashed password.
//
// This method hashes the password using bcrypt and stores the user
// in the database. The ID, CreatedAt, and UpdatedAt fields are
// automatically generated.
//
// Parameters:
//   - ctx: Context for the database operation
//   - username: Unique username for the admin user
//   - password: Plain-text password (will be hashed)
//   - email: Optional email address
//
// Returns the created User with generated fields populated, or an error.
//
// Example:
//
//	user, err := repo.CreateUser(ctx, "admin", "securePassword123", "admin@example.com")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Created user: %s (ID: %s)\n", user.Username, user.ID)
func (r *UserRepository) CreateUser(ctx context.Context, username string, password string, email string) (*User, error) {
	// Hash the password
	hash, err := HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Generate UUID and timestamps
	now := time.Now()
	user := &User{
		ID:           generateUUID(),
		Username:     username,
		PasswordHash: hash,
		Email:        email,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	// Insert into database
	query := `
		INSERT INTO admin_users (id, username, password_hash, email, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err = r.conn.Exec(ctx, query, user.ID, user.Username, user.PasswordHash, user.Email, user.CreatedAt, user.UpdatedAt)
	if err != nil {
		// Check for unique constraint violation
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
			return nil, fmt.Errorf("username '%s' already exists", username)
		}
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

// GetUserByUsername retrieves an admin user by username.
//
// This method is commonly used for login operations. The PasswordHash
// field is populated and can be used for password verification.
//
// Parameters:
//   - ctx: Context for the database operation
//   - username: Username to look up
//
// Returns the User if found, or an error if not found or on database error.
//
// Example:
//
//	user, err := repo.GetUserByUsername(ctx, "admin")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	// Verify password for login
//	if err := admin.VerifyPassword(inputPassword, user.PasswordHash); err == nil {
//	    // Login successful
//	}
func (r *UserRepository) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	query := `
		SELECT id, username, password_hash, email, created_at, updated_at
		FROM admin_users
		WHERE username = $1
	`

	var user User
	err := r.conn.QueryRow(ctx, query, username).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.Email,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("user '%s' not found", username)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

// GetUserByID retrieves an admin user by ID.
//
// Parameters:
//   - ctx: Context for the database operation
//   - id: UUID of the user to retrieve
//
// Returns the User if found, or an error if not found or on database error.
func (r *UserRepository) GetUserByID(ctx context.Context, id string) (*User, error) {
	query := `
		SELECT id, username, password_hash, email, created_at, updated_at
		FROM admin_users
		WHERE id = $1
	`

	var user User
	err := r.conn.QueryRow(ctx, query, id).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.Email,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("user with ID '%s' not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

// ListUsers retrieves all admin users.
//
// This returns all admin users in the system, ordered by creation date.
// The PasswordHash field is populated but should be handled with care.
//
// Parameters:
//   - ctx: Context for the database operation
//
// Returns a slice of all admin users, or an error on database failure.
func (r *UserRepository) ListUsers(ctx context.Context) ([]*User, error) {
	query := `
		SELECT id, username, password_hash, email, created_at, updated_at
		FROM admin_users
		ORDER BY created_at ASC
	`

	rows, err := r.conn.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		var user User
		if err := rows.Scan(
			&user.ID,
			&user.Username,
			&user.PasswordHash,
			&user.Email,
			&user.CreatedAt,
			&user.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, &user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating users: %w", err)
	}

	return users, nil
}

// UpdatePassword updates a user's password.
//
// This method hashes the new password using bcrypt before storing it.
// The UpdatedAt timestamp is automatically updated.
//
// Parameters:
//   - ctx: Context for the database operation
//   - username: Username of the user to update
//   - newPassword: New plain-text password (will be hashed)
//
// Returns an error if the user is not found or on database failure.
//
// Example:
//
//	err := repo.UpdatePassword(ctx, "admin", "newSecurePassword")
//	if err != nil {
//	    log.Fatal(err)
//	}
func (r *UserRepository) UpdatePassword(ctx context.Context, username string, newPassword string) error {
	// Hash the new password
	hash, err := HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Update in database
	query := `
		UPDATE admin_users
		SET password_hash = $1, updated_at = $2
		WHERE username = $3
	`
	now := time.Now()
	result, err := r.conn.Exec(ctx, query, hash, now, username)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("user '%s' not found", username)
	}

	return nil
}

// UpdateEmail updates a user's email address.
//
// Parameters:
//   - ctx: Context for the database operation
//   - username: Username of the user to update
//   - newEmail: New email address
//
// Returns an error if the user is not found or on database failure.
func (r *UserRepository) UpdateEmail(ctx context.Context, username string, newEmail string) error {
	query := `
		UPDATE admin_users
		SET email = $1, updated_at = $2
		WHERE username = $3
	`
	now := time.Now()
	result, err := r.conn.Exec(ctx, query, newEmail, now, username)
	if err != nil {
		return fmt.Errorf("failed to update email: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("user '%s' not found", username)
	}

	return nil
}

// DeleteUser deletes an admin user by username.
//
// Use with caution - this operation is irreversible.
//
// Parameters:
//   - ctx: Context for the database operation
//   - username: Username of the user to delete
//
// Returns an error if the user is not found or on database failure.
func (r *UserRepository) DeleteUser(ctx context.Context, username string) error {
	query := `DELETE FROM admin_users WHERE username = $1`
	result, err := r.conn.Exec(ctx, query, username)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("user '%s' not found", username)
	}

	return nil
}

// VerifyCredentials verifies a username and password combination.
//
// This is a convenience method that combines GetUserByUsername and
// VerifyPassword for login operations.
//
// Parameters:
//   - ctx: Context for the database operation
//   - username: Username to verify
//   - password: Plain-text password to verify
//
// Returns the User if credentials are valid, or an error if invalid.
//
// Example:
//
//	user, err := repo.VerifyCredentials(ctx, "admin", "password123")
//	if err != nil {
//	    log.Println("Login failed:", err)
//	    return
//	}
//	fmt.Printf("Login successful for user: %s\n", user.Username)
func (r *UserRepository) VerifyCredentials(ctx context.Context, username string, password string) (*User, error) {
	user, err := r.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials: %w", err)
	}

	if err := VerifyPassword(password, user.PasswordHash); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	return user, nil
}

// generateUUID generates a UUID v4.
//
// This is a simple implementation for generating UUIDs without external dependencies.
// For production use, consider using a more robust UUID library.
func generateUUID() string {
	// Simple UUID v4 generation using crypto/rand
	// Format: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback to time-based if crypto/rand fails
		for i := range b {
			b[i] = byte(time.Now().UnixNano() >> (i * 8))
		}
	}

	b[6] = (b[6] & 0x0f) | 0x40 // Version 4
	b[8] = (b[8] & 0x3f) | 0x80 // Variant 10

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
