// Package keys provides JWT key management and token generation for Supalite.
//
// It supports two modes of operation:
//   - ES256 (default): Asymmetric ECDSA P-256 keys for modern JWT signing
//   - HS256 (legacy): Symmetric HMAC-SHA256 using JWT_SECRET
//
// The package automatically generates anon and service_role API keys,
// persists them to disk, and provides JWKS endpoint support for
// public key discovery in ES256 mode.
//
// # ES256 Mode (Default)
//
// When no JWT secret is provided, the manager generates an ECDSA P-256
// key pair and creates JWT tokens signed with ES256:
//
//	manager, err := keys.NewManager("./data", "")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	anonKey := manager.GetAnonKey()      // Public token for clients
//	serviceKey := manager.GetServiceKey() // Admin token for server use
//
//	// Get JWKS for public key discovery
//	jwks, err := manager.GetJWKS()
//
// # Legacy HS256 Mode
//
// When a JWT secret is provided, the manager uses symmetric HS256 signing:
//
//	manager, err := keys.NewManager("./data", "your-secret-key")
//
// # Token Structure
//
// Generated tokens include standard Supabase claims:
//
//	{
//	  "iss": "supabase",
//	  "ref": "qd7xe4gnosbcm8053sh6", // 20-char project reference
//	  "role": "anon",                  // or "service_role"
//	  "iat": 1769652835,               // Issued at
//	  "exp": 2085012835                // Expires in 10 years
//	}
//
// # Key Persistence
//
// Keys are automatically persisted to <data-dir>/keys.json in ES256 mode:
//
//	{
//	  "private_key_pem": "-----BEGIN EC PRIVATE KEY-----\n...",
//	  "anon_key": "eyJhbGciOiJFUzI1NiIsInR5cCI6IkpXVCJ9...",
//	  "service_key": "eyJhbGciOiJFUzI1NiIsInR5cCI6IkpXVCJ9...",
//	  "project_ref": "qd7xe4gnosbcm8053sh6",
//	  "created_at": "2026-01-28T18:13:55.337051-08:00"
//	}
package keys

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

const (
	// KeyID is the key ID used in JWT headers and JWKS.
	// This value is included in the JWT "kid" header parameter
	// and the JWKS "kid" field for key identification.
	KeyID = "supalite-key-1"

	// TokenLifetime is the lifetime of anon/service_role tokens.
	// Set to 10 years to match Supabase's default token lifetime.
	// Tokens are long-lived because they're meant to be used
	// as API keys rather than session tokens.
	TokenLifetime = time.Hour * 24 * 365 * 10
)

// Manager handles JWT signing keys and token generation.
//
// The manager supports two modes:
//   - ES256 mode (default): Uses asymmetric ECDSA P-256 keys
//   - Legacy mode: Uses symmetric HS256 with JWT_SECRET
//
// In ES256 mode, the manager generates a key pair on first run
// and persists it to disk. Subsequent runs load the existing keys.
//
// In legacy mode, tokens are signed using the provided JWT_SECRET.
type Manager struct {
	privateKey   *ecdsa.PrivateKey // ES256 private key for signing
	publicKey    *ecdsa.PublicKey  // ES256 public key for verification
	jwtSecret    []byte            // HS256 secret for legacy mode
	useLegacy    bool              // true = HS256 mode, false = ES256 mode
	anonKey      string            // anon JWT token
	serviceKey   string            // service_role JWT token
	projectRef   string            // 20-character project reference
	keysFilePath string            // path to keys.json file
}

// StoredKeys represents the persisted keys on disk.
//
// This struct is used to serialize keys to JSON for storage.
// The private key is stored in PEM format for security and portability.
type StoredKeys struct {
	PrivateKeyPEM string    `json:"private_key_pem"` // PEM-encoded EC private key
	AnonKey       string    `json:"anon_key"`        // anon JWT token
	ServiceKey    string    `json:"service_key"`     // service_role JWT token
	ProjectRef    string    `json:"project_ref"`     // 20-character project reference
	CreatedAt     time.Time `json:"created_at"`      // Key generation timestamp
}

// NewManager creates a new key manager.
//
// If jwtSecret is provided (non-empty), the manager operates in legacy HS256 mode.
// If jwtSecret is empty, the manager operates in ES256 mode and loads or
// generates an ECDSA P-256 key pair.
//
// In ES256 mode, keys are automatically persisted to <dataDir>/keys.json
// and loaded from disk on subsequent runs.
//
// Parameters:
//   - dataDir: Directory for storing keys.json and other data
//   - jwtSecret: Optional JWT secret for legacy HS256 mode (empty = ES256 mode)
//
// Returns:
//   - *Manager: Configured key manager ready for use
//   - error: Any error during key generation or loading
//
// Example (ES256 mode):
//	manager, err := keys.NewManager("./data", "")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// Example (Legacy HS256 mode):
//	manager, err := keys.NewManager("./data", "my-secret-key")
func NewManager(dataDir string, jwtSecret string) (*Manager, error) {
	m := &Manager{
		keysFilePath: filepath.Join(dataDir, "keys.json"),
	}

	// Legacy mode: JWT_SECRET provided
	if jwtSecret != "" {
		m.useLegacy = true
		m.jwtSecret = []byte(jwtSecret)
		if err := m.generateLegacyTokens(); err != nil {
			return nil, err
		}
		return m, nil
	}

	// Modern mode: Load or generate ES256 keys
	if err := m.loadOrGenerateKeys(); err != nil {
		return nil, err
	}

	return m, nil
}

// loadOrGenerateKeys loads existing keys from disk or generates new ones.
//
// This method first attempts to load keys from the keys.json file.
// If the file doesn't exist or is invalid, new keys are generated.
//
// Returns an error if key generation fails or persisted keys are invalid.
func (m *Manager) loadOrGenerateKeys() error {
	// Try to load existing keys
	if data, err := os.ReadFile(m.keysFilePath); err == nil {
		var stored StoredKeys
		if err := json.Unmarshal(data, &stored); err == nil {
			// Decode private key from PEM
			block, _ := pem.Decode([]byte(stored.PrivateKeyPEM))
			if block != nil {
				key, err := x509.ParseECPrivateKey(block.Bytes)
				if err == nil {
					m.privateKey = key
					m.publicKey = &key.PublicKey
					m.anonKey = stored.AnonKey
					m.serviceKey = stored.ServiceKey
					m.projectRef = stored.ProjectRef
					return nil
				}
			}
		}
	}

	// Generate new keys
	return m.generateKeys()
}

// generateKeys creates a new ES256 key pair and JWT tokens.
//
// This method:
// 1. Generates an ECDSA P-256 key pair
// 2. Creates a random project reference
// 3. Generates anon and service_role JWT tokens
// 4. Persists everything to disk
//
// Returns an error if any step fails.
func (m *Manager) generateKeys() error {
	// Generate ECDSA P-256 private key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	m.privateKey = privateKey
	m.publicKey = &privateKey.PublicKey

	// Generate project ref (random string like Supabase)
	m.projectRef = generateProjectRef()

	// Generate anon and service_role JWT tokens
	anonToken, err := m.generateToken("anon")
	if err != nil {
		return fmt.Errorf("failed to generate anon token: %w", err)
	}

	serviceToken, err := m.generateToken("service_role")
	if err != nil {
		return fmt.Errorf("failed to generate service token: %w", err)
	}

	m.anonKey = anonToken
	m.serviceKey = serviceToken

	// Save to disk
	if err := m.saveKeys(); err != nil {
		return fmt.Errorf("failed to save keys: %w", err)
	}

	return nil
}

// generateLegacyTokens creates anon/service_role tokens using JWT_SECRET (HS256).
//
// This method is used in legacy mode when a JWT_SECRET is provided.
// Tokens are signed using HMAC-SHA256 symmetric encryption.
//
// Returns an error if JWT_SECRET is not set or token signing fails.
func (m *Manager) generateLegacyTokens() error {
	if len(m.jwtSecret) == 0 {
		return fmt.Errorf("JWT_SECRET not provided")
	}

	// Generate project ref
	m.projectRef = generateProjectRef()

	now := time.Now()

	// Generate anon token (HS256)
	anonToken, err := jwt.NewBuilder().
		Issuer("supabase").
		Claim("ref", m.projectRef).
		Claim("role", "anon").
		IssuedAt(now).
		Expiration(now.Add(TokenLifetime)).
		Build()
	if err != nil {
		return fmt.Errorf("failed to build anon token: %w", err)
	}

	// Sign with HS256
	signedAnon, err := jwt.Sign(anonToken, jwt.WithKey(jwa.HS256, m.jwtSecret))
	if err != nil {
		return fmt.Errorf("failed to sign anon token: %w", err)
	}
	m.anonKey = string(signedAnon)

	// Generate service_role token (HS256)
	serviceToken, err := jwt.NewBuilder().
		Issuer("supabase").
		Claim("ref", m.projectRef).
		Claim("role", "service_role").
		IssuedAt(now).
		Expiration(now.Add(TokenLifetime)).
		Build()
	if err != nil {
		return fmt.Errorf("failed to build service token: %w", err)
	}

	// Sign with HS256
	signedService, err := jwt.Sign(serviceToken, jwt.WithKey(jwa.HS256, m.jwtSecret))
	if err != nil {
		return fmt.Errorf("failed to sign service token: %w", err)
	}
	m.serviceKey = string(signedService)

	return nil
}

// generateToken creates a JWT token for the specified role (ES256).
//
// The token includes standard Supabase claims:
//   - iss: "supabase"
//   - ref: project reference (20 chars)
//   - role: "anon" or "service_role"
//   - iat: issued at timestamp
//   - exp: expiration timestamp (10 years)
//
// Parameters:
//   - role: The role claim ("anon" or "service_role")
//
// Returns the signed JWT token string or an error.
func (m *Manager) generateToken(role string) (string, error) {
	now := time.Now()

	token, err := jwt.NewBuilder().
		Issuer("supabase").
		Claim("ref", m.projectRef).
		Claim("role", role).
		IssuedAt(now).
		Expiration(now.Add(TokenLifetime)).
		Build()
	if err != nil {
		return "", err
	}

	// Sign with ES256
	signed, err := jwt.Sign(token, jwt.WithKey(jwa.ES256, m.privateKey))
	if err != nil {
		return "", err
	}

	return string(signed), nil
}

// saveKeys persists the keys to disk.
//
// Keys are saved to the keysFilePath (data/keys.json) with:
//   - Private key in PEM format
//   - Generated anon and service_role tokens
//   - Project reference
//   - Creation timestamp
//
// File permissions are set to 0600 (owner read/write only).
//
// Returns an error if serialization or writing fails.
func (m *Manager) saveKeys() error {
	// Encode private key to PEM
	privateKeyBytes, err := x509.MarshalECPrivateKey(m.privateKey)
	if err != nil {
		return err
	}

	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	stored := StoredKeys{
		PrivateKeyPEM: string(privateKeyPEM),
		AnonKey:       m.anonKey,
		ServiceKey:    m.serviceKey,
		ProjectRef:    m.projectRef,
		CreatedAt:     time.Now(),
	}

	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.keysFilePath, data, 0600)
}

// GetAnonKey returns the anonymous key (public token).
//
// The anon key is intended for client-side use and can be safely
// exposed in browser applications. It provides standard user-level
// access to the API.
func (m *Manager) GetAnonKey() string {
	return m.anonKey
}

// GetServiceKey returns the service role key (administrative token).
//
// The service_role key bypasses Row Level Security (RLS) policies
// and should only be used server-side. Keep this key secret!
func (m *Manager) GetServiceKey() string {
	return m.serviceKey
}

// GetProjectRef returns the project reference.
//
// The project ref is a 20-character random string that identifies
// this Supalite instance. It's included in JWT tokens for
// compatibility with Supabase's token structure.
func (m *Manager) GetProjectRef() string {
	return m.projectRef
}

// GetJWKS returns the JWKS (JSON Web Key Set) for public key discovery.
//
// This method is only available in ES256 mode. It returns the public
// key in standard JWKS format for JWT verification by clients.
//
// Returns an error if called in legacy HS256 mode.
//
// The JWKS format includes:
//   - kty: "EC" (key type - elliptic curve)
//   - kid: KeyID for key identification
//   - use: "sig" (usage - signature)
//   - alg: "ES256" (algorithm)
//   - crv: "P-256" (curve)
//   - x: X coordinate (base64url encoded)
//   - y: Y coordinate (base64url encoded)
func (m *Manager) GetJWKS() (map[string]interface{}, error) {
	if m.useLegacy {
		// Legacy mode doesn't support JWKS
		return nil, fmt.Errorf("JWKS not available in legacy mode")
	}

	// Get the x and y coordinates from the public key
	xBytes := m.publicKey.X.Bytes()
	yBytes := m.publicKey.Y.Bytes()

	jwks := map[string]interface{}{
		"keys": []map[string]interface{}{
			{
				"kty": "EC",
				"kid": KeyID,
				"use": "sig",
				"alg": "ES256",
				"crv": "P-256",
				"x":   base64.RawURLEncoding.EncodeToString(xBytes),
				"y":   base64.RawURLEncoding.EncodeToString(yBytes),
			},
		},
	}

	return jwks, nil
}

// VerifyToken verifies a JWT token and returns the claims.
//
// This method parses and verifies the JWT signature using the
// appropriate key based on the current mode (ES256 or HS256).
//
// Parameters:
//   - tokenString: The JWT token string to verify
//
// Returns the parsed JWT token or an error if verification fails.
//
// Note: This method is currently not used by the server. GoTrue
// handles token verification for authentication flows.
func (m *Manager) VerifyToken(tokenString string) (jwt.Token, error) {
	if m.useLegacy {
		return jwt.ParseString(tokenString, jwt.WithVerify(false), jwt.WithKey(jwa.HS256, m.jwtSecret))
	}
	return jwt.ParseString(tokenString, jwt.WithVerify(false), jwt.WithKey(jwa.ES256, m.publicKey))
}

// IsLegacyMode returns true if using legacy JWT_SECRET mode (HS256).
//
// Returns false if using ES256 mode (the default).
func (m *Manager) IsLegacyMode() bool {
	return m.useLegacy
}

// generateProjectRef generates a random project reference like Supabase.
//
// The project ref is a 20-character string using lowercase letters
// and digits. This matches Supabase's project reference format.
//
// Returns a 20-character random string.
func generateProjectRef() string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 20)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[n.Int64()]
	}
	return string(b)
}
