# Supalite Admin Dashboard

The Supalite admin dashboard provides a web-based interface for managing your Supalite instance, viewing database tables, monitoring system status, and managing authentication users.

## Quick Start

### Accessing the Dashboard

Once your Supalite server is running, access the dashboard at:

```
http://localhost:8080/_/
```

The dashboard is embedded directly in the Supalite binary, so no separate installation or setup is required.

### First Time Setup

When you first initialize Supalite, you'll be prompted to create an admin user:

```bash
./supalite init
```

You'll see:

```
===========================================
Create First Admin User
===========================================

No admin users found. Let's create the first admin user.

Email: admin@example.com
Password: ********
Confirm password: ********

✓ First admin user created successfully!
  Email: admin@example.com
```

Use these credentials to log in to the dashboard at `http://localhost:8080/_/`.

## Development vs Production

### Development Mode

During active development of the dashboard itself, you can run the frontend separately with hot-reload:

```bash
# Terminal 1: Start the Supalite backend
make serve
# Or: ./supalite serve

# Terminal 2: Start the Vite dev server
cd dashboard
npm install  # First time only
npm run dev
```

In development mode:
- Access the dashboard at `http://localhost:5173`
- Vite provides hot module replacement (HMR)
- API requests are proxied to the backend at `http://localhost:8080`
- Changes to frontend code are reflected immediately

**Vite Proxy Configuration:**

The `dashboard/vite.config.ts` file proxies API requests to the backend:

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

### Production Mode

For production deployment, the dashboard is embedded in the binary:

```bash
# Build the dashboard (first time)
make build-dashboard

# Build the complete binary
make build

# Run the server
./supalite serve
```

In production mode:
- Dashboard is embedded at `http://localhost:8080/_/`
- Served directly from the binary using Go's `embed` package
- No separate frontend server needed
- Single binary deployment

**Building from Scratch:**

```bash
# 1. Install frontend dependencies
cd dashboard
npm install

# 2. Build dashboard for production
npm run build

# 3. Copy dashboard to internal/dashboard for embedding
cd ..
rm -rf internal/dashboard/dist
cp -r dashboard/dist internal/dashboard/dist

# 4. Build Go binary
go build -o supalite .

# Or use the Makefile shortcut:
make build
```

## Dashboard Features

### Overview Page

The overview page displays:
- **System Status**: Health of PostgreSQL, pREST, GoTrue, and mail capture
- **API Keys**: Your anon and service_role keys with copy buttons
- **Database Info**: Connection details, table count, and data directory
- **Quick Stats**: Database size, user count, and other metrics

### Tables Page

Browse and manage your database tables:
- List all tables in the `public` schema
- View table data with pagination
- Inspect table schemas (columns, types, constraints)
- Coming soon: Inline editing, filtering, and SQL query editor

### Authentication

View authentication-related information:
- GoTrue server status and configuration
- Email/SMTP settings (masked for security)
- Mail capture mode status
- Coming soon: User management from auth.users table

### Settings

View and manage server configuration:
- Server host and port
- Data directory location
- PostgreSQL connection details
- JWT mode (ES256 or HS256)
- Email configuration summary

## Managing Admin Users

Admin users are stored in the `admin.users` table and managed via CLI commands.

### List Admin Users

```bash
./supalite admin list
```

Output:

```
===========================================
Admin Users
===========================================

Found 1 admin user(s):

1. admin@example.com
   ID: 550e8400-e29b-41d4-a716-446655440000
   Created: 2026-01-29T10:30:00Z
   Updated: 2026-01-29T10:30:00Z
```

### Add a New Admin User

```bash
./supalite admin add
```

You'll be prompted for:
- Email address
- Password (with confirmation)

### Change Admin Password

```bash
./supalite admin change-password
```

You'll be prompted for:
- Email address
- New password (with confirmation)

### Delete an Admin User

```bash
./supalite admin delete
```

You'll be prompted for:
- Email address
- Confirmation (type "y" or "yes")

**Warning:** You cannot delete the last remaining admin user. At least one admin must exist.

## Authentication Flow

### Login Process

1. Navigate to `http://localhost:8080/_/`
2. Enter your admin email and password
3. Click "Sign In"
4. Upon successful authentication:
   - A JWT token is issued (valid for 24 hours)
   - Token is stored in sessionStorage
   - You're redirected to the overview page

### JWT Token Structure

```json
{
  "iss": "supalite",
  "sub": "550e8400-e29b-41d4-a716-446655440000",
  "email": "admin@example.com",
  "role": "admin",
  "iat": 1738123456,
  "exp": 1738209856
}
```

### Protected Routes

All dashboard API endpoints (except `/api/login`) require authentication:

```bash
# Example: Get system status
curl http://localhost:8080/_/api/status \
  -H "Authorization: Bearer <your-jwt-token>"
```

## Troubleshooting

### Blank Page or 404 Errors

**Problem:** Dashboard shows a blank page or returns 404.

**Solutions:**

1. **Check if dashboard was built:**
   ```bash
   ls -la internal/dashboard/dist
   ```

   If the directory doesn't exist, build the dashboard:
   ```bash
   cd dashboard
   npm install
   npm run build
   cd ..
   make build
   ```

2. **Verify server is running:**
   ```bash
   curl http://localhost:8080/health
   # Should return: {"status":"healthy"}
   ```

3. **Check the URL:**
   - Production: `http://localhost:8080/_/` (note trailing slash)
   - Development: `http://localhost:5173`

4. **Check server logs for errors:**
   ```bash
   ./supalite serve
   # Look for: "Dashboard: http://localhost:8080/_/"
   ```

### Login Issues

**Problem:** Cannot log in with admin credentials.

**Solutions:**

1. **Verify admin user exists:**
   ```bash
   ./supalite admin list
   ```

2. **Reset admin password:**
   ```bash
   ./supalite admin change-password
   ```

3. **Create a new admin user:**
   ```bash
   ./supalite admin add
   ```

4. **Check browser console for errors:**
   - Open Developer Tools (F12)
   - Check Console tab for error messages
   - Check Network tab for failed requests

### API Connection Errors (Development Mode)

**Problem:** Dashboard dev server can't connect to backend API.

**Solutions:**

1. **Ensure backend is running:**
   ```bash
   # Terminal 1
   make serve
   ```

2. **Check Vite proxy configuration:**
   - Verify `dashboard/vite.config.ts` has correct proxy settings
   - Ensure backend is on port 8080

3. **Clear browser cache and dev tools:**
   - Hard refresh (Ctrl+Shift+R or Cmd+Shift+R)
   - Clear site data in DevTools > Application > Storage

### Build Errors

**Problem:** Dashboard build fails with errors.

**Solutions:**

1. **Clean and reinstall:**
   ```bash
   cd dashboard
   rm -rf node_modules package-lock.json
   npm install
   npm run build
   ```

2. **Check Node.js version:**
   ```bash
   node --version  # Should be 18.x or higher
   ```

3. **Check disk space:**
   ```bash
   df -h  # Ensure sufficient space
   ```

### Performance Issues

**Problem:** Dashboard loads slowly or feels sluggish.

**Solutions:**

1. **Check database size:**
   ```bash
   du -sh ./data
   ```

2. **Check for large tables:**
   ```bash
   curl http://localhost:8080/rest/v1/ \
     -H "apikey: <your-service-role-key>" \
     -H "Authorization: Bearer <your-service-role-key>"
   ```

3. **Restart the server:**
   ```bash
   # Stop with Ctrl+C, then:
   ./supalite serve
   ```

4. **Check system resources:**
   ```bash
   top  # Or Activity Monitor on macOS
   ```

## Security Best Practices

### Admin Credentials

- **Use strong passwords**: At least 12 characters with mixed case, numbers, and symbols
- **Don't share credentials**: Each admin should have their own account
- **Change passwords regularly**: Use `./supalite admin change-password`
- **Limit admin users**: Only create admin accounts for trusted personnel

### Dashboard Access

- **Use HTTPS in production**: Place Supalite behind a reverse proxy (nginx, Caddy)
- **Restrict network access**: Use firewall rules to limit dashboard access
- **Monitor access logs**: Regularly review who is accessing the dashboard
- **Logout when done**: Close the browser tab when finished

### Token Security

- **Token expiry**: Dashboard JWT tokens expire after 24 hours
- **Session storage**: Tokens are stored in browser sessionStorage (cleared on tab close)
- **No persistent sessions**: You must re-login after closing the browser tab

### Database Access

The dashboard has full access to your database via the embedded PostgreSQL connection. Ensure:
- Only trusted users have admin access
- Database credentials are not exposed in the UI
- SQL operations are logged (coming soon)

## Configuration

The dashboard uses the same configuration as the main Supalite server. No additional configuration is required.

### Environment Variables

The dashboard respects these environment variables:

- `SUPALITE_HOST`: Server bind address (default: `0.0.0.0`)
- `SUPALITE_PORT`: Server port (default: `8080`)
- `SUPALITE_DATA_DIR`: Data directory (default: `./data`)

### Custom Dashboard Secret

By default, the dashboard uses a randomly generated secret for JWT signing. To use a custom secret:

1. Add to `supalite.json`:
   ```json
   {
     "dashboard_jwt_secret": "your-32-byte-secret-key-here"
   }
   ```

2. Or set via environment variable:
   ```bash
   export SUPALITE_DASHBOARD_JWT_SECRET="your-32-byte-secret-key-here"
   ./supalite serve
   ```

**Warning:** If you change the dashboard JWT secret, all existing dashboard sessions will be invalidated and users will need to log in again.

## Architecture

### Technology Stack

- **Frontend**: React 18 + TypeScript
- **Build Tool**: Vite 5
- **UI Framework**: Tailwind CSS
- **Backend**: Go with chi router
- **Embedding**: Go's `embed` package (Go 1.16+)

### Directory Structure

```
supalite/
├── dashboard/              # Frontend source code
│   ├── src/
│   │   ├── components/    # Reusable React components
│   │   ├── pages/         # Dashboard pages
│   │   ├── lib/           # API client, utilities
│   │   └── main.tsx       # Entry point
│   ├── index.html
│   ├── vite.config.ts
│   ├── package.json
│   └── tailwind.config.js
├── internal/
│   ├── admin/             # Admin user management
│   │   ├── password.go    # Password hashing (bcrypt)
│   │   └── user.go        # User CRUD operations
│   └── dashboard/         # Dashboard serving
│       ├── server.go      # HTTP routes and middleware
│       ├── api.go         # API endpoints
│       └── jwt.go         # JWT token management
└── cmd/
    ├── admin.go           # CLI commands for admin management
    └── init.go            # Creates first admin user during init
```

### API Endpoints

#### Public Endpoints

- `POST /_/api/login` - Authenticate with email/password

#### Protected Endpoints (require JWT)

- `GET /_/api/me` - Get current user info
- `GET /_/api/status` - Get system status
- `GET /_/api/tables` - List all tables
- `GET /_/api/tables/{name}/schema` - Get table schema

#### Proxied Endpoints

- `/rest/v1/*` - Proxied to pREST (PostgREST-compatible API)
- `/auth/v1/*` - Proxied to GoTrue (Supabase Auth)

### Embedding Process

The dashboard is embedded in the binary using Go's `embed` package:

```go
//go:embed dist
var dashboardFS embed.FS
```

Build process:
1. Frontend is built with `npm run build` → `dashboard/dist/`
2. `dashboard/dist/` is copied to `internal/dashboard/dist/`
3. Go binary is built, embedding `internal/dashboard/dist/`
4. At runtime, files are served from the embedded filesystem

## Next Steps

- **Customize the dashboard**: Modify `dashboard/src/` to add features
- **Add monitoring**: Set up logging and alerts
- **Integrate with tools**: Connect to external monitoring services
- **Contribute**: See [Contributing Guidelines](../CONTRIBUTING.md) for details

## Support

For issues, questions, or contributions:
- **GitHub Issues**: [github.com/markb/supalite/issues](https://github.com/markb/supalite/issues)
- **Documentation**: [docs/](../docs/)
- **Project Structure**: See [README.md](../README.md)
