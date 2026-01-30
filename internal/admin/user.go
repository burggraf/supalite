package admin

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// User represents an admin user
type User struct {
	ID           uuid.UUID `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Create inserts a new admin user into the database
func Create(ctx context.Context, conn *pgx.Conn, email, password string) (*User, error) {
	hash, err := HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user := &User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: hash,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	query := `
		INSERT INTO admin.users (id, email, password_hash, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, email, password_hash, created_at, updated_at
	`

	row := conn.QueryRow(ctx, query, user.ID, user.Email, user.PasswordHash, user.CreatedAt, user.UpdatedAt)
	err = row.Scan(&user.ID, &user.Email, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

// FindByEmail looks up a user by email address
func FindByEmail(ctx context.Context, conn *pgx.Conn, email string) (*User, error) {
	if email == "" {
		return nil, fmt.Errorf("email cannot be empty")
	}

	query := `
		SELECT id, email, password_hash, created_at, updated_at
		FROM admin.users
		WHERE email = $1
	`

	var user User
	err := conn.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to find user by email: %w", err)
	}

	return &user, nil
}

// List returns all admin users
func List(ctx context.Context, conn *pgx.Conn) ([]User, error) {
	query := `
		SELECT id, email, created_at, updated_at
		FROM admin.users
		ORDER BY created_at DESC
	`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		return []User{}, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	users := make([]User, 0)
	for rows.Next() {
		var user User
		err := rows.Scan(&user.ID, &user.Email, &user.CreatedAt, &user.UpdatedAt)
		if err != nil {
			return []User{}, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return []User{}, fmt.Errorf("error iterating users: %w", err)
	}

	return users, nil
}

// Delete removes a user by email
func Delete(ctx context.Context, conn *pgx.Conn, email string) error {
	if email == "" {
		return fmt.Errorf("email cannot be empty")
	}

	query := `DELETE FROM admin.users WHERE email = $1`
	_, err := conn.Exec(ctx, query, email)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	return nil
}

// UpdatePassword changes a user's password
func UpdatePassword(ctx context.Context, conn *pgx.Conn, email, newPassword string) error {
	if email == "" {
		return fmt.Errorf("email cannot be empty")
	}
	if newPassword == "" {
		return fmt.Errorf("new password cannot be empty")
	}

	hash, err := HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	query := `
		UPDATE admin.users
		SET password_hash = $1, updated_at = $2
		WHERE email = $3
	`
	_, err = conn.Exec(ctx, query, hash, time.Now(), email)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}
	return nil
}

// Count returns the total number of admin users
func Count(ctx context.Context, conn *pgx.Conn) (int, error) {
	var count int
	err := conn.QueryRow(ctx, "SELECT COUNT(*) FROM admin.users").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count users: %w", err)
	}
	return count, nil
}
