package admin

import (
	"testing"
)

func TestHashPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{
			name:     "valid password",
			password: "mySecurePassword123!",
			wantErr:  false,
		},
		{
			name:     "short password",
			password: "pass",
			wantErr:  false,
		},
		{
			name:     "long password",
			password: "this-is-a-very-long-password-with-many-characters-123456789!@#$%^&*()",
			wantErr:  false,
		},
		{
			name:     "password with special chars",
			password: "p@ssw0rd!#$%^&*()",
			wantErr:  false,
		},
		{
			name:     "empty password",
			password: "",
			wantErr:  false, // bcrypt allows empty passwords
		},
		{
			name:     "password with unicode",
			password: "密码🔐",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := HashPassword(tt.password)

			if (err != nil) != tt.wantErr {
				t.Errorf("HashPassword() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify hash is not empty
				if hash == "" {
					t.Error("HashPassword() returned empty hash")
				}

				// Verify hash starts with bcrypt prefix
				if len(hash) < 4 || hash[:4] != "$2a$" && hash[:4] != "$2b$" {
					t.Errorf("HashPassword() hash doesn't have bcrypt prefix, got: %s", hash[:4])
				}

				// Verify hash is different from password
				if hash == tt.password {
					t.Error("HashPassword() hash equals password (not hashed)")
				}
			}
		})
	}
}

func TestHashPassword_Uniqueness(t *testing.T) {
	password := "samePassword"

	hash1, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() failed: %v", err)
	}

	hash2, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() failed: %v", err)
	}

	// Same password should produce different hashes due to salt
	if hash1 == hash2 {
		t.Error("HashPassword() produced identical hashes for same password (salt issue)")
	}
}

func TestVerifyPassword(t *testing.T) {
	password := "mySecurePassword123!"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() failed: %v", err)
	}

	tests := []struct {
		name     string
		password string
		hash     string
		wantErr  bool
	}{
		{
			name:     "correct password",
			password: password,
			hash:     hash,
			wantErr:  false,
		},
		{
			name:     "incorrect password",
			password: "wrongPassword",
			hash:     hash,
			wantErr:  true,
		},
		{
			name:     "empty password",
			password: "",
			hash:     hash,
			wantErr:  true,
		},
		{
			name:     "password with different case",
			password: "MYSECUREPASSWORD123!",
			hash:     hash,
			wantErr:  true,
		},
		{
			name:     "malformed hash",
			password: password,
			hash:     "invalid-hash",
			wantErr:  true,
		},
		{
			name:     "empty hash",
			password: password,
			hash:     "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := VerifyPassword(tt.password, tt.hash)
			if (err != nil) != tt.wantErr {
				t.Errorf("VerifyPassword() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestVerifyPassword_HashAndVerify(t *testing.T) {
	passwords := []string{
		"simple",
		"complex123!@#",
		"with spaces",
		"mixedCASE123",
		"unicode密码",
	}

	for _, password := range passwords {
		t.Run(password, func(t *testing.T) {
			hash, err := HashPassword(password)
			if err != nil {
				t.Fatalf("HashPassword() failed: %v", err)
			}

			// Correct password should verify
			if err := VerifyPassword(password, hash); err != nil {
				t.Errorf("VerifyPassword() failed for correct password: %v", err)
			}

			// Wrong password should not verify
			wrongPassword := password + "wrong"
			if err := VerifyPassword(wrongPassword, hash); err == nil {
				t.Error("VerifyPassword() succeeded for wrong password")
			}
		})
	}
}
