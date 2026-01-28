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
- **Zero Configuration** - Works out of the box with sensible defaults
- **Fast Startup** - Sub-second startup time, suitable for serverless/edge deployments
- **Local Development** - Perfect for local development and testing before deploying to Supabase

## Quick Start

### Build and Run

```bash
# Build the binary
make build
# Or: go build -o supalite .

# Run the server (auto-creates database)
./supalite serve

# With custom port
./supalite serve --port 3000

# With custom data directory
./supalite serve --data-dir ./mydata

# With custom JWT secret
./supalite serve --jwt-secret "my-secret-key"

# With custom site URL (for auth callbacks)
./supalite serve --site-url "http://localhost:3000"
```

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

## APIs

Once the server is running, the following APIs are available:

### Auth API (`/auth/v1/*`)

Powered by GoTrue, providing Supabase-compatible authentication:

```bash
# Sign up a new user
curl -X POST http://localhost:8080/auth/v1/signup \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"password123"}'

# Sign in with email/password
curl -X POST http://localhost:8080/auth/v1/token?grant_type=password \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"password123"}'

# Get current user
curl -X GET http://localhost:8080/auth/v1/user \
  -H "Authorization: Bearer <access-token>"

# Sign out
curl -X POST http://localhost:8080/auth/v1/logout \
  -H "Authorization: Bearer <access-token>"
```

### REST API (`/rest/v1/*`)

Powered by pREST, providing PostgREST-compatible database access:

```bash
# List all tables
curl http://localhost:8080/rest/v1/

# Query a table
curl http://localhost:8080/rest/v1/users

# Create a row
curl -X POST http://localhost:8080/rest/v1/users \
  -H "Content-Type: application/json" \
  -H "apikey: <your-api-key>" \
  -d '{"email":"user@example.com","name":"John Doe"}'

# Update a row
curl -X PATCH http://localhost:8080/rest/v1/users?id=eq.1 \
  -H "Content-Type: application/json" \
  -H "apikey: <your-api-key>" \
  -d '{"name":"Jane Doe"}'

# Delete a row
curl -X DELETE http://localhost:8080/rest/v1/users?id=eq.1 \
  -H "apikey: <your-api-key>"
```

### Health Check

```bash
curl http://localhost:8080/health
# Returns: {"status":"healthy"}
```

## Configuration

Configuration is available via command-line flags or environment variables.

| Command-Line Flag | Environment Variable | Default | Description |
|-------------------|---------------------|---------|-------------|
| `--host` | `SUPALITE_HOST` | `0.0.0.0` | Host to bind to |
| `--port` | `SUPALITE_PORT` | `8080` | API server port |
| `--data-dir` | `SUPALITE_DATA_DIR` | `./data` | Data directory for PostgreSQL |
| `--jwt-secret` | `SUPALITE_JWT_SECRET` | (warning) | JWT signing secret |
| `--site-url` | `SUPALITE_SITE_URL` | `http://localhost:8080` | Site URL for auth callbacks |

### Init Command Options

| Command-Line Flag | Default | Description |
|-------------------|---------|-------------|
| `--db` | `./data` | Data directory for PostgreSQL |
| `--port` | `5432` | Embedded PostgreSQL port |
| `--username` | `postgres` | Database username |
| `--password` | `postgres` | Database password |
| `--database` | `postgres` | Database name |
| `--pg-version` | `16.9.0` | PostgreSQL version to download |

## Requirements

- **Go 1.23+** - For building from source
- **Operating System** - Linux, macOS, or Windows
- **Architecture** - amd64, arm64 (Apple Silicon), or i386
- **Network** - Internet connection required for first run (downloads PostgreSQL binaries)
- **Disk Space** - ~100MB for PostgreSQL binaries

## Architecture

Supalite orchestrates three main components:

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
└───────────────────────────────────────────────┼──────────────┘
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

3. **pREST Server**
   - PostgREST-compatible API server
   - Managed by the `internal/prest` package
   - Provides `/rest/v1/*` endpoints
   - Direct database access with REST semantics

## Development

### Project Structure

```
supalite/
├── main.go                 # Entry point
├── cmd/                    # CLI commands
│   ├── root.go            # Root command, version variables
│   ├── init.go            # Database initialization
│   └── serve.go           # Server orchestration
├── internal/
│   ├── pg/                # Embedded PostgreSQL management
│   ├── auth/              # GoTrue auth server wrapper
│   ├── prest/             # pREST server wrapper
│   ├── server/            # Main HTTP server
│   └── log/               # Logging utilities
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
```

## License

MIT License - See LICENSE file for details
