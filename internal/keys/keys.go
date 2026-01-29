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
	// KeyID is the key ID used in JWT headers and JWKS
	KeyID = "supalite-key-1"

	// TokenLifetime is the lifetime of anon/service_role tokens (10 years, like Supabase)
	TokenLifetime = time.Hour * 24 * 365 * 10
)

// Manager handles JWT signing keys and token generation
type Manager struct {
	privateKey   *ecdsa.PrivateKey
	publicKey    *ecdsa.PublicKey
	jwtSecret    []byte // For legacy mode
	useLegacy    bool   // Whether using legacy JWT_SECRET mode
	anonKey      string
	serviceKey   string
	projectRef   string
	keysFilePath string
}

// StoredKeys represents the persisted keys on disk
type StoredKeys struct {
	PrivateKeyPEM string    `json:"private_key_pem"`
	AnonKey       string    `json:"anon_key"`
	ServiceKey    string    `json:"service_key"`
	ProjectRef    string    `json:"project_ref"`
	CreatedAt     time.Time `json:"created_at"`
}

// NewManager creates a new key manager
// If jwtSecret is provided, uses legacy mode
// Otherwise, loads or generates ES256 key pair
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

// loadOrGenerateKeys loads existing keys or generates new ones
func (m *Manager) loadOrGenerateKeys() error {
	// Try to load existing keys
	if data, err := os.ReadFile(m.keysFilePath); err == nil {
		var stored StoredKeys
		if err := json.Unmarshal(data, &stored); err == nil {
			// Decode private key
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

// generateKeys creates a new ES256 key pair and JWT tokens
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

// generateLegacyTokens creates anon/service_role tokens using JWT_SECRET (HS256)
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

// generateToken creates a JWT token for the specified role (ES256)
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

// saveKeys persists the keys to disk
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

// GetAnonKey returns the anonymous key (public)
func (m *Manager) GetAnonKey() string {
	return m.anonKey
}

// GetServiceKey returns the service role key (secret)
func (m *Manager) GetServiceKey() string {
	return m.serviceKey
}

// GetProjectRef returns the project reference
func (m *Manager) GetProjectRef() string {
	return m.projectRef
}

// GetJWKS returns the JWKS (JSON Web Key Set) for public key discovery
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

// VerifyToken verifies a JWT token and returns the claims
func (m *Manager) VerifyToken(tokenString string) (jwt.Token, error) {
	if m.useLegacy {
		return jwt.ParseString(tokenString, jwt.WithVerify(false), jwt.WithKey(jwa.HS256, m.jwtSecret))
	}
	return jwt.ParseString(tokenString, jwt.WithVerify(false), jwt.WithKey(jwa.ES256, m.publicKey))
}

// IsLegacyMode returns true if using legacy JWT_SECRET mode
func (m *Manager) IsLegacyMode() bool {
	return m.useLegacy
}

// generateProjectRef generates a random project reference like Supabase
func generateProjectRef() string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 20)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[n.Int64()]
	}
	return string(b)
}
