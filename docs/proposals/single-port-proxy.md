# Single-Port Proxy Architecture Proposal

**Status:** Under Consideration
**Date:** 2026-01-29
**Author:** Claude Opus 4.5

---

## Overview

This document analyzes the feasibility of consolidating all Supalite services onto a single network port using protocol multiplexing. Currently, Supalite exposes multiple ports for different protocols. This proposal explores running HTTP, PostgreSQL, and optionally SMTP on a single port.

---

## Current Architecture

### Port Usage Summary

| Port | Service | Protocol | Exposure | Purpose |
|------|---------|----------|----------|---------|
| 8080 | Main HTTP Server | HTTP/1.1 | External | REST API, Auth API, JWKS, Health |
| 5432 | PostgreSQL | PG Wire Protocol | External | Direct database access |
| 3000 | pREST | HTTP | Internal only | PostgREST-compatible backend |
| 9999 | GoTrue | HTTP | Internal only | Auth server subprocess |
| 1025 | Mail Capture | SMTP | Internal only | Development email capture |

### Current Request Flow

```
External Clients
       │
       ├──── HTTP (port 8080) ────► Main Server Router
       │                                  │
       │                           ┌──────┴──────┐
       │                           │             │
       │                           ▼             ▼
       │                      /rest/v1/*    /auth/v1/*
       │                      (pREST)       (GoTrue)
       │
       └──── PostgreSQL (port 5432) ────► Embedded PostgreSQL

Internal Only:
  GoTrue ──── SMTP (port 1025) ────► Mail Capture Server
```

### What's Already Multiplexed

The main HTTP server (port 8080) already multiplexes multiple APIs using path-based routing:

- `GET /health` - Health check endpoint
- `GET /.well-known/jwks.json` - JWKS public key discovery
- `/rest/v1/*` - REST API (proxied to internal pREST)
- `/auth/v1/*` - Auth API (reverse proxied to internal GoTrue)

---

## Proposed Architecture

### Single External Port

```
                    ┌─────────────────────────────────────┐
                    │         Single Port (8080)          │
                    │      Protocol Multiplexer (cmux)    │
                    └─────────────────────────────────────┘
                                    │
           ┌────────────────────────┼────────────────────────┐
           │                        │                        │
           ▼                        ▼                        ▼
    ┌─────────────┐         ┌─────────────┐         ┌─────────────┐
    │    HTTP     │         │  PostgreSQL │         │    SMTP     │
    │   Router    │         │    Proxy    │         │   Server    │
    └─────────────┘         └─────────────┘         └─────────────┘
           │                        │                        │
    ┌──────┴──────┐                 │                        │
    │             │                 │                        │
    ▼             ▼                 ▼                        ▼
 /rest/v1     /auth/v1      Embedded PG              Mail Capture
 (pREST)      (GoTrue)       (internal)               (database)
```

### Protocol Detection Strategy

Different protocols have distinct signatures in their initial bytes:

| Protocol | Detection Method | First Bytes |
|----------|------------------|-------------|
| HTTP | ASCII method names | `GET `, `POST`, `PUT `, `DELETE`, `HEAD`, `OPTIONS`, `PATCH` |
| PostgreSQL | Binary startup message | SSLRequest or StartupMessage format |
| SMTP | Server-speaks-first | Client waits silently (timeout-based detection) |

---

## Protocol-Specific Analysis

### HTTP Protocol

**Status:** Already working via Chi router

**Detection:** Check if first bytes match HTTP method names:
```go
func isHTTP(buf []byte) bool {
    methods := []string{"GET ", "POST", "PUT ", "DELE", "HEAD", "OPTI", "PATC", "CONN", "TRAC"}
    s := string(buf)
    for _, m := range methods {
        if strings.HasPrefix(s, m) {
            return true
        }
    }
    return false
}
```

**Risks:** Low - standard protocol, well-understood

---

### PostgreSQL Protocol

**Status:** Requires new proxy implementation

#### Wire Protocol Overview

PostgreSQL uses a binary protocol. Client connections begin with one of:

1. **SSLRequest** (for TLS negotiation):
   ```
   Bytes 0-3: Length = 8 (big-endian int32)
   Bytes 4-7: Code = 80877103 (0x04D2162F)
   ```

2. **StartupMessage** (for non-TLS or after TLS):
   ```
   Bytes 0-3: Length (big-endian int32)
   Bytes 4-7: Protocol version = 196608 (0x00030000 for v3.0)
   Bytes 8+:  Parameter name=value pairs (null-terminated)
   ```

3. **CancelRequest** (on separate connection):
   ```
   Bytes 0-3: Length = 16 (big-endian int32)
   Bytes 4-7: Code = 80877102 (0x04D2162E)
   Bytes 8-11: Process ID
   Bytes 12-15: Secret Key
   ```

#### Detection Implementation

```go
func isPostgres(buf []byte) bool {
    if len(buf) < 8 {
        return false
    }

    // Check for SSLRequest (80877103) or CancelRequest (80877102)
    code := binary.BigEndian.Uint32(buf[4:8])
    if code == 80877103 || code == 80877102 {
        return true
    }

    // Check for StartupMessage (protocol version 3.0 = 196608)
    if code == 196608 {
        return true
    }

    return false
}
```

#### SSL/TLS Negotiation Complexity

PostgreSQL has a unique SSL handshake that occurs **before** standard TLS:

```
Client                                  Server
   │                                      │
   │──── SSLRequest (80877103) ──────────►│
   │                                      │
   │◄─────────── 'S' or 'N' ─────────────│
   │         (S=accept SSL, N=reject)     │
   │                                      │
   │◄═══════ TLS Handshake ══════════════►│  (only if 'S')
   │                                      │
   │──── StartupMessage ─────────────────►│
   │      (protocol version, user, db)    │
```

**Problem:** If the multiplexer terminates TLS globally, PostgreSQL's custom SSL negotiation must be handled specially.

**Solutions:**

1. **Pass-through mode**: Don't terminate TLS at multiplexer; forward encrypted connections
2. **PostgreSQL-aware proxy**: Understand and respond to SSLRequest before proxying
3. **Disable SSL for PG**: Force `sslmode=disable` (acceptable for localhost/embedded use)

#### Client Compatibility Concerns

| Client/Tool | Risk Level | Notes |
|-------------|------------|-------|
| **pgx (Go)** | Low | Generally works well through proxies |
| **psycopg2/3 (Python)** | Low | Works, but SSL mode configuration matters |
| **node-postgres** | Low | Handles proxied connections well |
| **JDBC** | Medium | Some versions do protocol-level checks |
| **PgBouncer** | High | May not work behind generic proxy |
| **pg_dump/pg_restore** | Medium | Connection reuse through proxy can fail |
| **psql** | Low | Works well, common use case |

#### Protocol State Considerations

PostgreSQL's wire protocol is **stateful** with multiple message types:

```
StartupMessage → AuthenticationRequest → PasswordMessage →
AuthenticationOk → ParameterStatus* → BackendKeyData → ReadyForQuery
```

**Potential Issues:**

1. **COPY operations**: Use a different sub-protocol with raw data transfer
2. **Streaming replication**: Has its own protocol extension
3. **Cancellation requests**: Arrive on a **separate TCP connection** with CancelRequest message
4. **Extended query protocol**: Prepared statements have multi-step message sequences

**For Supalite's use case**, most of these are manageable since:
- Embedded PostgreSQL is controlled
- Primary access is through REST API, not direct PG connections
- Replication is not a use case

#### Authentication Considerations

```
Client → Proxy → Embedded PG
         │
         └── Who authenticates?
```

**Options:**

1. **Pass-through**: Proxy forwards auth to PG (simplest)
2. **Dual auth**: Proxy authenticates, then PG authenticates (most secure)
3. **Trust local**: Configure PG to trust connections from proxy (least secure)

**Recommendation:** Pass-through authentication. Since Supalite controls the embedded PG configuration, this is safe and simple.

---

### SMTP Protocol

**Status:** Complex due to server-speaks-first nature

#### The Server-Speaks-First Problem

SMTP is fundamentally different from HTTP and PostgreSQL:

```
SMTP Session:
  Client connects...
  (client waits silently)
                          Server: 220 mail.example.com ESMTP ready
  Client: EHLO client.example.com
                          Server: 250-mail.example.com Hello
                          Server: 250-STARTTLS
                          Server: 250 OK
  Client: MAIL FROM:<sender@example.com>
  ...
```

**Problem:** Protocol multiplexing typically peeks at the first bytes from the **client** to determine the protocol. SMTP clients wait silently for the server greeting.

#### Detection Strategies

**Option A: Timeout-Based Detection**

```go
func detectProtocol(conn net.Conn) Protocol {
    // Set short read deadline
    conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))

    buf := make([]byte, 8)
    n, err := conn.Read(buf)

    // Clear deadline for subsequent operations
    conn.SetReadDeadline(time.Time{})

    if errors.Is(err, os.ErrDeadlineExceeded) {
        // Client didn't send anything - probably SMTP
        return ProtocolSMTP
    }

    if isHTTP(buf[:n]) {
        return ProtocolHTTP
    }
    if isPostgres(buf[:n]) {
        return ProtocolPostgres
    }

    return ProtocolUnknown
}
```

**Problems with timeout approach:**

| Issue | Impact | Mitigation |
|-------|--------|------------|
| 100ms delay for every SMTP connection | Noticeable latency | Acceptable for dev-only feature |
| Slow/high-latency HTTP clients misidentified as SMTP | Broken requests | Increase timeout (trades off SMTP latency) |
| Race conditions with eager clients | Intermittent failures | Careful buffer management |

**Option B: Keep SMTP Separate**

```
Port 8080: HTTP + PostgreSQL (client-speaks-first protocols)
Port 1025: SMTP only (internal, not exposed externally)
```

**Rationale:**
- Mail capture is internal-only (GoTrue → localhost:1025)
- SMTP complexity isn't worth it for an internal feature
- Users don't connect to SMTP from outside

**Recommendation:** Keep SMTP on a separate internal port.

**Option C: Explicit Protocol Header**

Require clients to send a protocol identifier byte first:

```
0x00 = HTTP
0x01 = PostgreSQL
0x02 = SMTP
```

**Problems:**
- Breaks standard clients
- Requires custom client modifications
- Not practical

#### STARTTLS Complication

SMTP can upgrade to TLS mid-connection:

```
Client: EHLO example.com
Server: 250-STARTTLS
        250 OK
Client: STARTTLS
Server: 220 Ready to start TLS
<TLS handshake happens here>
Client: EHLO example.com  (again, now encrypted)
```

If TLS is terminated at the multiplexer level, this upgrade must be handled.

**For Supalite:** Not an issue since mail capture runs locally without TLS.

---

## Implementation Approach

### Recommended Library: cmux

The [soheilhy/cmux](https://github.com/soheilhy/cmux) library is designed for connection multiplexing:

```go
import "github.com/soheilhy/cmux"

func main() {
    // Create listener on single port
    l, err := net.Listen("tcp", ":8080")
    if err != nil {
        log.Fatal(err)
    }

    // Create multiplexer
    m := cmux.New(l)

    // Match PostgreSQL connections (must be first - binary protocol)
    pgMatcher := func(r io.Reader) bool {
        buf := make([]byte, 8)
        n, _ := io.ReadFull(r, buf)
        return n == 8 && isPostgres(buf)
    }
    pgL := m.Match(cmux.MatcherFunc(pgMatcher))

    // Match HTTP connections
    httpL := m.Match(cmux.HTTP1Fast())

    // Start services on respective listeners
    go pgProxy.Serve(pgL)
    go httpServer.Serve(httpL)

    // Start multiplexer
    m.Serve()
}
```

### PostgreSQL Proxy Implementation

A minimal PostgreSQL proxy needs to:

1. Accept connections from multiplexer
2. Handle SSLRequest (respond 'N' to disable or 'S' and upgrade)
3. Forward all bytes bidirectionally to embedded PostgreSQL
4. Handle connection cleanup

```go
type PGProxy struct {
    targetAddr string // e.g., "localhost:5432"
}

func (p *PGProxy) Serve(l net.Listener) error {
    for {
        clientConn, err := l.Accept()
        if err != nil {
            return err
        }
        go p.handleConnection(clientConn)
    }
}

func (p *PGProxy) handleConnection(client net.Conn) {
    defer client.Close()

    // Connect to actual PostgreSQL
    server, err := net.Dial("tcp", p.targetAddr)
    if err != nil {
        log.Printf("Failed to connect to PostgreSQL: %v", err)
        return
    }
    defer server.Close()

    // Bidirectional copy
    var wg sync.WaitGroup
    wg.Add(2)

    go func() {
        defer wg.Done()
        io.Copy(server, client)
    }()

    go func() {
        defer wg.Done()
        io.Copy(client, server)
    }()

    wg.Wait()
}
```

**Note:** This basic proxy forwards bytes directly. For SSL handling, additional logic is needed to intercept and respond to SSLRequest messages.

---

## Risk Assessment

### PostgreSQL Multiplexing Risks

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| SSL negotiation breaks | Medium | High | Disable SSL for local PG or implement PG-aware SSL handling |
| CancelRequest routing fails | Low | Medium | Detect CancelRequest signature and route correctly |
| COPY operations fail | Low | Medium | Ensure bidirectional streaming works correctly |
| Connection pooler incompatibility | Medium | Low | Document limitations; most users use REST API |
| Performance overhead | Low | Low | Minimal for connection setup; no per-query overhead |

### SMTP Multiplexing Risks

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Timeout misidentifies protocol | Medium | High | Keep SMTP on separate port (recommended) |
| Slow clients break | Medium | Medium | Increase timeout; accept latency trade-off |
| STARTTLS handling fails | N/A | N/A | Not applicable for local mail capture |

### Overall Assessment

| Component | Difficulty | Recommendation |
|-----------|------------|----------------|
| HTTP multiplexing | Already done | Continue using path-based routing |
| PostgreSQL multiplexing | Medium | Implement with cmux; disable SSL |
| SMTP multiplexing | Hard | Keep on separate internal port |

---

## Implementation Estimate

| Task | Effort | Notes |
|------|--------|-------|
| Add cmux dependency | 30 min | Simple go get |
| Create PostgreSQL proxy | 2-4 hours | Basic bidirectional proxy |
| Integrate protocol detection | 1-2 hours | Wire up cmux matchers |
| Handle PG SSL negotiation | 2-3 hours | Intercept SSLRequest, respond 'N' |
| Add configuration options | 1-2 hours | --single-port flag, etc. |
| Testing and edge cases | 2-4 hours | Various PG clients, error handling |
| Documentation | 1 hour | Update CLAUDE.md, README |

**Total estimate:** 10-16 hours of focused work

---

## Configuration Design

### Proposed Flags

```
--single-port           Enable single-port mode (HTTP + PostgreSQL on same port)
--single-port-pg        Enable PostgreSQL on main HTTP port (default: true when --single-port)
--pg-ssl-mode           PostgreSQL SSL mode: disable, prefer, require (default: disable in single-port)
```

### Example Configuration

```json
{
  "single_port": true,
  "port": 8080,
  "pg_ssl_mode": "disable"
}
```

### Backward Compatibility

- Default behavior unchanged (separate ports)
- Single-port mode is opt-in
- Existing configurations continue to work

---

## Alternatives Considered

### 1. HAProxy/nginx in Front

Use a dedicated reverse proxy with protocol detection.

**Pros:**
- Battle-tested solutions
- No code changes to Supalite

**Cons:**
- Additional deployment dependency
- Defeats "single binary" goal
- More complex configuration

### 2. TLS with ALPN

Use TLS Application-Layer Protocol Negotiation.

**Pros:**
- Standard-based protocol selection
- Works with any TLS client

**Cons:**
- Requires TLS for all connections
- PostgreSQL clients may not support ALPN
- More complex certificate management

### 3. Different Ports on Same IP

Keep separate ports but document clearly.

**Pros:**
- No implementation work
- Already working

**Cons:**
- Doesn't address user's concern about multiple ports
- More complex firewall rules

---

## Recommendation

### Implement HTTP + PostgreSQL Multiplexing

1. **Add cmux-based protocol detection** to distinguish HTTP from PostgreSQL
2. **Create a simple PostgreSQL proxy** that forwards connections to embedded PG
3. **Disable SSL for PostgreSQL** in single-port mode (acceptable for local/embedded use)
4. **Keep SMTP internal** - no benefit to exposing it externally
5. **Make it opt-in** via `--single-port` flag

### Benefits

- Simplifies firewall configuration (one port)
- Easier container/Kubernetes deployment
- Maintains backward compatibility
- Aligns with "single binary" philosophy

### Trade-offs Accepted

- No PostgreSQL SSL in single-port mode
- Slight complexity increase in connection handling
- Some PostgreSQL tools may not work (document limitations)

---

## References

- [cmux - Connection Multiplexer](https://github.com/soheilhy/cmux)
- [PostgreSQL Wire Protocol](https://www.postgresql.org/docs/current/protocol.html)
- [PostgreSQL SSLRequest](https://www.postgresql.org/docs/current/protocol-flow.html#id-1.10.6.7.11)
- [SMTP RFC 5321](https://tools.ietf.org/html/rfc5321)
- [sslh - Protocol Multiplexer](https://github.com/yrutschle/sslh)
