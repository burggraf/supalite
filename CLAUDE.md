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

# Configure email/SMTP settings interactively
./supalite config email
```

### Interactive Email Configuration

The `supalite config email` command provides an interactive wizard for configuring SMTP settings:

```bash
./supalite config email
```

**Features:**
- Prompts for all email configuration fields (SMTP host, port, username, password, admin email, autoconfirm)
- Preserves existing values from `supalite.json` when reconfiguring
- Displays current values as defaults
- Validates configuration and warns about:
  - Missing required fields
  - Unusual SMTP ports (not 25, 465, or 587)
  - Missing authentication credentials
  - Gmail-specific requirements (App Password, full email as username)
  - Autoconfirm mode being enabled
- Shows configuration summary with password masking before saving
- Writes configuration to `supalite.json`

**Sanity Checks:**
- Warns if SMTP host is set but username/password are missing
- Warns about non-standard SMTP ports
- Warns if using Gmail without an App Password
- Warns if admin email is missing (password resets won't work)
- Warns when autoconfirm is enabled (users won't receive confirmation emails)

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
├── serve.go         # Main serve command - config loading, orchestration entry point
└── init.go          # Database initialization command

internal/
├── config/
│   └── config.go    # Configuration loader (file + env + flags)
├── pg/
│   ├── embedded.go  # Postgres lifecycle (Create, Start, Stop)
│   └── config.go    # Postgres configuration struct
├── auth/
│   ├── server.go    # GoTrue subprocess management
│   └── config.go    # Auth configuration (including email)
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

Supalite supports three methods for configuration, applied in the following priority order:

1. **Command-line flags** (highest priority)
2. **`supalite.json` file** (if it exists)
3. **Environment variables** (fallback)
4. **Default values** (lowest priority)

### Configuration File

Create a `supalite.json` file in the working directory (see `supalite.example.json` for a template):

```json
{
  "host": "0.0.0.0",
  "port": 8080,
  "site_url": "http://localhost:8080",
  "data_dir": "./data",
  "pg_port": 5432,
  "pg_username": "postgres",
  "pg_password": "postgres",
  "pg_database": "postgres",
  "email": {
    "smtp_host": "smtp.gmail.com",
    "smtp_port": 587,
    "smtp_user": "your-email@gmail.com",
    "smtp_pass": "your-app-password",
    "smtp_admin_email": "admin@yourdomain.com",
    "mailer_autoconfirm": false
  }
}
```

**Security Note**: `supalite.json` is listed in `.gitignore` to prevent committing secrets. Use `supalite.example.json` as a template for version control.

### Server Configuration Options

| Flag | Env Var | Default | Purpose |
|------|---------|---------|---------|
| `--host` | `SUPALITE_HOST` | `0.0.0.0` | Server bind address |
| `--port` | `SUPALITE_PORT` | `8080` | Main server port |
| `--data-dir` | `SUPALITE_DATA_DIR` | `./data` | PostgreSQL and keys directory |
| `--jwt-secret` | `SUPALITE_JWT_SECRET` | (none) | JWT secret for legacy HS256 mode |
| `--site-url` | `SUPALITE_SITE_URL` | `http://localhost:8080` | Auth callback URL |
| `--pg-port` | `SUPALITE_PG_PORT` | `5432` | PostgreSQL port |
| `--pg-username` | `SUPALITE_PG_USERNAME` | `postgres` | PostgreSQL username |
| `--pg-password` | `SUPALITE_PG_PASSWORD` | `postgres` | PostgreSQL password |
| `--pg-database` | `SUPALITE_PG_DATABASE` | `postgres` | PostgreSQL database name |
| `--anon-key` | `SUPALITE_ANON_KEY` | (auto-generated) | Pre-generated anon key |
| `--service-role-key` | `SUPALITE_SERVICE_ROLE_KEY` | (auto-generated) | Pre-generated service_role key |

### Email Configuration

GoTrue handles email sending for user authentication flows (email confirmation, password reset, etc.). Email is optional - if not configured, users can still sign up but email confirmation will be skipped (autoconfirm mode).

| Flag | Env Var | Default | Purpose |
|------|---------|---------|---------|
| `--smtp-host` | `SUPALITE_SMTP_HOST` | (none) | SMTP server hostname |
| `--smtp-port` | `SUPALITE_SMTP_PORT` | (none) | SMTP server port (typically 587 for TLS) |
| `--smtp-user` | `SUPALITE_SMTP_USER` | (none) | SMTP username |
| `--smtp-pass` | `SUPALITE_SMTP_PASS` | (none) | SMTP password |
| `--smtp-admin-email` | `SUPALITE_SMTP_ADMIN_EMAIL` | (none) | Admin email for password resets |
| `--mailer-autoconfirm` | `SUPALITE_MAILER_AUTOCONFIRM` | `false` | Skip email confirmation for new users |
| `--mailer-urlpaths-invite` | `SUPALITE_MAILER_URLPATHS_INVITE` | `/auth/v1/verify` | Invite email URL path |
| `--mailer-urlpaths-confirmation` | `SUPALITE_MAILER_URLPATHS_CONFIRMATION` | `/auth/v1/verify` | Confirmation email URL path |
| `--mailer-urlpaths-recovery` | `SUPALITE_MAILER_URLPATHS_RECOVERY` | `/auth/v1/verify` | Recovery email URL path |
| `--mailer-urlpaths-email-change` | `SUPALITE_MAILER_URLPATHS_EMAIL_CHANGE` | `/auth/v1/verify` | Email change URL path |

### Email Examples

**Using Gmail (requires App Password):**
```json
{
  "email": {
    "smtp_host": "smtp.gmail.com",
    "smtp_port": 587,
    "smtp_user": "your-email@gmail.com",
    "smtp_pass": "your-16-char-app-password",
    "smtp_admin_email": "your-email@gmail.com"
  }
}
```

**Using Mailgun:**
```json
{
  "email": {
    "smtp_host": "smtp.mailgun.org",
    "smtp_port": 587,
    "smtp_user": "postmaster@mg.yourdomain.com",
    "smtp_pass": "your-mailgun-password",
    "smtp_admin_email": "admin@yourdomain.com"
  }
}
```

**Using AWS SES:**
```json
{
  "email": {
    "smtp_host": "email-smtp.us-east-1.amazonaws.com",
    "smtp_port": 587,
    "smtp_user": "your-ses-smtp-username",
    "smtp_pass": "your-ses-smtp-password",
    "smtp_admin_email": "noreply@yourdomain.com"
  }
}
```

**Development (skip email confirmation):**
```json
{
  "email": {
    "mailer_autoconfirm": true
  }
}
```

### JWT Mode Selection

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

- **Email configuration**: GoTrue handles all email sending internally. Supalite passes email configuration via environment variables (`GOTRUE_SMTP_*`). Email is optional - without it, GoTrue runs in autoconfirm mode where users are immediately verified.

- **Config loading**: The `internal/config` package loads configuration in priority order: flags > `supalite.json` > environment variables > defaults. This allows flexible deployment scenarios (development with file, production with env vars, overrides with flags).

- **Configuration security**: `supalite.json` is in `.gitignore` because it may contain sensitive credentials (SMTP passwords, API keys). Use `supalite.example.json` for documentation in version control.

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
