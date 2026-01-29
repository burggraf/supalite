# Supalite

A lightweight, single-binary backend providing Supabase-compatible functionality using:
- **Embedded PostgreSQL** (no external database required)
- **pREST** for PostgREST-compatible REST API
- **Supabase Auth (GoTrue)** for authentication

## Features

- **Single Binary Deployment** - No external dependencies, distributed as one executable
- **Embedded PostgreSQL** - Full PostgreSQL 16.x database embedded in the binary
- **Supabase-Compatible APIs** - Drop-in replacement for many Supabase use cases
- **Auth API** - Full Supabase Auth compatibility via GoTrue (`/auth/v1/*`)
- **REST API** - PostgREST-compatible API for direct database access (`/rest/v1/*`)
- **ES256 JWT Signing** - Modern asymmetric key cryptography for API tokens (default)
- **Legacy HS256 Support** - Backward compatible with JWT_SECRET configuration
- **JWKS Endpoint** - Public key discovery for JWT verification (`/.well-known/jwks.json`)
- **Zero Configuration** - Works out of the box with sensible defaults
- **Fast Startup** - Sub-second startup time, suitable for serverless/edge deployments
- **Local Development** - Perfect for local development and testing before deploying to Supabase

## Quick Start

### Build and Run

```bash
# Build the binary
make build
# Or: go build -o supalite .

# Configure email/SMTP (optional, for user auth emails)
./supalite config email

# Run the server (auto-creates database and generates API keys)
./supalite serve

# With custom port
./supalite serve --port 3000

# With custom data directory
./supalite serve --data-dir ./mydata

# With legacy JWT secret (HS256 mode)
./supalite serve --jwt-secret "my-secret-key"

# With custom site URL (for auth callbacks)
./supalite serve --site-url "http://localhost:3000"
```

### First Run - API Keys

On first run, Supalite automatically generates ES256 key pairs and displays your API keys:

```
==========================================
Project API Keys
==========================================
Project URL: http://localhost:8080

anon key (public):
  eyJhbGciOiJFUzI1NiIsInR5cCI6IkpXVCJ9...

service_role key (secret - keep hidden!):
  eyJhbGciOiJFUzI1NiIsInR5cCI6IkpXVCJ9...
==========================================
```

**Important:** Save these keys! They are persisted in `./data/keys.json` and reused on subsequent runs.

### Using the Makefile

```bash
# Build the binary
make build

# Build and run
make run

# Run tests
make test

# Clean build artifacts and data
make clean

# Initialize database (standalone)
make init

# Run without building (uses go run)
make serve

# Install GoTrue (for development)
make install-gotrue
```

### Manual Database Initialization

If you want to initialize the database separately:

```bash
./supalite init

# With custom configuration
./supalite init \
  --db ./mydata \
  --port 5432 \
  --username myuser \
  --password mypass \
  --database mydb \
  --pg-version 16.9.0
```

## Authentication & API Keys

Supalite supports two authentication modes for JWT signing:

### ES256 Mode (Default)

Modern asymmetric cryptography using ECDSA P-256 keys:

- **Private key**: Signs JWT tokens (stored securely on server)
- **Public key**: Verifies JWT tokens (available via JWKS endpoint)
- **More secure**: Private key never leaves the server
- **Supabase compatible**: Matches Supabase's future authentication direction

**Auto-generated on first run:**
- ES256 key pair generated and saved to `data/keys.json`
- `anon` key (public token) for client-side use
- `service_role` key (secret token) for server-side use
- 10-year token lifetime (matching Supabase)

**JWKS Endpoint:**
```bash
curl http://localhost:8080/.well-known/jwks.json
```

Returns the public key in standard JWKS format for JWT verification.

### Legacy HS256 Mode

Symmetric HMAC-SHA256 signing using `JWT_SECRET`:

- Activated by providing `--jwt-secret` flag
- Backward compatible with existing configurations
- Uses same secret for signing and verification

**Not recommended for production** - migrate to ES256 when possible.

### Using API Keys

Use the `anon` key in your client applications:

```javascript
import { createClient } from '@supabase/supabase-js'

const supabase = createClient(
  'http://localhost:8080',
  'eyJhbGciOiJFUzI1NiIsInR5cCI6IkpXVCJ9...' // your anon key
)

// Use the client
const { data, error } = await supabase
  .from('users')
  .select('*')
```

Use the `service_role` key for administrative operations (server-side only):

```javascript
import { createClient } from '@supabase/supabase-js'

const supabase = createClient(
  'http://localhost:8080',
  'eyJhbGciOiJFUzI1NiIsInR5cCI6IkpXVCJ9...', // your service_role key
  {
    auth: { persistSession: false }
  }
)

// Bypasses RLS policies - use with caution!
const { data, error } = await supabase
  .from('users')
  .select('*')
```

### Token Structure

Tokens include standard Supabase claims:

```json
{
  "iss": "supabase",
  "ref": "qd7xe4gnosbcm8053sh6",
  "role": "anon",
  "iat": 1769652835,
  "exp": 2085012835
}
```

- `iss`: Issuer (always "supabase")
- `ref`: Project reference (20-character random string)
- `role`: Token role (`anon` or `service_role`)
- `iat`: Issued at timestamp
- `exp`: Expiration timestamp (10 years from issuance)

## APIs

Once the server is running, the following APIs are available:

### Auth API (`/auth/v1/*`)

Powered by GoTrue, providing Supabase-compatible authentication:

```bash
# Sign up a new user
curl -X POST http://localhost:8080/auth/v1/signup \
  -H "Content-Type: application/json" \
  -H "apikey: <your-anon-key>" \
  -d '{"email":"user@example.com","password":"password123"}'

# Sign in with email/password
curl -X POST http://localhost:8080/auth/v1/token?grant_type=password \
  -H "Content-Type: application/json" \
  -H "apikey: <your-anon-key>" \
  -d '{"email":"user@example.com","password":"password123"}'

# Get current user
curl -X GET http://localhost:8080/auth/v1/user \
  -H "Authorization: Bearer <access-token>" \
  -H "apikey: <your-anon-key>"

# Sign out
curl -X POST http://localhost:8080/auth/v1/logout \
  -H "Authorization: Bearer <access-token>" \
  -H "apikey: <your-anon-key>"
```

### REST API (`/rest/v1/*`)

Powered by pREST, providing PostgREST-compatible database access:

```bash
# List all tables
curl http://localhost:8080/rest/v1/ \
  -H "apikey: <your-anon-key>"

# Query a table
curl http://localhost:8080/rest/v1/users \
  -H "apikey: <your-anon-key>"

# Create a row
curl -X POST http://localhost:8080/rest/v1/users \
  -H "Content-Type: application/json" \
  -H "apikey: <your-anon-key>" \
  -d '{"email":"user@example.com","name":"John Doe"}'

# Update a row
curl -X PATCH http://localhost:8080/rest/v1/users?id=eq.1 \
  -H "Content-Type: application/json" \
  -H "apikey: <your-anon-key>" \
  -d '{"name":"Jane Doe"}'

# Delete a row
curl -X DELETE http://localhost:8080/rest/v1/users?id=eq.1 \
  -H "apikey: <your-anon-key>"
```

### JWKS Endpoint (`/.well-known/jwks.json`)

Public key discovery for ES256 mode:

```bash
curl http://localhost:8080/.well-known/jwks.json
```

**Response (ES256 mode):**
```json
{
  "keys": [
    {
      "kty": "EC",
      "kid": "supalite-key-1",
      "use": "sig",
      "alg": "ES256",
      "crv": "P-256",
      "x": "ohEEm7BlSPqVGsIuVPL12AmhnNlJuPAD-KGwJs26kHI",
      "y": "PGuAtrwXBPwaCeRXqvpYFKNw-zOE7IfICBRK9dLrYno"
    }
  ]
}
```

Use this endpoint to verify JWT signatures in your applications.

### Health Check

```bash
curl http://localhost:8080/health
# Returns: {"status":"healthy"}
```

## Configuration

Supalite supports three methods for configuration, applied in the following priority order:

1. **Command-line flags** (highest priority)
2. **`supalite.json` file** (if it exists in working directory)
3. **Environment variables** (fallback)
4. **Default values** (lowest priority)

### Configuration File

Create a `supalite.json` file in your working directory (see `supalite.example.json` for a template):

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

### Server Configuration

| Command-Line Flag | Environment Variable | Default | Description |
|-------------------|---------------------|---------|-------------|
| `--host` | `SUPALITE_HOST` | `0.0.0.0` | Host to bind to |
| `--port` | `SUPALITE_PORT` | `8080` | API server port |
| `--data-dir` | `SUPALITE_DATA_DIR` | `./data` | Data directory for PostgreSQL and keys |
| `--jwt-secret` | `SUPALITE_JWT_SECRET` | (none) | JWT secret for legacy HS256 mode |
| `--site-url` | `SUPALITE_SITE_URL` | `http://localhost:8080` | Site URL for auth callbacks |
| `--anon-key` | `SUPALITE_ANON_KEY` | (auto-generated) | Pre-generated anon key |
| `--service-role-key` | `SUPALITE_SERVICE_ROLE_KEY` | (auto-generated) | Pre-generated service_role key |

### Database Configuration

| Command-Line Flag | Environment Variable | Default | Description |
|-------------------|---------------------|---------|-------------|
| `--pg-port` | `SUPALITE_PG_PORT` | `5432` | Embedded PostgreSQL port |
| `--pg-username` | `SUPALITE_PG_USERNAME` | `postgres` | PostgreSQL username |
| `--pg-password` | `SUPALITE_PG_PASSWORD` | `postgres` | PostgreSQL password |
| `--pg-database` | `SUPALITE_PG_DATABASE` | `postgres` | PostgreSQL database name |

### Email Configuration

GoTrue handles email sending for authentication flows (email confirmation, password reset, etc.). Email is **optional** - if not configured, users can still sign up but email confirmation will be skipped (autoconfirm mode).

| Command-Line Flag | Environment Variable | Default | Description |
|-------------------|---------------------|---------|-------------|
| `--smtp-host` | `SUPALITE_SMTP_HOST` | (none) | SMTP server hostname |
| `--smtp-port` | `SUPALITE_SMTP_PORT` | (none) | SMTP server port (typically 587 for TLS) |
| `--smtp-user` | `SUPALITE_SMTP_USER` | (none) | SMTP username |
| `--smtp-pass` | `SUPALITE_SMTP_PASS` | (none) | SMTP password |
| `--smtp-admin-email` | `SUPALITE_SMTP_ADMIN_EMAIL` | (none) | Admin email for password resets |
| `--mailer-autoconfirm` | `SUPALITE_MAILER_AUTOCONFIRM` | `false` | Skip email confirmation for new users |

#### Email Examples

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

### Init Command Options

| Command-Line Flag | Default | Description |
|-------------------|---------|-------------|
| `--db` | `./data` | Data directory for PostgreSQL |
| `--port` | `5432` | Embedded PostgreSQL port |
| `--username` | `postgres` | Database username |
| `--password` | `postgres` | Database password |
| `--database` | `postgres` | Database name |
| `--pg-version` | `16.9.0` | PostgreSQL version to download |

## Key Storage

Keys are persisted in `data/keys.json`:

```json
{
  "private_key_pem": "-----BEGIN EC PRIVATE KEY-----\nMHcCAQEE...",
  "anon_key": "eyJhbGciOiJFUzI1NiIsInR5cCI6IkpXVCJ9...",
  "service_key": "eyJhbGciOiJFUzI1NiIsInR5cCI6IkpXVCJ9...",
  "project_ref": "qd7xe4gnosbcm8053sh6",
  "created_at": "2026-01-28T18:13:55.337051-08:00"
}
```

**Security notes:**
- `private_key_pem`: The ES256 private key - **keep this secret**
- `anon_key`: Public token - safe to share with clients
- `service_key`: Administrative token - **keep this secret**
- File permissions are set to `0600` (owner read/write only)

## Migration from Legacy Mode

If you're currently using `--jwt-secret` (legacy HS256 mode):

1. **Stop using `--jwt-secret`** to enable ES256 mode
2. **Generate new keys** - they'll be created automatically on next run
3. **Update your clients** with the new `anon` and `service_role` keys
4. **Verify JWKS endpoint** works at `/.well-known/jwks.json`

**Note:** ES256 and HS256 tokens are not interchangeable. Update all clients when switching modes.

## Requirements

- **Go 1.23+** - For building from source
- **Operating System** - Linux, macOS, or Windows
- **Architecture** - amd64, arm64 (Apple Silicon), or i386
- **Network** - Internet connection required for first run (downloads PostgreSQL binaries)
- **Disk Space** - ~100MB for PostgreSQL binaries

## Architecture

Supalite orchestrates four main components:

```
┌─────────────────────────────────────────────────────────────┐
│                      Supalite Server                         │
│                      (localhost:8080)                        │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │   Health     │  │   Auth API   │  │   REST API   │      │
│  │   /health    │  │   /auth/v1   │  │   /rest/v1   │      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
│                                               │              │
│  ┌──────────────────────────────────────────────────────┐   │
│  │              JWKS Endpoint                           │   │
│  │        /.well-known/jwks.json                        │   │
│  └──────────────────────────────────────────────────────┘   │
└───────────────────────────────────────────────┬──────────────┘
                                                │
                    ┌───────────────────────────┘
                    │
                    ▼
        ┌───────────────────────────┐
        │  Embedded PostgreSQL      │
        │  (localhost:5432)         │
        │  - Version: 16.9.0        │
        │  - Auth Schema            │
        │  - Storage Schema         │
        │  - Public Schema          │
        └───────────────────────────┘
                    │
                    ▼
        ┌───────────────────────────┐
        │  File System              │
        │  ./data/                  │
        │  ├── pg/                  │
        │  ├── keys.json            │
        │  └── postgres.conf        │
        └───────────────────────────┘
```

### Components

1. **Embedded PostgreSQL**
   - Full PostgreSQL 16.x database
   - Managed by the `internal/pg` package
   - Downloads binaries on first run
   - Stores data in `--data-dir`

2. **GoTrue Auth Server**
   - Supabase's authentication server
   - Managed by the `internal/auth` package
   - Provides `/auth/v1/*` endpoints
   - Supports email/password authentication
   - Uses ES256 or HS256 for token signing

3. **pREST Server**
   - PostgREST-compatible API server
   - Managed by the `internal/prest` package
   - Provides `/rest/v1/*` endpoints
   - Direct database access with REST semantics

4. **Key Manager**
   - ES256 key pair generation and storage
   - Managed by the `internal/keys` package
   - Provides JWKS endpoint for public key discovery
   - Auto-generates anon and service_role tokens
   - Supports both ES256 (default) and HS256 (legacy) modes

## Development

### Project Structure

```
supalite/
├── main.go                 # Entry point
├── cmd/                    # CLI commands
│   ├── root.go            # Root command, version variables
│   ├── init.go            # Database initialization
│   └── serve.go           # Server orchestration & config loading
├── internal/
│   ├── config/            # Configuration loader (file + env + flags)
│   ├── pg/                # Embedded PostgreSQL management
│   ├── auth/              # GoTrue auth server wrapper
│   ├── prest/             # pREST server wrapper
│   ├── keys/              # JWT key management (ES256/HS256)
│   ├── server/            # Main HTTP server
│   └── log/               # Logging utilities
├── docs/                  # Documentation
└── e2e/                   # End-to-end tests
```

### Building with Version Info

The Makefile automatically injects version information:

```makefile
VERSION=1.0.0
BUILD_TIME=2025-01-28T12:00:00Z
GIT_COMMIT=abc1234

go build -ldflags \
  "-X github.com/markb/supalite/cmd.Version=$(VERSION)" \
  "-X github.com/markb/supalite/cmd.BuildTime=$(BUILD_TIME)" \
  "-X github.com/markb/supalite/cmd.GitCommit=$(GIT_COMMIT)" \
  -o supalite .
```

This information is available via `./supalite version`.

### Running Tests

```bash
# Run all tests
make test

# Run tests with verbose output
make test-verbose

# Run specific package tests
go test ./internal/pg/...
go test ./internal/auth/...
go test ./internal/keys/...
```

## License

MIT License - See LICENSE file for details
