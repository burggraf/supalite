package pg

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fergusstrange/embedded-postgres"
	"github.com/jackc/pgx/v5"
)

type EmbeddedDatabase struct {
	postgres   *embeddedpostgres.EmbeddedPostgres
	config     Config
	connString string
	mu         sync.RWMutex
	started    bool
}

func NewEmbeddedDatabase(cfg Config) *EmbeddedDatabase {
	// Apply defaults
	if cfg.Port == 0 {
		cfg.Port = 5432
	}
	if cfg.Username == "" {
		cfg.Username = "postgres"
	}
	if cfg.Password == "" {
		cfg.Password = "postgres"
	}
	if cfg.Database == "" {
		cfg.Database = "postgres"
	}
	if cfg.Version == "" {
		cfg.Version = "16.9.0"
	}

	return &EmbeddedDatabase{
		config: cfg,
		connString: fmt.Sprintf("postgres://%s:%s@localhost:%d/%s",
			cfg.Username, cfg.Password, cfg.Port, cfg.Database),
	}
}

func (db *EmbeddedDatabase) Start(ctx context.Context) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.started {
		return nil
	}

	config := embeddedpostgres.DefaultConfig().
		Port(uint32(db.config.Port)).
		Username(db.config.Username).
		Password(db.config.Password).
		Database(db.config.Database).
		Version(embeddedpostgres.PostgresVersion(db.config.Version))

	if db.config.DataDir != "" {
		if err := os.MkdirAll(db.config.DataDir, 0755); err != nil {
			return fmt.Errorf("failed to create data directory: %w", err)
		}
		config = config.DataPath(filepath.Join(db.config.DataDir, "data"))
	}

	db.postgres = embeddedpostgres.NewDatabase(config)

	done := make(chan error, 1)
	go func() {
		done <- db.postgres.Start()
	}()

	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("failed to start postgres: %w", err)
		}
	case <-ctx.Done():
		return fmt.Errorf("postgres start timed out: %w", ctx.Err())
	}

	if err := db.waitReady(ctx); err != nil {
		return fmt.Errorf("postgres not ready: %w", err)
	}

	db.started = true
	return nil
}

func (db *EmbeddedDatabase) Stop() {
	db.mu.Lock()
	defer db.mu.Unlock()

	if !db.started {
		return
	}

	if db.postgres != nil {
		db.postgres.Stop()
	}
	db.started = false
}

func (db *EmbeddedDatabase) ConnectionString() string {
	return db.connString
}

func (db *EmbeddedDatabase) Connect(ctx context.Context) (*pgx.Conn, error) {
	return pgx.Connect(ctx, db.connString)
}

func (db *EmbeddedDatabase) waitReady(ctx context.Context) error {
	const maxRetries = 60
	const retryDelay = 500 * time.Millisecond

	for i := 0; i < maxRetries; i++ {
		conn, err := db.Connect(ctx)
		if err == nil {
			conn.Close(ctx)
			return nil
		}

		select {
		case <-time.After(retryDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return fmt.Errorf("postgres did not become ready")
}
