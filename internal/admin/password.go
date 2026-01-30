// Package admin provides user management and authentication for the Supalite admin dashboard.
//
// This package handles:
//   - Password hashing and verification using bcrypt
//   - Admin user CRUD operations
//   - Database persistence of admin users
//
// # Security
//
// Passwords are hashed using bcrypt with a cost factor of 10 (bcrypt.DefaultCost),
// which provides a good balance between security and performance. The bcrypt algorithm is
// resistant to brute force attacks and includes its own salt.
//
// # Database Schema
//
// Admin users are stored in the admin_users table:
//
//	CREATE TABLE admin_users (
//	    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
//	    username TEXT UNIQUE NOT NULL,
//	    password_hash TEXT NOT NULL,
//	    email TEXT,
//	    created_at TIMESTAMPTZ DEFAULT NOW(),
//	    updated_at TIMESTAMPTZ DEFAULT NOW()
//	);
package admin

import (
	"golang.org/x/crypto/bcrypt"
)

const (
	// BcryptCost is the cost factor for bcrypt hashing.
	// A cost of 10 provides a good balance between security and performance.
	// Each increment doubles the time required to hash a password.
	BcryptCost = bcrypt.DefaultCost
)

// HashPassword hashes a plain-text password using bcrypt.
//
// The bcrypt algorithm automatically generates a salt and includes it in
// the returned hash. The hash can be stored directly in the database.
//
// Parameters:
//   - password: Plain-text password to hash
//
// Returns the hashed password or an error if hashing fails.
//
// Example:
//
//	hash, err := admin.HashPassword("my-password")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	// Store hash in database
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// VerifyPassword verifies a plain-text password against a bcrypt hash.
//
// This function uses bcrypt's constant-time comparison to prevent timing attacks.
//
// Parameters:
//   - password: Plain-text password to verify
//   - hash: bcrypt hash to compare against (from database)
//
// Returns nil if the password matches, or an error if it doesn't.
//
// Example:
//
//	err := admin.VerifyPassword("my-password", storedHash)
//	if err != nil {
//	    // Password doesn't match
//	}
func VerifyPassword(password string, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
