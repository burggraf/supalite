# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Supalite is a single-binary Go backend that provides Supabase-compatible functionality by orchestrating three services: Embedded PostgreSQL, pREST (PostgREST-compatible API), and Supabase GoTrue Auth. All components are bundled into one executable that can run without external dependencies.

## Common Commands

### Building and Running
```bash
make build              # Build binary with version info injected
make run                # Build and run
make serve              # Run without building (uses go run)
./supalite serve        # Run built binary
```

### Testing
```bash
make test               # Run all tests
make test-verbose       # Run tests with verbose output
go test ./...           # Direct test execution
go test ./internal/pg/... # Run specific package tests
```

### Database
```bash
make init               # Initialize database (standalone)
./supalite init         # Same with custom flags
make clean              # Clean build artifacts and data directory
```

### GoTrue (Development)
```bash
make install-gotrue     # Install GoTrue binary locally (required for auth)
```

## Architecture

### Component Orchestration

The main server (`internal/server/server.go`) acts as an orchestration layer that:

1. **Starts Embedded PostgreSQL** (`internal/pg/`) - Uses `fergusstrange/embedded-postgres` library. Downloads PostgreSQL 16.x binaries on first run (~100MB). Data persists in the configured `--data-dir` (default: `./data`).

2. **Initializes Database Schemas** - Creates `auth`, `storage`, and `public` schemas required by Supabase tooling.

3. **Starts pREST** (`internal/prest/`) - Provides PostgREST-compatible API at `/rest/v1/*`. Configured via environment variables. Connects directly to embedded PostgreSQL.

4. **Starts GoTrue** (`internal/auth/`) - Supabase auth server running as a subprocess. Provides `/auth/v1/*` endpoints. Requires GoTrue binary installed locally (via `make install-gotrue`).

5. **Proxies Requests** - Main HTTP server (Chi router) routes requests to appropriate services and provides `/health` endpoint.

6. **Graceful Shutdown** - On SIGTERM/SIGINT, shuts down services in reverse order (GoTrue → pREST → PostgreSQL).

### Service Communication

- **Client → Main Server (port 8080)**: All requests go through the main server
- **Main Server → PostgreSQL (port 5432)**: Direct pgx connection
- **Main Server → pREST**: HTTP proxy to `/rest/v1/*`
- **Main Server → GoTrue**: HTTP proxy to `/auth/v1/*`

### Package Structure

```
cmd/
├── root.go          # Cobra root command, version variables (injected at build time)
├── serve.go         # Main serve command - orchestration entry point
└── init.go          # Database initialization command

internal/
├── pg/
│   ├── embedded.go  # Postgres lifecycle (Create, Start, Stop)
│   └── config.go    # Postgres configuration struct
├── auth/
│   ├── server.go    # GoTrue subprocess management
│   └── config.go    # Auth configuration
├── prest/
│   ├── server.go    # pREST server configuration
│   └── config.go    # pREST configuration
├── server/
│   └── server.go    # Chi router, HTTP proxying, orchestration
└── log/
    └── logger.go    # Structured logging wrapper
```

## Configuration

All configuration uses Cobra flags with environment variable fallbacks:

| Flag | Env Var | Default | Purpose |
|------|---------|---------|---------|
| `--host` | `SUPALITE_HOST` | `0.0.0.0` | Server bind address |
| `--port` | `SUPALITE_PORT` | `8080` | Main server port |
| `--data-dir` | `SUPALITE_DATA_DIR` | `./data` | PostgreSQL data directory |
| `--jwt-secret` | `SUPALITE_JWT_SECRET` | (warning) | JWT signing for auth |
| `--site-url` | `SUPALITE_SITE_URL` | `http://localhost:8080` | Auth callback URL |
| `--pg-port` | `SUPALITE_PG_PORT` | `5432` | PostgreSQL port |

Version info (`Version`, `BuildTime`, `GitCommit`) is injected via `-ldflags` at build time - see Makefile for pattern.

## Key Dependencies

- `fergusstrange/embedded-postgres` v1.33.0 - Embedded PostgreSQL
- `prest/prest` v1.5.5 - PostgREST-compatible API
- `spf13/cobra` v1.10.2 - CLI framework
- `jackc/pgx/v5` v5.8.0 - PostgreSQL driver
- `go-chi/chi` v5.x - HTTP router

## Development Notes

- **GoTrue requirement**: The auth server (`internal/auth/server.go`) runs GoTrue as a subprocess. For local development, install via `make install-gotrue` which places the binary in a known location.

- **PostgreSQL download**: First run downloads PostgreSQL binaries. This requires internet connection and ~100MB disk space. Binaries are cached per platform.

- **Schema initialization**: Database schemas are created automatically on first run. Do not manually modify schema initialization without understanding Supabase's schema requirements.

- **Graceful shutdown**: Always test graceful shutdown when modifying server startup code. Services must stop in reverse order to avoid connection errors.

- **Testing**: E2E tests in `e2e/` verify full server startup. Add tests for new orchestration logic.
