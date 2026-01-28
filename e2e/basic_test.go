package e2e

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/markb/supalite/internal/server"
)

func TestServer_Startup(t *testing.T) {
	// Create server with test ports
	srv := server.New(server.Config{
		Host:      "127.0.0.1",
		Port:      18080,
		DataDir:   "/tmp/supalite-e2e",
		JWTSecret: "test-secret",
		SiteURL:   "http://localhost:18080",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	// Start server in background
	errCh := make(chan error, 1)
	go func() {
		err := srv.Start(ctx)
		if err != nil && err != context.Canceled {
			t.Logf("Server start error: %v", err)
		}
		errCh <- err
	}()

	// Wait for startup with retries
	var resp *http.Response
	var err error

	t.Log("Waiting for server to start...")
	for i := 0; i < 120; i++ {
		time.Sleep(1 * time.Second)
		resp, err = http.Get("http://127.0.0.1:18080/health")
		if err == nil {
			t.Logf("Server started after %d seconds", i+1)
			break
		}
		if (i+1)%10 == 0 {
			t.Logf("Attempt %d: %v", i+1, err)
		}
	}

	if err != nil {
		t.Fatalf("Health check failed after 120 attempts: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Health check returned status %d", resp.StatusCode)
	}

	t.Log("Health check passed!")

	// Cancel to trigger shutdown
	cancel()

	// Verify clean shutdown
	if err := <-errCh; err != nil && err != context.Canceled {
		t.Errorf("Server error: %v", err)
	}
}
