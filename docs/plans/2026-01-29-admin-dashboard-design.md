# Admin Dashboard Design

**Date:** 2026-01-29
**Status:** Approved for Implementation

## Overview

Create an admin dashboard for Supalite accessible at `http://localhost:8080/_/` that provides basic Supabase dashboard functionality. The dashboard will be embedded in the Supalite binary using Go's `embed` package.

## Architecture

### Technology Stack

- **Frontend:** React + TypeScript + Vite
- **UI Framework:** ShadCN UI + Tailwind CSS
- **Backend:** Go with embedded filesystem
- **Auth:** Custom admin user system with JWT tokens

### Directory Structure

```
supalite/
├── dashboard/               # Frontend project
│   ├── src/
│   │   ├── components/     # ShadCN components
│   │   ├── pages/         # Dashboard pages
│   │   ├── lib/           # API client, utilities
│   │   └── main.tsx
│   ├── index.html
│   ├── vite.config.ts
│   ├── package.json
│   └── tailwind.config.js
├── internal/
│   ├── admin/             # Admin user management
│   │   ├── password.go    # Password hashing (bcrypt)
│   │   └── user.go        # User CRUD operations
│   └── dashboard/         # Dashboard serving
│       ├── embed.go       # Embedded filesystem
│       ├── server.go      # Dashboard HTTP routes
│       └── api.go         # Dashboard API endpoints
└── Makefile
```

## Authentication System

### Database Schema

```sql
CREATE SCHEMA IF NOT EXISTS admin;

CREATE TABLE IF NOT EXISTS admin.users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS admin_users_email_idx
    ON admin.users(email);
```

### CLI Commands

```bash
# Create first admin during init (prompted if no admin users exist)
./supalite init

# Add a new admin user
./supalite admin add <email>

# Change admin password
./supalite admin change-password <email>

# Delete admin user
./supalite admin delete <email>

# List admin users
./supalite admin list
```

### Authentication Flow

1. User navigates to `/_/` and sees login page
2. Enters email/password
3. POST to `/_/api/login` with credentials
4. Server verifies hash, returns JWT token
5. Token stored in sessionStorage
6. Subsequent requests include `Authorization: Bearer <token>`
7. Token expiry: 24 hours

### JWT Token Structure

```json
{
  "iss": "supalite",
  "sub": "<user-id>",
  "email": "admin@example.com",
  "role": "admin",
  "iat": 1234567890,
  "exp": 1234567890 + 86400
}
```

## Dashboard Pages

### 1. Login Page (`/_/login`)
- Email/password form
- Error handling for invalid credentials
- Stores JWT token on success

### 2. Overview (`_/`)
- System status indicators (PostgreSQL, pREST, GoTrue, Mail Capture)
- API keys display (anon, service_role) with copy buttons
- Quick stats: table count, database size
- Database connection info

### 3. Table Editor (`/_/tables`)
- List all tables in `public` schema
- View table data (paginated)
- Inline editing, add/delete rows
- Filter and sort
- SQL query editor

### 4. Auth Management (`/_/auth`)
- List users from `auth.users`
- View user details
- Toggle email verification
- Email configuration display
- Test email sending

### 5. API Keys (`/_/api-keys`)
- Display anon and service_role keys
- Show JWKS public key
- Copy-to-clipboard
- Regenerate keys (with confirmation)

### 6. Settings (`/_/settings`)
- Server configuration overview
- Data directory location
- Port bindings
- JWT mode indicator
- Restart server button

## Backend API Endpoints

### Auth Endpoints
- `POST /_/api/login` - Authenticate and return JWT
- `GET /_/api/me` - Get current user info (authenticated)

### Dashboard Endpoints (authenticated)
- `GET /_/api/status` - System status
- `GET /_/api/tables` - List all tables
- `GET /_/api/tables/{name}` - Get table schema
- `GET /_/api/admin/users` - List admin users

### Proxied Endpoints
- `/rest/v1/*` - Proxied to pREST with admin auth
- `/auth/v1/*` - Proxied to GoTrue (for viewing auth users)

## Development Workflow

### Development Mode (Two Processes)

```bash
# Terminal 1: Go backend
make serve  # Runs on http://localhost:8080

# Terminal 2: Vite dev server
cd dashboard
npm run dev  # Runs on http://localhost:5173
```

Access dashboard at `http://localhost:5173` during development.

### Production Build

```bash
make build  # Builds dashboard then Go binary
./supalite serve  # Dashboard embedded at http://localhost:8080/_/
```

### Vite Proxy Configuration

```typescript
server: {
  port: 5173,
  proxy: {
    '/rest': 'http://localhost:8080',
    '/auth': 'http://localhost:8080',
    '/_/api': 'http://localhost:8080',
  }
}
```

## Implementation Phases

### Phase 1: Foundation
- [ ] Create `dashboard/` directory with Vite+React+Tailwind
- [ ] Initialize ShadCN UI
- [ ] Create basic routing layout
- [ ] Set up `internal/admin/` package
- [ ] Create `internal/dashboard/` package with embed support

### Phase 2: Authentication
- [ ] Implement admin.users table in schema init
- [ ] Create password hashing utilities
- [ ] Add CLI commands (init prompt, admin add/change-password/delete/list)
- [ ] Implement login API endpoint
- [ ] Build login page frontend
- [ ] Add JWT middleware for protected routes

### Phase 3: Core Pages
- [ ] Overview page (system status)
- [ ] Table browser (list tables, view data)
- [ ] Basic table operations (CRUD)

### Phase 4: Advanced Features
- [ ] Auth management page
- [ ] API keys page
- [ ] Settings page
- [ ] SQL query editor

### Phase 5: Polish
- [ ] Error handling, loading states
- [ ] Responsive design
- [ ] Tests

## Security Considerations

1. **Password Hashing:** Use bcrypt with default cost factor
2. **JWT Tokens:** 24-hour expiry, signed with existing key manager
3. **HTTPS:** Recommend using reverse proxy (nginx/caddy) in production
4. **CORS:** Dashboard served from same origin as API, no CORS needed
5. **SQL Injection:** Use parameterized queries via pgx

## Dependencies

### Go
- `github.com/go-chi/chi/v5` (already in use)
- `golang.org/x/crypto/bcrypt` (password hashing)

### Frontend
```json
{
  "dependencies": {
    "react": "^18.3.1",
    "react-dom": "^18.3.1",
    "react-router-dom": "^6.22.0",
    "@tanstack/react-query": "^5.0.0"
  },
  "devDependencies": {
    "@vitejs/plugin-react": "^4.2.1",
    "vite": "^5.1.0",
    "typescript": "^5.3.0",
    "tailwindcss": "^3.4.0"
  }
}
```

## Binary Size Impact

- Dashboard build: ~400-800 KB (gzipped)
- Supalite binary: ~50MB+ (embedded PostgreSQL)
- Relative impact: <2% increase
