package server

import (
	"context"
	cryptoRand "crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/markb/supalite/internal/auth"
	"github.com/markb/supalite/internal/keys"
	"github.com/markb/supalite/internal/log"
	"github.com/markb/supalite/internal/mailcapture"
	"github.com/markb/supalite/internal/pg"
	"github.com/markb/supalite/internal/prest"
	"github.com/rs/cors"
)

type Server struct {
	config     Config
	router     *chi.Mux
	httpServer *http.Server

	pgDatabase    *pg.EmbeddedDatabase
	prestServer   *prest.Server
	authServer    *auth.Server
	keyManager    *keys.Manager
	captureServer *mailcapture.Server
}

type Config struct {
	Host         string
	Port         int
	PGPort       uint16
	DataDir      string
	JWTSecret    string
	SiteURL      string
	PGUsername   string
	PGPassword   string
	PGDatabase   string
	RuntimePath  string // Optional: unique runtime path for test isolation
	AnonKey      string // Optional: pre-generated anon key
	ServiceRoleKey string // Optional: pre-generated service_role key
	Email        *auth.EmailConfig // Optional: email configuration for GoTrue
}

func New(cfg Config) *Server {
	return &Server{
		config: cfg,
		router: chi.NewRouter(),
	}
}

// quoteIdentifier quotes a SQL identifier for PostgreSQL.
// Identifiers with spaces or special characters need to be double-quoted.
// Double quotes within the identifier are escaped by doubling them.
func quoteIdentifier(ident string) string {
	// Escape existing double quotes by doubling them
	escaped := strings.ReplaceAll(ident, "\"", "\"\"")
	// Wrap in double quotes
	return fmt.Sprintf("\"%s\"", escaped)
}

func (s *Server) Start(ctx context.Context) error {
	log.Info("starting Supalite server...")

	// 1. Start embedded PostgreSQL
	log.Info("starting embedded PostgreSQL...")

	// Set default credentials if not provided
	pgUsername := s.config.PGUsername
	if pgUsername == "" {
		pgUsername = "postgres"
	}
	pgPassword := s.config.PGPassword
	if pgPassword == "" {
		pgPassword = "postgres"
	}
	pgDatabase := s.config.PGDatabase
	if pgDatabase == "" {
		pgDatabase = "postgres"
	}

	pgCfg := pg.Config{
		Port:        s.config.PGPort,
		Username:    pgUsername,
		Password:    pgPassword,
		Database:    pgDatabase,
		DataDir:     s.config.DataDir,
		Version:     "16.9.0",
		RuntimePath: s.config.RuntimePath,
	}
	s.pgDatabase = pg.NewEmbeddedDatabase(pgCfg)

	if err := s.pgDatabase.Start(ctx); err != nil {
		return fmt.Errorf("failed to start PostgreSQL: %w", err)
	}
	log.Info("PostgreSQL started", "port", s.config.PGPort)

	// 2. Initialize database schema
	if err := s.initSchema(ctx); err != nil {
		return fmt.Errorf("failed to initialize schema: %w", err)
	}

	// 2.5. Initialize key manager (anon/service_role keys)
	log.Info("initializing key manager...")

	var keyManager *keys.Manager
	var err error

	if s.config.JWTSecret == "" {
		// ES256 mode (default): use empty string to trigger ES256 mode
		log.Info("using ES256 mode with auto-generated keys")
		keyManager, err = keys.NewManager(s.config.DataDir, "")
	} else {
		// Legacy mode: user explicitly provided JWT_SECRET
		log.Info("using legacy mode (JWT_SECRET)")
		keyManager, err = keys.NewManager(s.config.DataDir, s.config.JWTSecret)
	}

	if err != nil {
		return fmt.Errorf("failed to initialize key manager: %w", err)
	}
	s.keyManager = keyManager

	// Set JWT secret for GoTrue (needs it regardless of mode)
	jwtSecret := s.config.JWTSecret
	if jwtSecret == "" {
		jwtSecret = generateRandomSecret(32)
	}

	if keyManager.IsLegacyMode() {
		log.Info("keys initialized", "mode", "legacy (JWT_SECRET)")
	} else {
		log.Info("keys initialized", "mode", "ES256")
	}

	// Display the keys
	log.Info("==========================================")
	log.Info("Project API Keys")
	log.Info("==========================================")
	log.Info("Project URL:", s.config.SiteURL)
	log.Info("")
	log.Info("anon key (public):")
	log.Info("  " + s.keyManager.GetAnonKey())
	log.Info("")
	log.Warn("service_role key (secret - keep hidden!):")
	log.Warn("  " + s.keyManager.GetServiceKey())
	log.Info("")
	log.Info("Use these keys in your Supabase client libraries:")
	log.Info(fmt.Sprintf("  const supabase = createClient('%s',", s.config.SiteURL))
	log.Info(fmt.Sprintf("    '%s',  // anon key", s.keyManager.GetAnonKey()))
	log.Info(fmt.Sprintf("    '%s'  // service_role key (use with caution!)", s.keyManager.GetServiceKey()))
	log.Info("  )")
	log.Info("==========================================")

	connString := s.pgDatabase.ConnectionString()

	// 3. Start pREST server
	log.Info("starting pREST server...")
	prestCfg := prest.DefaultConfig(connString)
	s.prestServer = prest.NewServer(prestCfg)
	if err := s.prestServer.Start(ctx); err != nil {
		return fmt.Errorf("failed to start pREST: %w", err)
	}
	log.Info("pREST started", "port", prestCfg.Port)

	// 3.5. Start mail capture server if configured
	if s.config.Email != nil && s.config.Email.CaptureMode {
		capturePort := s.config.Email.CapturePort
		if capturePort == 0 {
			capturePort = 1025
		}

		log.Info("starting mail capture server...")
		s.captureServer = mailcapture.NewServer(mailcapture.Config{
			Port:     capturePort,
			Host:     "localhost",
			Database: s.pgDatabase,
		})

		if err := s.captureServer.Start(ctx); err != nil {
			log.Warn("failed to start mail capture server", "error", err)
		} else {
			log.Info("mail capture server started", "port", capturePort)
		}
	}

	// 4. Start GoTrue auth server
	log.Info("starting GoTrue auth server...")
	authCfg := auth.DefaultConfig()
	// Add search_path for GoTrue to find its tables in the auth schema
	authCfg.ConnString = connString + "?search_path=auth"
	authCfg.JWTSecret = jwtSecret // Use the JWT secret we set up for the key manager
	authCfg.SiteURL = s.config.SiteURL

	// Handle email configuration
	if s.config.Email != nil {
		if s.config.Email.CaptureMode && s.captureServer != nil && s.captureServer.IsRunning() {
			// Override SMTP settings to point to local capture server
			log.Info("configuring GoTrue to use mail capture server")
			authCfg.Email = &auth.EmailConfig{
				SMTPHost:   "localhost",
				SMTPPort:   s.captureServer.Port(),
				SMTPUser:   "capture",
				SMTPPass:   "capture",
				AdminEmail: s.config.Email.AdminEmail,
				URLPathsInvite:       s.config.Email.URLPathsInvite,
				URLPathsConfirmation: s.config.Email.URLPathsConfirmation,
				URLPathsRecovery:     s.config.Email.URLPathsRecovery,
				URLPathsEmailChange:  s.config.Email.URLPathsEmailChange,
				Autoconfirm:          s.config.Email.Autoconfirm,
			}
		} else {
			authCfg.Email = s.config.Email
		}
	}

	s.authServer = auth.NewServer(authCfg)
	if err := s.authServer.Start(ctx); err != nil {
		log.Warn("failed to start GoTrue", "error", err)
		log.Warn("auth API will not be available")
	} else {
		log.Info("GoTrue started", "port", authCfg.Port)
	}

	// 5. Setup orchestration routes
	s.setupRoutes()

	// 6. Start main HTTP server
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      s.corsHandler(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("Supalite listening", "addr", addr)
		log.Info("APIs available:")
		log.Info("  Auth:    http://localhost:8080/auth/v1/*")
		log.Info("  REST:    http://localhost:8080/rest/v1/*")
		log.Info("  Health:  http://localhost:8080/health")
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// 7. Wait for shutdown signal (use background context to avoid timeout)
	return s.waitForShutdown(context.Background())
}

func (s *Server) setupRoutes() {
	s.router.Get("/health", s.handleHealth)

	// JWKS endpoint for public key discovery (ES256 mode)
	s.router.HandleFunc("/.well-known/jwks.json", s.handleJWKS)

	// Create Supabase-compatible REST API handler
	// Translates /rest/v1/{table} to /{database}/{schema}/{table} for pREST
	s.router.HandleFunc("/rest/v1", s.handleSupabaseREST)
	s.router.HandleFunc("/rest/v1/*", s.handleSupabaseREST)

	// Proxy requests to GoTrue auth server
	s.router.HandleFunc("/auth/v1/*", s.handleAuthRequest)
}

// corsHandler returns a CORS-wrapped handler for the router
func (s *Server) corsHandler() http.Handler {
	// Configure CORS to allow requests from browser-based Supabase clients
	// Uses permissive settings for development (can be made configurable for production)
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"}, // Allow all origins for development
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"}, // Allow all headers (including Authorization, apikey, Content-Type, etc.)
		AllowCredentials: false,         // Must be false when AllowedOrigins is "*"
		MaxAge:           86400,         // Cache preflight response for 24 hours
	})
	return c.Handler(s.router)
}

// handleJWKS serves the JWKS (JSON Web Key Set) for public key discovery
func (s *Server) handleJWKS(w http.ResponseWriter, r *http.Request) {
	if s.keyManager == nil {
		http.Error(w, "Key manager not initialized", http.StatusInternalServerError)
		return
	}

	jwks, err := s.keyManager.GetJWKS()
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(jwks)
}


// handleAuthRequest proxies requests to the GoTrue auth server
func (s *Server) handleAuthRequest(w http.ResponseWriter, r *http.Request) {
	// Strip /auth/v1 prefix from the path
	prefix := "/auth/v1"
	originalPath := r.URL.Path

	// Check if the path starts with the prefix
	if len(originalPath) > len(prefix) && originalPath[:len(prefix)+1] == prefix+"/" {
		// Create a modified request with the prefix stripped
		requestPath := originalPath[len(prefix):]
		if requestPath == "" {
			requestPath = "/"
		}

		// Clone the request and update the path
		r = r.Clone(r.Context())
		r.URL.Path = requestPath
		r.URL.RawPath = requestPath
	}

	s.authServer.Handler().ServeHTTP(w, r)
}

// handleSupabaseREST implements Supabase/PostgREST-compatible REST API
// URL format: /rest/v1/{table}?select=*&order=name&limit=10
func (s *Server) handleSupabaseREST(w http.ResponseWriter, r *http.Request) {
	// Remove /rest/v1 prefix
	remainingPath := r.URL.Path[len("/rest/v1"):]
	if remainingPath == "" || remainingPath == "/" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	// Remove leading slash
	if remainingPath[0] == '/' {
		remainingPath = remainingPath[1:]
	}

	// Parse the table name
	parts := strings.Split(remainingPath, "/")
	if len(parts) == 0 {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	tableName := parts[0]

	// Get the HTTP method
	method := r.Method

	// Build and execute query based on method
	ctx := r.Context()
	conn, err := s.pgDatabase.Connect(ctx)
	if err != nil {
		http.Error(w, fmt.Sprintf("database connection error: %v", err), http.StatusInternalServerError)
		return
	}
	defer conn.Close(ctx)

	switch method {
	case "GET":
		s.handleGET(ctx, conn, w, r, tableName)
	case "HEAD":
		s.handleHEAD(ctx, conn, w, r, tableName)
	case "POST":
		s.handlePOST(ctx, conn, w, r, tableName)
	case "PATCH", "PUT":
		s.handlePATCH(ctx, conn, w, r, tableName)
	case "DELETE":
		s.handleDELETE(ctx, conn, w, r, tableName)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// embeddedResource represents a foreign key relationship to fetch
type embeddedResource struct {
	alias       string // e.g., "sender" in sender:users!sender_id(id,name)
	table       string // e.g., "users"
	fkColumn    string // e.g., "sender_id" (if specified with !)
	columns     string // e.g., "id,name"
	isInner     bool   // true for !inner modifier
}

// parseSelectClause parses the PostgREST-style select string and returns:
// - mainColumns: columns to select from the main table
// - embedded: list of embedded resources to fetch
func parseSelectClause(selectStr string) (mainColumns []string, embedded []embeddedResource) {
	if selectStr == "" || selectStr == "*" {
		return []string{"*"}, nil
	}

	// Track parenthesis depth to properly split on commas
	depth := 0
	current := ""
	parts := []string{}

	for _, ch := range selectStr {
		switch ch {
		case '(':
			depth++
			current += string(ch)
		case ')':
			depth--
			current += string(ch)
		case ',':
			if depth == 0 {
				if trimmed := strings.TrimSpace(current); trimmed != "" {
					parts = append(parts, trimmed)
				}
				current = ""
			} else {
				current += string(ch)
			}
		default:
			current += string(ch)
		}
	}
	if trimmed := strings.TrimSpace(current); trimmed != "" {
		parts = append(parts, trimmed)
	}

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Check if this is an embedded resource: table(columns) or alias:table!fk(columns)
		if parenIdx := strings.Index(part, "("); parenIdx > 0 && strings.HasSuffix(part, ")") {
			embRes := embeddedResource{}
			prefix := part[:parenIdx]
			embRes.columns = part[parenIdx+1 : len(part)-1]

			// Check for alias: alias:table or alias:table!fk
			if colonIdx := strings.Index(prefix, ":"); colonIdx > 0 {
				embRes.alias = prefix[:colonIdx]
				prefix = prefix[colonIdx+1:]
			}

			// Check for foreign key specifier and inner join: table!inner or table!fk or table!fk!inner
			if bangIdx := strings.Index(prefix, "!"); bangIdx > 0 {
				modifierParts := strings.Split(prefix, "!")
				embRes.table = modifierParts[0]
				for _, modifier := range modifierParts[1:] {
					if modifier == "inner" {
						embRes.isInner = true
					} else {
						embRes.fkColumn = modifier
					}
				}
			} else {
				embRes.table = prefix
			}

			if embRes.alias == "" {
				embRes.alias = embRes.table
			}
			embedded = append(embedded, embRes)
		} else {
			// Regular column - might have JSON arrow notation
			mainColumns = append(mainColumns, part)
		}
	}

	if len(mainColumns) == 0 {
		mainColumns = []string{"*"}
	}

	return mainColumns, embedded
}

// containsColumn checks if a column list contains a specific column
func containsColumn(columns []string, col string) bool {
	for _, c := range columns {
		if strings.TrimSpace(c) == col {
			return true
		}
	}
	return false
}

// buildSelectColumn builds a SQL column expression from a PostgREST column spec
func buildSelectColumn(col string) string {
	col = strings.TrimSpace(col)
	if col == "*" {
		return "*"
	}

	// Handle JSON arrow notation: address->city or address->>city
	if strings.Contains(col, "->>") {
		parts := strings.SplitN(col, "->>", 2)
		return fmt.Sprintf("%s->>'%s' AS %s", quoteIdentifier(parts[0]), parts[1], quoteIdentifier(parts[1]))
	}
	if strings.Contains(col, "->") {
		parts := strings.SplitN(col, "->", 2)
		return fmt.Sprintf("%s->'%s' AS %s", quoteIdentifier(parts[0]), parts[1], quoteIdentifier(parts[1]))
	}

	return quoteIdentifier(col)
}

// handleGET processes SELECT requests
func (s *Server) handleGET(ctx context.Context, conn *pgx.Conn, w http.ResponseWriter, r *http.Request, table string) {
	query := r.URL.Query()

	// Quote table name for SQL
	quotedTable := quoteIdentifier(table)

	// Parse select clause
	var selectStr string
	if selectVals := query["select"]; len(selectVals) > 0 {
		selectStr = selectVals[0]
	} else {
		selectStr = "*"
	}

	mainColumns, embedded := parseSelectClause(selectStr)

	// Pre-analyze embedded resources to find required join columns
	extraCols := make(map[string]bool) // columns we need but weren't requested
	var fkInfoMap = make(map[string]*foreignKeyInfo)

	if len(embedded) > 0 && !containsColumn(mainColumns, "*") {
		ctx := r.Context()
		for _, emb := range embedded {
			fkInfo, err := s.findForeignKey(ctx, conn, table, emb.table, emb.fkColumn)
			if err == nil {
				fkInfoMap[emb.alias] = fkInfo
				// Determine which column we need from main table
				if fkInfo.isReverse {
					// Need the referenced column (usually 'id')
					if !containsColumn(mainColumns, fkInfo.referencedColumn) {
						extraCols[fkInfo.referencedColumn] = true
					}
				} else {
					// Need the FK column (e.g., 'country_id')
					if !containsColumn(mainColumns, fkInfo.column) {
						extraCols[fkInfo.column] = true
					}
				}
			}
		}
	}

	// Build SELECT clause with proper quoting
	quotedCols := make([]string, 0, len(mainColumns)+len(extraCols))
	for _, col := range mainColumns {
		quotedCols = append(quotedCols, buildSelectColumn(col))
	}

	// Add extra columns needed for joins
	for col := range extraCols {
		quotedCols = append(quotedCols, quoteIdentifier(col))
	}

	selectClause := strings.Join(quotedCols, ", ")

	sqlQuery := fmt.Sprintf("SELECT %s FROM public.%s", selectClause, quotedTable)

	// Add WHERE clause (but filter out embedded table filters for now)
	whereClause, whereArgs := s.buildWhereClause(query, 0)
	if whereClause != "" {
		sqlQuery += " WHERE " + whereClause
	}

	// Add ORDER BY with proper quoting
	if orderVals := query["order"]; len(orderVals) > 0 {
		orderClause := orderVals[0]
		// Handle order with direction (e.g., "name.desc" or "name ASC")
		if strings.Contains(orderClause, ".") {
			parts := strings.SplitN(orderClause, ".", 2)
			if len(parts) == 2 {
				// Use space before direction to avoid ambiguity with quoted identifiers
				// e.g., "another column".desc becomes "another column" DESC
				direction := strings.ToUpper(parts[1])
				if direction == "ASC" || direction == "DESC" {
					orderClause = fmt.Sprintf("%s %s", quoteIdentifier(parts[0]), direction)
				} else {
					// Unknown direction, treat as part of column name
					orderClause = quoteIdentifier(orderClause)
				}
			} else {
				orderClause = quoteIdentifier(orderClause)
			}
		} else if strings.Contains(strings.ToUpper(orderClause), " ASC") || strings.Contains(strings.ToUpper(orderClause), " DESC") {
			// Split by the last space to separate column from direction
			lastSpace := strings.LastIndex(orderClause, " ")
			if lastSpace > 0 {
				col := orderClause[:lastSpace]
				dir := orderClause[lastSpace+1:]
				orderClause = fmt.Sprintf("%s %s", quoteIdentifier(col), dir)
			} else {
				orderClause = quoteIdentifier(orderClause)
			}
		} else {
			orderClause = quoteIdentifier(orderClause)
		}
		sqlQuery += fmt.Sprintf(" ORDER BY %s", orderClause)
	}

	// Add LIMIT
	if limitVals := query["limit"]; len(limitVals) > 0 {
		sqlQuery += fmt.Sprintf(" LIMIT %s", limitVals[0])
	}

	// Add OFFSET
	if offsetVals := query["offset"]; len(offsetVals) > 0 {
		sqlQuery += fmt.Sprintf(" OFFSET %s", offsetVals[0])
	}

	// Execute main query
	rows, err := conn.Query(ctx, sqlQuery, whereArgs...)
	if err != nil {
		http.Error(w, fmt.Sprintf("query error: %v", err), http.StatusBadRequest)
		return
	}
	defer rows.Close()

	// Fetch all results
	results := make([]map[string]interface{}, 0)
	for rows.Next() {
		row, err := rows.Values()
		if err != nil {
			http.Error(w, fmt.Sprintf("row scan error: %v", err), http.StatusInternalServerError)
			return
		}

		// Get column descriptions
		desc := rows.FieldDescriptions()
		result := make(map[string]interface{})
		for i, col := range desc {
			result[col.Name] = row[i]
		}
		results = append(results, result)
	}

	// Fetch embedded resources if any
	if len(embedded) > 0 && len(results) > 0 {
		var err error
		results, err = s.fetchEmbeddedResourcesWithFKInfo(ctx, conn, table, results, embedded, query, fkInfoMap)
		if err != nil {
			http.Error(w, fmt.Sprintf("embedded resource error: %v", err), http.StatusBadRequest)
			return
		}
	}

	// Remove extra columns that were added just for joining
	if len(extraCols) > 0 {
		for _, result := range results {
			for col := range extraCols {
				delete(result, col)
			}
		}
	}

	// Check for count header
	prefer := r.Header.Get("Prefer")
	if strings.Contains(prefer, "count=exact") {
		// Execute count query
		countQuery := fmt.Sprintf("SELECT COUNT(*) FROM public.%s", quotedTable)
		if whereClause != "" {
			countQuery += " WHERE " + whereClause
		}
		var count int64
		err := conn.QueryRow(ctx, countQuery, whereArgs...).Scan(&count)
		if err == nil {
			// Set Content-Range header: items 0-N/total
			rangeEnd := int64(len(results)) - 1
			if rangeEnd < 0 {
				rangeEnd = 0
			}
			w.Header().Set("Content-Range", fmt.Sprintf("0-%d/%d", rangeEnd, count))
		}
	}

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(results)
}

// handleHEAD processes HEAD requests (count-only)
func (s *Server) handleHEAD(ctx context.Context, conn *pgx.Conn, w http.ResponseWriter, r *http.Request, table string) {
	query := r.URL.Query()
	quotedTable := quoteIdentifier(table)

	// Build WHERE clause
	whereClause, whereArgs := s.buildWhereClause(query, 0)

	// Execute count query
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM public.%s", quotedTable)
	if whereClause != "" {
		countQuery += " WHERE " + whereClause
	}

	var count int64
	err := conn.QueryRow(ctx, countQuery, whereArgs...).Scan(&count)
	if err != nil {
		http.Error(w, fmt.Sprintf("count error: %v", err), http.StatusBadRequest)
		return
	}

	// Set Content-Range header
	w.Header().Set("Content-Range", fmt.Sprintf("0-0/%d", count))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

// fetchEmbeddedResourcesWithFKInfo fetches related data using pre-computed FK info
// Returns a possibly filtered results slice (for inner joins that filter out non-matching rows)
func (s *Server) fetchEmbeddedResourcesWithFKInfo(ctx context.Context, conn *pgx.Conn, mainTable string, results []map[string]interface{}, embedded []embeddedResource, query url.Values, fkInfoMap map[string]*foreignKeyInfo) ([]map[string]interface{}, error) {
	for _, emb := range embedded {
		// Get pre-computed FK info
		fkInfo, ok := fkInfoMap[emb.alias]
		if !ok {
			// Try to find it now (shouldn't happen, but fallback)
			var err error
			fkInfo, err = s.findForeignKey(ctx, conn, mainTable, emb.table, emb.fkColumn)
			if err != nil {
				return nil, fmt.Errorf("cannot find relationship for %s: %w", emb.table, err)
			}
		}

		// Check if there's a filter on this embedded table
		embeddedFilter := ""
		for key, vals := range query {
			if strings.HasPrefix(key, emb.alias+".") || strings.HasPrefix(key, emb.table+".") {
				filterCol := strings.TrimPrefix(key, emb.alias+".")
				filterCol = strings.TrimPrefix(filterCol, emb.table+".")
				if len(vals) > 0 {
					// Parse the filter value
					filterVal := vals[0]
					if strings.HasPrefix(filterVal, "eq.") {
						embeddedFilter = fmt.Sprintf("%s = '%s'", quoteIdentifier(filterCol), filterVal[3:])
					}
				}
				break
			}
		}

		// Fetch related data based on relationship direction
		if fkInfo.isManyToMany {
			// Many-to-many through junction table
			// e.g., users -> user_teams -> teams
			for _, result := range results {
				mainID := result["id"]
				if mainID == nil {
					result[emb.alias] = []interface{}{}
					continue
				}

				// Build column list for embedded query
				var embCols string
				if emb.columns == "" || emb.columns == "*" {
					embCols = fmt.Sprintf("t.*")
				} else {
					cols := strings.Split(emb.columns, ",")
					quotedCols := make([]string, len(cols))
					for i, c := range cols {
						quotedCols[i] = fmt.Sprintf("t.%s", quoteIdentifier(strings.TrimSpace(c)))
					}
					embCols = strings.Join(quotedCols, ", ")
				}

				// Query through junction table
				embQuery := fmt.Sprintf(`
					SELECT %s FROM public.%s t
					INNER JOIN public.%s j ON j.%s = t.id
					WHERE j.%s = $1`,
					embCols,
					quoteIdentifier(emb.table),
					quoteIdentifier(fkInfo.junctionTable),
					quoteIdentifier(fkInfo.junctionForeignFK),
					quoteIdentifier(fkInfo.junctionMainFK))
				if embeddedFilter != "" {
					embQuery += " AND " + embeddedFilter
				}

				embRows, err := conn.Query(ctx, embQuery, mainID)
				if err != nil {
					return nil, fmt.Errorf("embedded query error: %w", err)
				}

				embResults := make([]map[string]interface{}, 0)
				for embRows.Next() {
					embRow, err := embRows.Values()
					if err != nil {
						embRows.Close()
						return nil, fmt.Errorf("embedded row error: %w", err)
					}
					embDesc := embRows.FieldDescriptions()
					embResult := make(map[string]interface{})
					for i, col := range embDesc {
						embResult[col.Name] = embRow[i]
					}
					embResults = append(embResults, embResult)
				}
				embRows.Close()

				// Remove if inner join with no match, or if there's a filter but no matching result
				if len(embResults) == 0 && (emb.isInner || embeddedFilter != "") {
					result["__remove__"] = true
				} else {
					result[emb.alias] = embResults
				}
			}
		} else if fkInfo.isReverse {
			// The foreign table has FK pointing to main table
			// e.g., instruments.section_id -> orchestral_sections.id
			// When querying orchestral_sections, fetch instruments where section_id = orchestral_sections.id
			for _, result := range results {
				mainID := result[fkInfo.referencedColumn]
				if mainID == nil {
					result[emb.alias] = []interface{}{}
					continue
				}

				// Build column list for embedded query
				var embCols string
				if emb.columns == "" || emb.columns == "*" {
					embCols = "*"
				} else {
					cols := strings.Split(emb.columns, ",")
					quotedCols := make([]string, len(cols))
					for i, c := range cols {
						quotedCols[i] = quoteIdentifier(strings.TrimSpace(c))
					}
					embCols = strings.Join(quotedCols, ", ")
				}

				embQuery := fmt.Sprintf("SELECT %s FROM public.%s WHERE %s = $1",
					embCols, quoteIdentifier(emb.table), quoteIdentifier(fkInfo.column))
				if embeddedFilter != "" {
					embQuery += " AND " + embeddedFilter
				}

				embRows, err := conn.Query(ctx, embQuery, mainID)
				if err != nil {
					return nil, fmt.Errorf("embedded query error: %w", err)
				}

				embResults := make([]map[string]interface{}, 0)
				for embRows.Next() {
					embRow, err := embRows.Values()
					if err != nil {
						embRows.Close()
						return nil, fmt.Errorf("embedded row error: %w", err)
					}
					embDesc := embRows.FieldDescriptions()
					embResult := make(map[string]interface{})
					for i, col := range embDesc {
						embResult[col.Name] = embRow[i]
					}
					embResults = append(embResults, embResult)
				}
				embRows.Close()

				// Remove if inner join with no match, or if there's a filter but no matching result
				if len(embResults) == 0 && (emb.isInner || embeddedFilter != "") {
					result["__remove__"] = true
				} else {
					result[emb.alias] = embResults
				}
			}
		} else {
			// Main table has FK pointing to foreign table
			// e.g., cities.country_id -> countries.id
			// When querying cities, fetch the country where id = cities.country_id
			for _, result := range results {
				fkValue := result[fkInfo.column]
				if fkValue == nil {
					result[emb.alias] = nil
					continue
				}

				// Build column list for embedded query
				var embCols string
				if emb.columns == "" || emb.columns == "*" {
					embCols = "*"
				} else {
					cols := strings.Split(emb.columns, ",")
					quotedCols := make([]string, len(cols))
					for i, c := range cols {
						quotedCols[i] = quoteIdentifier(strings.TrimSpace(c))
					}
					embCols = strings.Join(quotedCols, ", ")
				}

				embQuery := fmt.Sprintf("SELECT %s FROM public.%s WHERE %s = $1",
					embCols, quoteIdentifier(emb.table), quoteIdentifier(fkInfo.referencedColumn))
				if embeddedFilter != "" {
					embQuery += " AND " + embeddedFilter
				}

				var embResult map[string]interface{}
				embRow, err := conn.Query(ctx, embQuery, fkValue)
				if err != nil {
					return nil, fmt.Errorf("embedded query error: %w", err)
				}
				if embRow.Next() {
					vals, err := embRow.Values()
					if err != nil {
						embRow.Close()
						return nil, fmt.Errorf("embedded row error: %w", err)
					}
					embDesc := embRow.FieldDescriptions()
					embResult = make(map[string]interface{})
					for i, col := range embDesc {
						embResult[col.Name] = vals[i]
					}
				}
				embRow.Close()

				// Remove if inner join with no match, or if there's a filter but no matching result
				if embResult == nil && (emb.isInner || embeddedFilter != "") {
					result["__remove__"] = true
				} else {
					result[emb.alias] = embResult
				}
			}
		}
	}

	// Remove rows marked for removal (inner join with no match)
	finalResults := make([]map[string]interface{}, 0, len(results))
	for _, r := range results {
		if _, remove := r["__remove__"]; !remove {
			delete(r, "__remove__")
			finalResults = append(finalResults, r)
		}
	}

	return finalResults, nil
}

// foreignKeyInfo holds information about a foreign key relationship
type foreignKeyInfo struct {
	column           string // The column in the "from" table
	referencedTable  string // The referenced table
	referencedColumn string // The column in the referenced table
	isReverse        bool   // true if the FK points from the foreign table to main table
	isManyToMany     bool   // true if this is a many-to-many through junction table
	junctionTable    string // The junction table name (for many-to-many)
	junctionMainFK   string // FK column in junction pointing to main table
	junctionForeignFK string // FK column in junction pointing to foreign table
}

// findForeignKey finds the foreign key relationship between two tables
func (s *Server) findForeignKey(ctx context.Context, conn *pgx.Conn, mainTable, foreignTable, specifiedFK string) (*foreignKeyInfo, error) {
	// First, check if there's a direct FK from main table to foreign table
	query := `
		SELECT
			kcu.column_name,
			ccu.table_name AS foreign_table_name,
			ccu.column_name AS foreign_column_name
		FROM information_schema.table_constraints AS tc
		JOIN information_schema.key_column_usage AS kcu
			ON tc.constraint_name = kcu.constraint_name
			AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage AS ccu
			ON ccu.constraint_name = tc.constraint_name
			AND ccu.table_schema = tc.table_schema
		WHERE tc.constraint_type = 'FOREIGN KEY'
			AND tc.table_name = $1
			AND ccu.table_name = $2
	`
	if specifiedFK != "" {
		query += fmt.Sprintf(" AND kcu.column_name = '%s'", specifiedFK)
	}

	rows, err := conn.Query(ctx, query, mainTable, foreignTable)
	if err != nil {
		return nil, err
	}

	var foundDirect bool
	var directCol, directRefTable, directRefCol string
	if rows.Next() {
		if err := rows.Scan(&directCol, &directRefTable, &directRefCol); err != nil {
			rows.Close()
			return nil, err
		}
		foundDirect = true
	}
	rows.Close() // Close before next query

	if foundDirect {
		return &foreignKeyInfo{
			column:           directCol,
			referencedTable:  directRefTable,
			referencedColumn: directRefCol,
			isReverse:        false,
		}, nil
	}

	// Check reverse: FK from foreign table to main table
	query2 := `
		SELECT
			kcu.column_name,
			ccu.table_name AS foreign_table_name,
			ccu.column_name AS foreign_column_name
		FROM information_schema.table_constraints AS tc
		JOIN information_schema.key_column_usage AS kcu
			ON tc.constraint_name = kcu.constraint_name
			AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage AS ccu
			ON ccu.constraint_name = tc.constraint_name
			AND ccu.table_schema = tc.table_schema
		WHERE tc.constraint_type = 'FOREIGN KEY'
			AND tc.table_name = $1
			AND ccu.table_name = $2
	`
	if specifiedFK != "" {
		query2 += fmt.Sprintf(" AND kcu.column_name = '%s'", specifiedFK)
	}

	rows2, err := conn.Query(ctx, query2, foreignTable, mainTable)
	if err != nil {
		return nil, err
	}

	var foundReverse bool
	var reverseCol, reverseRefTable, reverseRefCol string
	if rows2.Next() {
		if err := rows2.Scan(&reverseCol, &reverseRefTable, &reverseRefCol); err != nil {
			rows2.Close()
			return nil, err
		}
		foundReverse = true
	}
	rows2.Close() // Close before next query

	if foundReverse {
		return &foreignKeyInfo{
			column:           reverseCol,
			referencedTable:  reverseRefTable,
			referencedColumn: reverseRefCol,
			isReverse:        true,
		}, nil
	}

	// Check for many-to-many through a junction table
	// Look for a junction table that has FKs to both tables
	junctionQuery := `
		SELECT DISTINCT tc.table_name as junction_table
		FROM information_schema.table_constraints AS tc
		JOIN information_schema.key_column_usage AS kcu
			ON tc.constraint_name = kcu.constraint_name
		JOIN information_schema.constraint_column_usage AS ccu
			ON ccu.constraint_name = tc.constraint_name
		WHERE tc.constraint_type = 'FOREIGN KEY'
			AND ccu.table_name = $1
		INTERSECT
		SELECT DISTINCT tc.table_name as junction_table
		FROM information_schema.table_constraints AS tc
		JOIN information_schema.key_column_usage AS kcu
			ON tc.constraint_name = kcu.constraint_name
		JOIN information_schema.constraint_column_usage AS ccu
			ON ccu.constraint_name = tc.constraint_name
		WHERE tc.constraint_type = 'FOREIGN KEY'
			AND ccu.table_name = $2
	`
	jRows, err := conn.Query(ctx, junctionQuery, mainTable, foreignTable)
	if err != nil {
		return nil, err
	}
	defer jRows.Close()

	if jRows.Next() {
		var junctionTable string
		if err := jRows.Scan(&junctionTable); err != nil {
			return nil, err
		}
		jRows.Close()

		// Get the FK column names from the junction table
		fkQuery := `
			SELECT kcu.column_name, ccu.table_name
			FROM information_schema.table_constraints AS tc
			JOIN information_schema.key_column_usage AS kcu
				ON tc.constraint_name = kcu.constraint_name
			JOIN information_schema.constraint_column_usage AS ccu
				ON ccu.constraint_name = tc.constraint_name
			WHERE tc.constraint_type = 'FOREIGN KEY'
				AND tc.table_name = $1
				AND (ccu.table_name = $2 OR ccu.table_name = $3)
		`
		fkRows, err := conn.Query(ctx, fkQuery, junctionTable, mainTable, foreignTable)
		if err != nil {
			return nil, err
		}

		var mainFK, foreignFK string
		for fkRows.Next() {
			var col, refTable string
			if err := fkRows.Scan(&col, &refTable); err != nil {
				fkRows.Close()
				return nil, err
			}
			if refTable == mainTable {
				mainFK = col
			} else if refTable == foreignTable {
				foreignFK = col
			}
		}
		fkRows.Close()

		return &foreignKeyInfo{
			column:            "id",
			referencedTable:   foreignTable,
			referencedColumn:  "id",
			isReverse:         true,
			isManyToMany:      true,
			junctionTable:     junctionTable,
			junctionMainFK:    mainFK,
			junctionForeignFK: foreignFK,
		}, nil
	}

	return nil, fmt.Errorf("no foreign key relationship found between %s and %s", mainTable, foreignTable)
}

// buildWhereClause constructs WHERE clause from query parameters
// Supabase format: ?column=eq.value ?column=gt.value ?column=lt.value
// offset is the starting parameter number (for use in UPDATE queries with SET clause)
func (s *Server) buildWhereClause(query url.Values, offset int) (string, []interface{}) {
	var clauses []string
	var args []interface{}

	// Skip non-filter parameters (like select, order, limit, offset)
	// Also skip embedded table filters (e.g., countries.name=eq.Canada) - they're handled separately
	skipParams := map[string]bool{
		"select": true,
		"order":  true,
		"limit":  true,
		"offset": true,
	}

	for key, values := range query {
		if skipParams[key] || len(values) == 0 {
			continue
		}

		// Skip embedded table filters (e.g., countries.name=eq.Canada)
		// These have a dot in the key that's not a JSON arrow operator
		if strings.Contains(key, ".") && !strings.Contains(key, "->") {
			continue
		}

		value := values[0]

		// Parse operator from value (e.g., "eq.1", "gt.5", "lt.10")
		if strings.Contains(value, ".") {
			parts := strings.SplitN(value, ".", 2)
			if len(parts) == 2 {
				operator := parts[0]
				argValue := parts[1]

				// Build the column reference - handle JSON arrow operators
				var colRef string
				if strings.Contains(key, "->>") {
					// JSON arrow operator: address->>postcode becomes "address"->>'postcode'
					jsonParts := strings.SplitN(key, "->>", 2)
					colRef = fmt.Sprintf("%s->>'%s'", quoteIdentifier(jsonParts[0]), jsonParts[1])
				} else if strings.Contains(key, "->") {
					// JSON arrow operator: address->city becomes "address"->'city'
					jsonParts := strings.SplitN(key, "->", 2)
					colRef = fmt.Sprintf("%s->'%s'", quoteIdentifier(jsonParts[0]), jsonParts[1])
				} else {
					// Regular column reference
					colRef = quoteIdentifier(key)
				}

				switch operator {
				case "eq":
					clauses = append(clauses, fmt.Sprintf("%s = $%d", colRef, offset+len(args)+1))
					args = append(args, argValue)
				case "neq":
					clauses = append(clauses, fmt.Sprintf("%s != $%d", colRef, offset+len(args)+1))
					args = append(args, argValue)
				case "gt":
					clauses = append(clauses, fmt.Sprintf("%s > $%d", colRef, offset+len(args)+1))
					args = append(args, argValue)
				case "gte":
					clauses = append(clauses, fmt.Sprintf("%s >= $%d", colRef, offset+len(args)+1))
					args = append(args, argValue)
				case "lt":
					clauses = append(clauses, fmt.Sprintf("%s < $%d", colRef, offset+len(args)+1))
					args = append(args, argValue)
				case "lte":
					clauses = append(clauses, fmt.Sprintf("%s <= $%d", colRef, offset+len(args)+1))
					args = append(args, argValue)
				case "like":
					clauses = append(clauses, fmt.Sprintf("%s LIKE $%d", colRef, offset+len(args)+1))
					args = append(args, argValue)
				case "ilike":
					clauses = append(clauses, fmt.Sprintf("%s ILIKE $%d", colRef, offset+len(args)+1))
					args = append(args, argValue)
				case "in":
					// Handle IN clause: in.(1,2,3) - strip parentheses
					argValue = strings.TrimPrefix(argValue, "(")
					argValue = strings.TrimSuffix(argValue, ")")
					inValues := strings.Split(argValue, ",")

					// Infer data type from the first non-empty value
					// If all values look like integers, cast to integer, otherwise use text
					allIntegers := true
					for _, v := range inValues {
						trimmed := strings.TrimSpace(v)
						if trimmed == "" {
							continue
						}
						// Check if the value is a valid integer (optional negative sign)
						_, err := strconv.ParseInt(trimmed, 10, 64)
						if err != nil {
							allIntegers = false
							break
						}
					}

					// Build IN clause with proper casting for each element
					inClauses := make([]string, len(inValues))
					baseIdx := offset + len(args) // Calculate base before loop
					for i, v := range inValues {
						paramIdx := baseIdx + i + 1
						if allIntegers {
							// Cast each parameter to integer using CAST syntax
							inClauses[i] = fmt.Sprintf("CAST($%d AS integer)", paramIdx)
						} else {
							// Cast each parameter to text using CAST syntax
							inClauses[i] = fmt.Sprintf("CAST($%d AS text)", paramIdx)
						}
						args = append(args, v)
					}

					// Use simple IN clause instead of ANY - this avoids type ambiguity
					clauses = append(clauses, fmt.Sprintf("%s IN (%s)", colRef, strings.Join(inClauses, ", ")))
				default:
					// Unknown operator, treat as direct equality
					clauses = append(clauses, fmt.Sprintf("%s = $%d", colRef, offset+len(args)+1))
					args = append(args, value)
				}
				continue
			}
		}

		// No operator specified, use direct equality with JSON support
		var colRef string
		if strings.Contains(key, "->>") {
			jsonParts := strings.SplitN(key, "->>", 2)
			colRef = fmt.Sprintf("%s->>'%s'", quoteIdentifier(jsonParts[0]), jsonParts[1])
		} else if strings.Contains(key, "->") {
			jsonParts := strings.SplitN(key, "->", 2)
			colRef = fmt.Sprintf("%s->'%s'", quoteIdentifier(jsonParts[0]), jsonParts[1])
		} else {
			colRef = quoteIdentifier(key)
		}
		clauses = append(clauses, fmt.Sprintf("%s = $%d", colRef, offset+len(args)+1))
		args = append(args, value)
	}

	if len(clauses) > 0 {
		return strings.Join(clauses, " AND "), args
	}
	return "", nil
}

// handlePOST processes INSERT and UPSERT requests
func (s *Server) handlePOST(ctx context.Context, conn *pgx.Conn, w http.ResponseWriter, r *http.Request, table string) {
	// Quote table name for SQL
	quotedTable := quoteIdentifier(table)

	// Decode JSON body - can be single object or array
	var rawData interface{}
	if err := json.NewDecoder(r.Body).Decode(&rawData); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	// Convert to array for uniform processing
	var records []map[string]interface{}
	switch v := rawData.(type) {
	case map[string]interface{}:
		records = []map[string]interface{}{v}
	case []interface{}:
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				records = append(records, m)
			}
		}
	default:
		http.Error(w, "invalid JSON format", http.StatusBadRequest)
		return
	}

	if len(records) == 0 {
		http.Error(w, "no data provided", http.StatusBadRequest)
		return
	}

	// Check for UPSERT via on_conflict query parameter or Prefer header
	query := r.URL.Query()
	onConflict := query.Get("on_conflict")

	// Check for Prefer header - Supabase uses this to indicate upsert
	prefer := r.Header.Get("Prefer")
	isUpsert := strings.Contains(prefer, "resolution=merge-duplicates") || strings.Contains(prefer, "resolution=ignore-duplicates")
	ignoreDuplicates := strings.Contains(prefer, "resolution=ignore-duplicates")

	// Get all unique columns from all records
	colMap := make(map[string]bool)
	for _, record := range records {
		for col := range record {
			colMap[col] = true
		}
	}
	columns := make([]string, 0, len(colMap))
	for col := range colMap {
		columns = append(columns, quoteIdentifier(col))
	}

	// Build VALUES clauses and collect values
	valueSets := make([]string, 0, len(records))
	values := make([]interface{}, 0)
	paramIdx := 1
	for _, record := range records {
		placeholders := make([]string, 0, len(columns))
		for _, col := range columns {
			colName := strings.Trim(col, "\"")
			val := record[colName]
			placeholders = append(placeholders, fmt.Sprintf("$%d", paramIdx))
			values = append(values, val)
			paramIdx++
		}
		valueSets = append(valueSets, fmt.Sprintf("(%s)", strings.Join(placeholders, ", ")))
	}

	// Parse select parameter for RETURNING clause (Supabase compatibility)
	var returningClause string
	if selectVals := query["select"]; len(selectVals) > 0 {
		selectCols := strings.Split(selectVals[0], ",")
		quotedCols := make([]string, 0, len(selectCols))
		for _, col := range selectCols {
			col = strings.TrimSpace(col)
			if col != "*" {
				quotedCols = append(quotedCols, quoteIdentifier(col))
			} else {
				quotedCols = append(quotedCols, "*")
			}
		}
		returningClause = strings.Join(quotedCols, ", ")
	} else {
		returningClause = "*"
	}

	// Determine conflict target for UPSERT (quote it)
	conflictTarget := onConflict
	if (onConflict != "" || isUpsert) && conflictTarget == "" {
		// If on_conflict not specified, try to infer primary key
		// Common primary key names to try
		for _, pk := range []string{"id", "ID", "Id", "pk", "PK"} {
			for _, col := range columns {
				colName := strings.Trim(col, "\"")
				if colName == pk {
					conflictTarget = colName
					break
				}
			}
			if conflictTarget != "" {
				break
			}
		}
		// If still no conflict target, use first column
		if conflictTarget == "" && len(columns) > 0 {
			conflictTarget = strings.Trim(columns[0], "\"")
		}
	}

	var sqlQuery string
	if onConflict != "" || isUpsert {
		// UPSERT: INSERT ... ON CONFLICT ... DO UPDATE

		// Build the UPDATE SET clause for conflicting rows
		updateSets := make([]string, 0)
		for _, col := range columns {
			colName := strings.Trim(col, "\"")
			// Skip the conflict target column(s) in the update
			if !strings.Contains(conflictTarget, colName) {
				updateSets = append(updateSets, fmt.Sprintf("%s = EXCLUDED.%s", col, col))
			}
		}

		if ignoreDuplicates {
			// ON CONFLICT DO NOTHING (when ignoreDuplicates is true, always DO NOTHING)
			sqlQuery = fmt.Sprintf("INSERT INTO public.%s (%s) VALUES %s ON CONFLICT (%s) DO NOTHING RETURNING %s",
				quotedTable,
				strings.Join(columns, ", "),
				strings.Join(valueSets, ", "),
				quoteIdentifier(conflictTarget),
				returningClause)
		} else {
			// ON CONFLICT ... DO UPDATE SET ...
			sqlQuery = fmt.Sprintf("INSERT INTO public.%s (%s) VALUES %s ON CONFLICT (%s) DO UPDATE SET %s RETURNING %s",
				quotedTable,
				strings.Join(columns, ", "),
				strings.Join(valueSets, ", "),
				quoteIdentifier(conflictTarget),
				strings.Join(updateSets, ", "),
				returningClause)
		}
	} else {
		// Regular INSERT
		sqlQuery = fmt.Sprintf("INSERT INTO public.%s (%s) VALUES %s RETURNING %s",
			quotedTable,
			strings.Join(columns, ", "),
			strings.Join(valueSets, ", "),
			returningClause)
	}

	// Execute query
	rows, err := conn.Query(ctx, sqlQuery, values...)
	if err != nil {
		http.Error(w, fmt.Sprintf("insert error: %v", err), http.StatusBadRequest)
		return
	}
	defer rows.Close()

	// Fetch all results
	results := make([]map[string]interface{}, 0)
	for rows.Next() {
		row, err := rows.Values()
		if err != nil {
			http.Error(w, fmt.Sprintf("row scan error: %v", err), http.StatusInternalServerError)
			return
		}

		desc := rows.FieldDescriptions()
		result := make(map[string]interface{})
		for i, col := range desc {
			result[col.Name] = row[i]
		}
		results = append(results, result)
	}

	// Special handling for ignoreDuplicates: if no rows returned (conflict occurred),
	// fetch the existing row to match Supabase behavior
	if ignoreDuplicates && len(results) == 0 && len(records) == 1 && conflictTarget != "" {
		// Build WHERE clause to fetch the conflicting row
		record := records[0]
		var whereClauses []string
		var whereArgs []interface{}
		argIdx := 1

		// Use the conflict target column value to find the existing row
		conflictColValue := record[conflictTarget]
		if conflictColValue != nil {
			whereClauses = append(whereClauses, fmt.Sprintf("\"%s\" = $%d", conflictTarget, argIdx))
			whereArgs = append(whereArgs, conflictColValue)
			argIdx++
		}

		if len(whereClauses) > 0 {
			selectQuery := fmt.Sprintf("SELECT %s FROM public.%s WHERE %s",
				returningClause, table, strings.Join(whereClauses, " AND "))

			selectRows, err := conn.Query(ctx, selectQuery, whereArgs...)
			if err == nil {
				defer selectRows.Close()
				for selectRows.Next() {
					row, err := selectRows.Values()
					if err != nil {
						break
					}
					desc := selectRows.FieldDescriptions()
					result := make(map[string]interface{})
					for i, col := range desc {
						result[col.Name] = row[i]
					}
					results = append(results, result)
				}
			}
		}
	}

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(results)
}

// handlePATCH processes UPDATE requests
func (s *Server) handlePATCH(ctx context.Context, conn *pgx.Conn, w http.ResponseWriter, r *http.Request, table string) {
	// Quote table name for SQL
	quotedTable := quoteIdentifier(table)

	var data map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	// Parse select columns for returning clause (Supabase supports .select() after update)
	query := r.URL.Query()
	var returningClause string
	if selectVals := query["select"]; len(selectVals) > 0 {
		columns := strings.Split(selectVals[0], ",")
		if len(columns) == 1 && columns[0] == "*" {
			returningClause = "*"
		} else {
			quotedCols := make([]string, 0, len(columns))
			for _, col := range columns {
				col = strings.TrimSpace(col)
				if col != "*" {
					quotedCols = append(quotedCols, quoteIdentifier(col))
				} else {
					quotedCols = append(quotedCols, "*")
				}
			}
			returningClause = strings.Join(quotedCols, ", ")
		}
	} else {
		returningClause = "*"
	}

	// Build UPDATE query
	var sets []string
	args := []interface{}{}
	i := 1
	for col, val := range data {
		sets = append(sets, fmt.Sprintf("%s = $%d", quoteIdentifier(col), i))
		args = append(args, val)
		i++
	}

	// Add WHERE clause from query parameters (offset by number of SET parameters)
	whereClause, whereArgs := s.buildWhereClause(query, len(args))
	if whereClause == "" {
		http.Error(w, "missing filter", http.StatusBadRequest)
		return
	}
	args = append(args, whereArgs...)

	sqlQuery := fmt.Sprintf("UPDATE public.%s SET %s WHERE %s RETURNING %s",
		quotedTable,
		strings.Join(sets, ", "),
		whereClause,
		returningClause)

	// Execute query
	rows, err := conn.Query(ctx, sqlQuery, args...)
	if err != nil {
		http.Error(w, fmt.Sprintf("update error: %v", err), http.StatusBadRequest)
		return
	}
	defer rows.Close()

	// Get all updated rows (return as array like Supabase)
	results := make([]map[string]interface{}, 0)
	for rows.Next() {
		row, err := rows.Values()
		if err != nil {
			http.Error(w, fmt.Sprintf("row scan error: %v", err), http.StatusInternalServerError)
			return
		}

		// Convert to map
		desc := rows.FieldDescriptions()
		result := make(map[string]interface{})
		for i, col := range desc {
			result[col.Name] = row[i]
		}
		results = append(results, result)
	}

	// Return JSON response (empty array if no rows matched, not an error)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(results)
}

// handleDELETE processes DELETE requests
func (s *Server) handleDELETE(ctx context.Context, conn *pgx.Conn, w http.ResponseWriter, r *http.Request, table string) {
	// Quote table name for SQL
	quotedTable := quoteIdentifier(table)

	// Parse select columns for returning clause (Supabase supports .select() after delete)
	query := r.URL.Query()
	var returningClause string
	if selectVals := query["select"]; len(selectVals) > 0 {
		columns := strings.Split(selectVals[0], ",")
		if len(columns) == 1 && columns[0] == "*" {
			returningClause = "*"
		} else {
			quotedCols := make([]string, 0, len(columns))
			for _, col := range columns {
				col = strings.TrimSpace(col)
				if col != "*" {
					quotedCols = append(quotedCols, quoteIdentifier(col))
				} else {
					quotedCols = append(quotedCols, "*")
				}
			}
			returningClause = strings.Join(quotedCols, ", ")
		}
	} else {
		returningClause = "*"
	}

	// Add WHERE clause from query parameters
	whereClause, whereArgs := s.buildWhereClause(query, 0)
	if whereClause == "" {
		http.Error(w, "missing filter", http.StatusBadRequest)
		return
	}

	// Build DELETE query
	sqlQuery := fmt.Sprintf("DELETE FROM public.%s WHERE %s RETURNING %s",
		quotedTable,
		whereClause,
		returningClause)

	// Execute query
	rows, err := conn.Query(ctx, sqlQuery, whereArgs...)
	if err != nil {
		http.Error(w, fmt.Sprintf("delete error: %v", err), http.StatusBadRequest)
		return
	}
	defer rows.Close()

	// Get deleted rows
	results := make([]map[string]interface{}, 0)
	for rows.Next() {
		row, err := rows.Values()
		if err != nil {
			http.Error(w, fmt.Sprintf("row scan error: %v", err), http.StatusInternalServerError)
			return
		}

		desc := rows.FieldDescriptions()
		result := make(map[string]interface{})
		for i, col := range desc {
			result[col.Name] = row[i]
		}
		results = append(results, result)
	}

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(results)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"healthy"}`)
}

func (s *Server) initSchema(ctx context.Context) error {
	conn, err := s.pgDatabase.Connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	_, err = conn.Exec(ctx, `
		CREATE SCHEMA IF NOT EXISTS auth;
		CREATE SCHEMA IF NOT EXISTS storage;
		CREATE SCHEMA IF NOT EXISTS public;

		-- Captured emails table for development/testing
		CREATE TABLE IF NOT EXISTS public.captured_emails (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			from_addr TEXT NOT NULL,
			to_addr TEXT NOT NULL,
			subject TEXT,
			text_body TEXT,
			html_body TEXT,
			raw_message BYTEA
		);

		CREATE INDEX IF NOT EXISTS captured_emails_created_at_idx
			ON public.captured_emails(created_at DESC);

		CREATE INDEX IF NOT EXISTS captured_emails_to_addr_idx
			ON public.captured_emails(to_addr);
	`)
	return err
}

func (s *Server) waitForShutdown(ctx context.Context) error {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Info("received signal, shutting down...", "signal", sig)
	case <-ctx.Done():
		return ctx.Err()
	}

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if s.httpServer != nil {
		s.httpServer.Shutdown(shutdownCtx)
	}

	if s.authServer != nil {
		_ = s.authServer.Stop()
	}

	if s.captureServer != nil {
		_ = s.captureServer.Stop()
	}

	if s.prestServer != nil {
		s.prestServer.Stop()
	}

	if s.pgDatabase != nil {
		s.pgDatabase.Stop()
	}

	log.Info("Supalite stopped")
	return nil
}

// generateRandomSecret generates a random secret string of specified length
func generateRandomSecret(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	randomBytes := make([]byte, length)
	if _, err := cryptoRand.Read(randomBytes); err != nil {
		// Fallback to less secure but working method
		for i := range b {
			b[i] = charset[i%len(charset)]
		}
		return string(b)
	}
	for i := range b {
		b[i] = charset[randomBytes[i]%byte(len(charset))]
	}
	return string(b)
}
