# Supalite

A lightweight, single-binary backend providing Supabase-compatible functionality using:
- Embedded PostgreSQL (no external database required)
- pREST for PostgREST-compatible REST API
- Supabase Auth (GoTrue) for authentication

## Quick Start

```bash
# Build
go build -o supalite .

# Run (auto-creates database)
./supalite serve

# With custom port
./supalite serve --port 3000
```

## APIs

- **Auth:** `http://localhost:8080/auth/v1/*` (GoTrue)
- **REST:** `http://localhost:8080/rest/v1/*` (pREST)

## License

MIT
