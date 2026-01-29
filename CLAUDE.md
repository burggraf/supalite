# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Supalite is a single-binary Go backend that provides Supabase-compatible functionality by orchestrating four services: Embedded PostgreSQL, pREST (PostgREST-compatible API), Supabase GoTrue Auth, and an ES256 JWT Key Manager. All components are bundled into one executable that can run without external dependencies.

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
go test ./internal/keys/... # Test key management
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

3. **Initializes Key Manager** (`internal/keys/`) - Manages JWT signing keys and token generation. Supports both ES256 (asymmetric, default) and HS256 (legacy) modes. Auto-generates anon and service_role keys on first run. Persists keys in `data/keys.json`.

4. **Starts pREST** (`internal/prest/`) - Provides PostgREST-compatible API at `/rest/v1/*`. Configured via environment variables. Connects directly to embedded PostgreSQL.

5. **Starts GoTrue** (`internal/auth/`) - Supabase auth server running as a subprocess. Provides `/auth/v1/*` endpoints. Requires GoTrue binary installed locally (via `make install-gotrue`).

6. **Proxies Requests** - Main HTTP server (Chi router) routes requests to appropriate services and provides `/health` and `/.well-known/jwks.json` endpoints.

7. **Graceful Shutdown** - On SIGTERM/SIGINT, shuts down services in reverse order (GoTrue → pREST → PostgreSQL).

### Service Communication

- **Client → Main Server (port 8080)**: All requests go through the main server
- **Main Server → PostgreSQL (port 5432)**: Direct pgx connection
- **Main Server → pREST**: HTTP proxy to `/rest/v1/*`
- **Main Server → GoTrue**: HTTP proxy to `/auth/v1/*`
- **Main Server → Key Manager**: In-process calls for token generation and JWKS

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
├── keys/
│   └── keys.go      # JWT key management (ES256/HS256), token generation, JWKS
├── server/
│   └── server.go    # Chi router, HTTP proxying, orchestration
└── log/
    └── logger.go    # Structured logging wrapper
```

## Key Management System

The `internal/keys` package provides JWT signing and token generation with support for two modes:

### ES256 Mode (Default)

Asymmetric cryptography using ECDSA P-256 keys:

- **Key Pair**: Generated on first run and persisted in `data/keys.json`
- **Private Key**: Signs JWT tokens (server-only, never exposed)
- **Public Key**: Available via JWKS endpoint at `/.well-known/jwks.json`
- **Tokens**: `anon` (public) and `service_role` (administrative)
- **Lifetime**: 10 years (matching Supabase)
- **Project Ref**: 20-character random string

### Legacy HS256 Mode

Symmetric HMAC-SHA256 signing:

- **Activated**: By providing `--jwt-secret` flag
- **Secret**: Used for both signing and verification
- **Use Case**: Backward compatibility with existing configurations
- **Not Recommended**: For production use

### Key Manager API

```go
// Create key manager (ES256 mode when jwtSecret is empty)
manager, err := keys.NewManager(dataDir, jwtSecret)

// Get generated API keys
anonKey := manager.GetAnonKey()      // Public token
serviceKey := manager.GetServiceKey() // Administrative token

// Get JWKS for public key discovery
jwks, err := manager.GetJWKS() // Returns map[string]interface{}

// Check mode
isLegacy := manager.IsLegacyMode() // true if HS256, false if ES256

// Verify tokens (for future use)
token, err := manager.VerifyToken(tokenString)
```

### Stored Keys Format

`data/keys.json`:
```json
{
  "private_key_pem": "-----BEGIN EC PRIVATE KEY-----\n...",
  "anon_key": "eyJhbGciOiJFUzI1NiIsInR5cCI6IkpXVCJ9...",
  "service_key": "eyJhbGciOiJFUzI1NiIsInR5cCI6IkpXVCJ9...",
  "project_ref": "qd7xe4gnosbcm8053sh6",
  "created_at": "2026-01-28T18:13:55.337051-08:00"
}
```

### Important Constants

- `KeyID`: `"supalite-key-1"` - Used in JWT headers and JWKS
- `TokenLifetime`: `time.Hour * 24 * 365 * 10` - 10 years

### Security Considerations

- Private key in `keys.json` has file permissions `0600`
- ES256 mode is preferred for production (private key never leaves server)
- Legacy mode shares secret for signing/verification (less secure)
- Keys are displayed on startup - save them securely
- `service_role` key bypasses RLS - keep secret!

## Configuration

All configuration uses Cobra flags with environment variable fallbacks:

| Flag | Env Var | Default | Purpose |
|------|---------|---------|---------|
| `--host` | `SUPALITE_HOST` | `0.0.0.0` | Server bind address |
| `--port` | `SUPALITE_PORT` | `8080` | Main server port |
| `--data-dir` | `SUPALITE_DATA_DIR` | `./data` | PostgreSQL and keys directory |
| `--jwt-secret` | `SUPALITE_JWT_SECRET` | (none) | JWT secret for legacy HS256 mode |
| `--site-url` | `SUPALITE_SITE_URL` | `http://localhost:8080` | Auth callback URL |
| `--pg-port` | `SUPALITE_PG_PORT` | `5432` | PostgreSQL port |
| `--anon-key` | `SUPALITE_ANON_KEY` | (auto-generated) | Pre-generated anon key |
| `--service-role-key` | `SUPALITE_SERVICE_ROLE_KEY` | (auto-generated) | Pre-generated service_role key |

**JWT Mode Selection:**
- No `--jwt-secret` provided → ES256 mode (asymmetric, recommended)
- `--jwt-secret` provided → HS256 mode (legacy, backward compatible)

Version info (`Version`, `BuildTime`, `GitCommit`) is injected via `-ldflags` at build time - see Makefile for pattern.

## Key Dependencies

- `fergusstrange/embedded-postgres` v1.33.0 - Embedded PostgreSQL
- `prest/prest` v1.5.5 - PostgREST-compatible API
- `spf13/cobra` v1.10.2 - CLI framework
- `jackc/pgx/v5` v5.8.0 - PostgreSQL driver
- `go-chi/chi` v5.x - HTTP router
- `lestrrat-go/jwx/v2` v2.x - JWT/JWKS library (ES256/HS256 support)

## Development Notes

- **GoTrue requirement**: The auth server (`internal/auth/server.go`) runs GoTrue as a subprocess. For local development, install via `make install-gotrue` which places the binary in a known location.

- **PostgreSQL download**: First run downloads PostgreSQL binaries. This requires internet connection and ~100MB disk space. Binaries are cached per platform.

- **Schema initialization**: Database schemas are created automatically on first run. Do not manually modify schema initialization without understanding Supabase's schema requirements.

- **Key generation**: ES256 keys are auto-generated on first run. Keys persist in `data/keys.json`. To regenerate, delete this file and restart.

- **JWKS endpoint**: Only available in ES256 mode. Returns HTTP 404 in legacy mode. Used by clients to verify JWT signatures.

- **Graceful shutdown**: Always test graceful shutdown when modifying server startup code. Services must stop in reverse order to avoid connection errors.

- **Testing**: E2E tests in `e2e/` verify full server startup. Add tests for new orchestration logic. Unit tests for key management in `internal/keys/`.

- **Mode switching**: Switching between ES256 and HS256 modes requires updating all client tokens (they're not interchangeable).

- **Token verification**: The `VerifyToken` method exists but is not currently used by the server. GoTrue handles token verification for auth flows.

## Integration with Supabase Client Libraries

The generated anon and service_role keys are compatible with Supabase client libraries:

```javascript
import { createClient } from '@supabase/supabase-js'

// Client-side (uses anon key)
const supabase = createClient(
  'http://localhost:8080',
  '<anon-key-from-startup>'
)

// Server-side (uses service_role key)
const adminClient = createClient(
  'http://localhost:8080',
  '<service_role-key-from-startup>',
  { auth: { persistSession: false } }
)
```

ES256 tokens include standard Supabase claims: `iss`, `ref`, `role`, `iat`, `exp`.
