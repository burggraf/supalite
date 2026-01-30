// Package dashboard provides JWT authentication and the web dashboard for Supalite.
//
// The dashboard package handles:
//   - JWT token generation and verification for dashboard authentication
//   - HTTP routes for login, user info, and status
//   - Static file serving for the web UI
//
// JWT tokens use HS256 signing with a secret generated on first run.
// Tokens expire after 24 hours and include user email in claims.
package dashboard

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	// TokenLifetime is the lifetime of dashboard JWT tokens.
	// Dashboard tokens are shorter-lived (24 hours) compared to
	// API keys (10 years) since they're used for interactive sessions.
	TokenLifetime = time.Hour * 24

	// Issuer is the JWT issuer claim for dashboard tokens.
	Issuer = "supalite-dashboard"
)

// JWTManager handles JWT token generation and verification for dashboard authentication.
//
// The manager uses HS256 signing with a secret key that should be
// generated on first run and persisted to disk.
type JWTManager struct {
	secretKey []byte
}

// Claims represents the JWT claims for dashboard authentication.
//
// Claims include standard JWT registered claims plus custom fields:
//   - Email: User's email address
//   - TokenID: Unique identifier for the token (UUID)
type Claims struct {
	Email   string `json:"email"`
	TokenID string `json:"token_id"`
	jwt.RegisteredClaims
}

// NewJWTManager creates a new JWT manager for dashboard authentication.
//
// Parameters:
//   - secretKey: The secret key for signing JWT tokens (should be 32+ bytes)
//
// Returns a configured JWT manager ready for token generation and verification.
//
// Example:
//	manager := dashboard.NewJWTManager([]byte("your-secret-key-min-32-bytes"))
//	token, err := manager.GenerateToken("user@example.com")
func NewJWTManager(secretKey []byte) *JWTManager {
	return &JWTManager{
		secretKey: secretKey,
	}
}

// GenerateToken generates a JWT token for the specified email.
//
// The token includes:
//   - Email claim: The user's email address
//   - TokenID claim: Unique UUID for this token
//   - Issuer: "supalite-dashboard"
//   - IssuedAt: Current time
//   - Expiration: 24 hours from now
//
// Parameters:
//   - email: The user's email address to include in the token
//
// Returns the signed JWT token string or an error.
//
// Example:
//	token, err := manager.GenerateToken("user@example.com")
//	if err != nil {
//	    log.Fatal(err)
//	}
func (m *JWTManager) GenerateToken(email string) (string, error) {
	if len(m.secretKey) == 0 {
		return "", errors.New("secret key is empty")
	}

	now := time.Now()
	tokenID := uuid.New().String()

	claims := &Claims{
		Email:   email,
		TokenID: tokenID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    Issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(TokenLifetime)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secretKey)
}

// VerifyToken verifies a JWT token and returns the claims.
//
// This method parses and validates the JWT signature and expiration.
// Returns the claims if the token is valid, or an error otherwise.
//
// Parameters:
//   - tokenString: The JWT token string to verify
//
// Returns the parsed claims or an error if verification fails.
//
// Example:
//	claims, err := manager.VerifyToken(tokenString)
//	if err != nil {
//	    return nil, err
//	}
//	fmt.Printf("User: %s\n", claims.Email)
func (m *JWTManager) VerifyToken(tokenString string) (*Claims, error) {
	if len(m.secretKey) == 0 {
		return nil, errors.New("secret key is empty")
	}

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return m.secretKey, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}
