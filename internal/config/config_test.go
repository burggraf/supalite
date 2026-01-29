package config

import (
	"os"
	"testing"
)

func TestEmailConfig_CaptureMode(t *testing.T) {
	// Create temp config file with capture mode
	configJSON := `{
		"email": {
			"capture_mode": true,
			"capture_port": 2525
		}
	}`

	if err := os.WriteFile("supalite.json", []byte(configJSON), 0644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove("supalite.json")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Email == nil {
		t.Fatal("Email config should not be nil")
	}
	if !cfg.Email.CaptureMode {
		t.Error("CaptureMode should be true")
	}
	if cfg.Email.CapturePort != 2525 {
		t.Errorf("CapturePort = %d, want 2525", cfg.Email.CapturePort)
	}
}

func TestEmailConfig_CaptureMode_EnvFallback(t *testing.T) {
	os.Setenv("SUPALITE_CAPTURE_MODE", "true")
	os.Setenv("SUPALITE_CAPTURE_PORT", "3025")
	defer os.Unsetenv("SUPALITE_CAPTURE_MODE")
	defer os.Unsetenv("SUPALITE_CAPTURE_PORT")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if !cfg.Email.CaptureMode {
		t.Error("CaptureMode should be true from env var")
	}
	if cfg.Email.CapturePort != 3025 {
		t.Errorf("CapturePort = %d, want 3025", cfg.Email.CapturePort)
	}
}
