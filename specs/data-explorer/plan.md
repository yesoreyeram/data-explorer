# Data Explorer — Technical Implementation Plan

> **Spec-Kit artifact** — This document describes *how* to build Data Explorer. It translates the product requirements in `spec.md` into concrete technology choices, architecture decisions, and component designs. For governing principles, see `.specify/memory/constitution.md`. For the implementation task list, see `tasks.md`.

---

## Table of Contents

1. [Technology Stack](#1-technology-stack)
2. [Backend Architecture](#2-backend-architecture)
3. [Frontend Architecture](#3-frontend-architecture)
4. [Authentication and Session Design](#4-authentication-and-session-design)
5. [RBAC Design](#5-rbac-design)
6. [Connection and Connector Design](#6-connection-and-connector-design)
7. [Workflow Engine Design](#7-workflow-engine-design)
8. [Scheduler Design](#8-scheduler-design)
9. [Observability Design](#9-observability-design)
10. [Database and Migration Strategy](#10-database-and-migration-strategy)
11. [Security Implementation](#11-security-implementation)
12. [Deployment Topology](#12-deployment-topology)
13. [Risk Register](#13-risk-register)

---

## 1. Technology Stack

### Rationale for key choices

| Layer | Choice | Rationale |
|---|---|---|
| Backend language | **Go 1.25+** | Single binary, low memory footprint, strong concurrency primitives, fast startup (critical for container environments), excellent standard library for HTTP and crypto |
| HTTP router | **chi** | Lightweight, idiomatic, middleware-composable; avoids magic of larger frameworks while providing URL parameter matching and group routing |
| Database driver | **pgx v5** | Native PostgreSQL protocol; no cgo; better performance than `database/sql` + lib/pq; supports PostgreSQL-specific types (JSONB, UUID) natively |
| Password hashing | **Argon2id** | OWASP-recommended memory-hard KDF; resists GPU and ASIC attacks |
| JWT | **HS256** | Symmetric signing; sufficient for single-issuer deployment; avoids key distribution complexity of RS256 |
| Transform language | **JSONata (blues/jsonata-go)** | Well-defined semantics; sandboxed evaluation; no arbitrary code execution; familiar to data engineers |
| Frontend framework | **React 19 + TypeScript** | Dominant ecosystem for data-dense UIs; strong typing prevents entire classes of runtime errors |
| Build tool | **Vite** | Fast HMR; native ESM; tree-shaking; excellent TypeScript support |
| Canvas | **React Flow** | De facto standard for node-based UI; handles zoom, pan, edge routing; abstracts D3 complexity |
| Server state | **TanStack Query** | Caching, deduplication, background refresh; eliminates manual loading/error state management |
| Client state | **Zustand** | Minimal boilerplate; no provider wrapping; predictable for UI state (theme, sidebar) |
| Linter | **Oxlint** | ~50x faster than ESLint; catches common issues without plugin configuration overhead |
| Cloud SDKs | **aws-sdk-go-v2, cloud.google.com/go, azure-sdk-for-go** | Official SDKs; maintained by cloud providers; support ambient credential chains |

---

## 2. Backend Architecture

### 2.1 Package layout

```
backend/
  cmd/server/             # main.go — wiring only: construct services, start HTTP server
  db/migrations/          # embedded SQL migration files (golang-migrate format)
  pkg/
    dataframe/            # standalone pandas-style Frame/Schema; no internal/* imports
    httpclient/           # pluggable auth + pagination + retry; no internal/* imports
    egress/               # SSRF guard (resolve-then-dial); no internal/* imports
  internal/
    config/               # 12-factor env config; fails fast on missing required vars
    domain/               # pure entity structs; no business logic; no external imports
    platform/
      logger/             # slog setup; request-id injection
      crypto/             # AES-256-GCM encrypt/decrypt; Argon2id hash/verify
      dbx/                # database connection pool setup; health check
      migrator/           # embedded migration runner
      httpx/              # request decode/encode; error response helpers
    auth/                 # registration, login, JWT issue/verify, refresh token rotation
    rbac/                 # Principal struct, permission constants, RequirePermission middleware
    audit/                # append-only writer; query service
    connections/
      repository.go       # SQL CRUD + secret storage
      service.go          # business rules: encrypt/decrypt, classify errors, rate limit
      connectors/         # one package per connector type
        postgres/
        mysql/
        rest/
        graphql/
        aws/
        gcp/
        azure/
        sqlguard/         # read-only SQL enforcement
    catalog/              # static integration catalog (JSON embedded in binary)
    workflow/
      dag.go              # node/edge types; DAG validation; topological sort
      engine.go           # execution loop; node dispatch; guardrails
      nodes/              # one file per node type
    scheduler/            # poll loop; cron parsing; distributed lock stub
    observability/        # Prometheus registry; metric constructors
    quota/                # per-user rate / quota store
    adapters/             # thin wrappers adapting external types to internal interfaces
    api/
      middleware/         # auth, RBAC, rate-limit, request-id, panic-recovery
      handlers/           # one file per resource group
      router.go           # route registration; middleware wiring
```

### 2.2 Request lifecycle

```
HTTP request
  → chi router (URL matching)
  → middleware chain:
      RequestID        (generate/propagate X-Request-Id)
      RealIP           (extract real IP from X-Forwarded-For)
      Logger           (structured access log at request end)
      Recoverer        (panic → 500)
      RateLimiter      (per-IP token bucket)
      Authenticate     (JWT validation → set Principal in context)
      RequirePermission(check permission code against Principal)
  → handler:
      decode body (httpx.DecodeJSON)
      call service
      encode response (httpx.WriteJSON)
  → structured log line written (with request_id, actor, status, duration_ms)
```

### 2.3 Error taxonomy

All connector errors flow through `connections.Classify`, which maps low-level errors (timeout, DNS failure, TLS error, SQL error codes, HTTP status codes) to stable `HealthError` codes. This ensures the UI always shows a stable, actionable error code regardless of the underlying connector.

```go
type HealthError struct {
    Code        string  // one of: timeout, network_unreachable, auth_failed, ...
    Message     string  // human-readable
    Remediation string  // suggested fix
    Duration    time.Duration
}
```

---

## 3. Frontend Architecture

### 3.1 Page structure

```
frontend/src/
  pages/
    LoginPage.tsx
    RegisterPage.tsx
    DashboardPage.tsx
    ConnectionsPage.tsx      # list + create
    ConnectionDetailPage.tsx # edit + health panel + query
    ExploreQueryPage.tsx     # ad-hoc query builder
    WorkflowsPage.tsx        # list + create
    WorkflowBuilderPage.tsx  # React Flow canvas + execution panel
    AuditLogPage.tsx
    UsersPage.tsx
    SettingsPage.tsx
  components/
    ui/                      # design system primitives (Button, Input, Badge, etc.)
    DataFrameView.tsx        # tabular result display + export + charting
    DataTable.tsx            # virtualized table for large result sets
    Modal.tsx                # accessible focus-trap modal
    PermissionGate.tsx       # conditional render based on JWT permissions
    ProtectedRoute.tsx       # redirect to login if unauthenticated
    ThemeSwitcher.tsx        # light/dark/system toggle
    Sidebar.tsx              # collapsible navigation
    WorkflowCanvas.tsx       # React Flow wrapper + node types
  api/
    auth.ts                  # typed fetch wrappers for auth endpoints
    connections.ts
    workflows.ts
    explore.ts
    audit.ts
    users.ts
  state/
    auth.ts                  # Zustand: current user, token, permissions
    ui.ts                    # Zustand: theme, sidebar state
```

### 3.2 Authentication state machine

```
unauthenticated
  → POST /auth/login success → store accessToken in memory (NOT localStorage)
  → authenticated (accessToken in memory; refresh token in httpOnly cookie)

authenticated
  → accessToken expires → POST /auth/refresh (uses cookie) → new accessToken
  → refresh fails → unauthenticated (redirect to login)
  → POST /auth/logout → clear accessToken; redirect to login
```

**Key decision**: Access tokens are stored in React state (in-memory), never in `localStorage` or `sessionStorage`. This prevents XSS token theft. The refresh token in `httpOnly` cookie is not accessible to JavaScript.

### 3.3 Permission resolution

The JWT payload contains a `permissions` array. On login, permissions are decoded and stored in Zustand `auth` store. `<PermissionGate>` reads from this store and renders children only if the required permission is present. The check is purely client-side (UX) — server-side enforcement is the authoritative gate.

### 3.4 React Flow node architecture

Each node type is a React component registered with `nodeTypes` in `WorkflowCanvas.tsx`. Node configs are stored in the workflow definition JSONB; the canvas renders them from `initialNodes`/`initialEdges` passed by the builder page. On save, the `getNodes()`/`getEdges()` React Flow API is called to serialize the current state.

---

## 4. Authentication and Session Design

### 4.1 Registration flow

1. `POST /auth/register` → validate email format, password ≥ 8 chars, display name non-empty.
2. Hash password with Argon2id (memory=64MB, iterations=3, parallelism=2).
3. Insert user with `status='active'`; assign `viewer` role via `user_roles`.
4. Write audit log: `action=user.register`, `outcome=success`.
5. Return `201 Created` with user object (no token — user must log in).

### 4.2 Login flow

1. Look up user by email.
2. Verify password with constant-time Argon2id comparison.
3. If user not found or password wrong: record failed audit entry; wait for constant-time delay; return 401 with generic message.
4. Generate JWT: `sub=user_id`, `permissions=[...]`, `exp=now+15min`, signed with `JWT_SECRET`.
5. Generate refresh token: 32 random bytes, hex-encoded; store SHA-256 hash in `refresh_tokens` with `expires_at=now+7days`.
6. Set `httpOnly SameSite=Strict` cookie with refresh token value.
7. Return JWT in response body.

### 4.3 Refresh flow

1. Read refresh token from cookie.
2. Hash with SHA-256; look up in `refresh_tokens`.
3. If not found or expired: return 401.
4. Atomic rotation: delete old row; insert new row; issue new JWT.
5. Set new cookie.

### 4.4 OIDC flow

1. `GET /auth/oidc/{provider}` → generate `state` (CSRF) and PKCE `code_verifier`; store in `SameSite=Lax` cookies; redirect to provider.
2. `GET /auth/oidc/{provider}/callback` → validate `state`; exchange code with PKCE verifier for tokens; verify `email_verified=true`; upsert user; issue JWT + refresh cookie.

---

## 5. RBAC Design

### 5.1 Permission embedding

On JWT issue, query `user_roles → role_permissions → permissions` and embed permission codes in JWT claims. No per-request DB lookup. Permissions are re-embedded on refresh.

### 5.2 Middleware

```go
func RequirePermission(code string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            principal := rbac.FromContext(r.Context())
            if !principal.HasPermission(code) {
                httpx.WriteError(w, http.StatusForbidden, "forbidden", "")
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

### 5.3 Permission model

| Role | Permissions |
|---|---|
| `admin` | all 11 permission codes |
| `editor` | `connections:read`, `connections:write`, `connections:test`, `workflows:read`, `workflows:write`, `workflows:execute` |
| `viewer` | `connections:read`, `workflows:read` |

---

## 6. Connection and Connector Design

### 6.1 Connector interface

```go
type Connector interface {
    // Query executes a read query and returns a DataFrame.
    Query(ctx context.Context, config ConnectorConfig, query string) (*dataframe.Frame, error)
    // Validate checks the config for structural correctness without making a network call.
    Validate(config ConnectorConfig) error
}
```

### 6.2 Secret encryption

```go
// AES-256-GCM with a random 12-byte nonce prepended to the ciphertext.
// Key is a 32-byte value from CONNECTION_ENCRYPTION_KEY env var.
func Encrypt(key []byte, plaintext []byte) ([]byte, error)
func Decrypt(key []byte, ciphertext []byte) ([]byte, error)
```

Secrets are encrypted before any write to the database. They are decrypted in `connections.Service` immediately before being passed to a connector. The decrypted value never travels beyond the `Service` method call.

### 6.3 Error classification

`connections.Classify(err error) HealthError` maps:
- `context.DeadlineExceeded` → `timeout`
- DNS lookup failure → `network_unreachable`
- TLS handshake failure → `auth_failed`
- HTTP 401/403 → `auth_failed` / `permission_denied`
- HTTP 404 → `not_found`
- HTTP 429 → `rate_limited`
- SQL auth error codes → `auth_failed`
- Config parse errors → `invalid_config`
- All others → `unknown`

### 6.4 Authentication matrix per connector

| Type | Auth Methods |
|---|---|
| `postgres` | username/password; mTLS (client certificate); IAM (RDS via AWS SDK) |
| `mysql` | username/password; mTLS |
| `rest` | None; API Key (header/query param); Basic Auth; ******; OAuth 2.0 client credentials |
| `graphql` | Same as REST |
| `aws` | Static credentials; assumed role; instance profile / ECS task role (ambient) |
| `gcp` | Service account key JSON; workload identity (ambient) |
| `azure` | Service principal (client ID + secret); managed identity (ambient) |

### 6.5 SQL guard

`sqlguard.EnsureReadOnlySQL` validates user-supplied SQL using keyword analysis:
- Rejects any statement starting with a mutating keyword (`INSERT`, `UPDATE`, `DELETE`, `DROP`, `CREATE`, `ALTER`, `TRUNCATE`, `GRANT`, `REVOKE`).
- Rejects comments that attempt to hide injections.
- Applies to both `postgres` and `mysql` connectors.

---

## 7. Workflow Engine Design

### 7.1 Node types and responsibilities

| Node Type | Input | Output | Key Config |
|---|---|---|---|
| `source` | — | `*Frame` | connectionId, query |
| `filter` | `*Frame` | `*Frame` | JSONata predicate expression |
| `transform` | `*Frame` | `*Frame` | JSONata mapping expression |
| `join` | `[*Frame, *Frame]` | `*Frame` | joinType (inner/left/right), joinKeys |
| `aggregate` | `*Frame` | `*Frame` | groupBy columns, aggregate functions |
| `output` | `*Frame` | — | connectionId, targetTable (write-back; optional) |

### 7.2 DAG validation

Before execution (and on save), the engine validates:
1. No cycles (DFS-based cycle detection).
2. No disconnected nodes (all nodes reachable from at least one source).
3. Node count ≤ `MaxNodes` (default: 50).
4. Edge count ≤ `MaxEdges` (default: 200).
5. Each non-source node has exactly the required number of inputs (join requires 2; others require 1).

### 7.3 Execution algorithm

```
1. Validate DAG.
2. Compute topological order (Kahn's algorithm).
3. For each node in topological order:
   a. Gather input frames from upstream nodes.
   b. Execute node logic (connector query or transform).
   c. Cap output at MaxRowsPerNode (100,000 rows).
   d. Store result in execution context keyed by node ID.
   e. Record per-node timing and row count.
   f. If error: mark execution as 'failure'; stop loop.
4. Persist execution record with status, duration, node_results.
5. Write audit log entry.
```

### 7.4 JSONata sandboxing

JSONata expressions are evaluated using `blues/jsonata-go`. The evaluator runs in the Go process (no subprocess or network). Execution is bounded by the `MaxExecutionDuration` context timeout. There is no file system or network access from within a JSONata expression.

---

## 8. Scheduler Design

### 8.1 Poll loop

The scheduler runs as a goroutine in the same process:

```
every 30 seconds:
  SELECT id, definition FROM workflows
  WHERE schedule_enabled = true
    AND schedule_next_run <= now()
  ORDER BY schedule_next_run ASC
  LIMIT 10;

  for each workflow:
    attempt to acquire advisory lock (pg_try_advisory_lock(workflow_id))
    if acquired:
      execute workflow (same engine path as manual execution)
      update schedule_next_run = next cron tick after now()
      release lock
```

### 8.2 Distributed lock stub

For single-replica deployments, the advisory lock is sufficient (always acquired since only one instance is running). For multi-replica deployments, the advisory lock at the PostgreSQL level prevents duplicate executions as long as all replicas share the same PostgreSQL instance.

### 8.3 Cron parsing

Uses `robfig/cron/v3` for cron expression parsing and next-run computation. Standard 5-field cron format. `schedule_next_run` is pre-computed on schedule save, making the scheduler query cheap (indexed column lookup, not cron evaluation in DB).

---

## 9. Observability Design

### 9.1 Logging setup

- Production: structured JSON via `slog.NewJSONHandler(os.Stdout, ...)`.
- Development: text via `slog.NewTextHandler(os.Stderr, ...)`.
- `APP_ENV=production` controls the handler selection.
- Request logger middleware adds `request_id`, `actor`, `route`, `method`, `status`, `duration_ms` to each log line at request completion.

### 9.2 Prometheus metrics

All metrics are registered in `internal/observability/registry.go` and shared via a single `*prometheus.Registry`. The `GET /metrics` handler uses `promhttp.HandlerFor(registry, ...)`.

Route labels are normalized in the logging middleware to use chi's route pattern (e.g., `/connections/{id}`) not the raw path (e.g., `/connections/123e4567-...`), preventing cardinality explosion.

### 9.3 Health probe implementation

```go
// /healthz — no DB check; always returns 200
func HealthzHandler(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte(`{"status":"ok"}`))
}

// /readyz — pings DB; returns 200 or 503
func ReadyzHandler(db *pgxpool.Pool) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
        defer cancel()
        if err := db.Ping(ctx); err != nil {
            httpx.WriteError(w, http.StatusServiceUnavailable, "db_unavailable", err.Error())
            return
        }
        w.WriteHeader(http.StatusOK)
        w.Write([]byte(`{"status":"ok"}`))
    }
}
```

---

## 10. Database and Migration Strategy

### 10.1 Migration tool

Uses `golang-migrate` with SQL files embedded in the binary via `//go:embed`. On startup, `migrator.Run(db)` applies all pending migrations. Migrations are numbered sequentially (`000001_create_users.up.sql`, `000001_create_users.down.sql`).

### 10.2 Migration principles

- Every migration is idempotent (uses `IF NOT EXISTS`, `IF EXISTS`).
- Down migrations must undo the up migration completely.
- No data migrations that could fail silently (use explicit transactions).
- Foreign keys always defined at table creation (not added later via ALTER).
- Index names follow the convention: `idx_{table}_{columns}`.

### 10.3 Connection pooling

`pgxpool.New()` with configurable pool size (default: 25 connections). Pool is created once in `cmd/server/main.go` and injected into all repositories. The pool handles connection lifecycle; repositories never call `pool.Close()`.

---

## 11. Security Implementation

### 11.1 HTTP security headers middleware

Applied globally via chi middleware before all route handlers:

```
Content-Security-Policy: default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
Referrer-Policy: strict-origin-when-cross-origin
Permissions-Policy: camera=(), microphone=(), geolocation=()
Strict-Transport-Security: max-age=31536000; includeSubDomains  (production only)
```

### 11.2 CORS middleware

```go
cors.New(cors.Options{
    AllowedOrigins:   strings.Split(os.Getenv("HTTP_ALLOWED_ORIGINS"), ","),
    AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
    AllowedHeaders:   []string{"Authorization", "Content-Type", "X-Request-Id"},
    AllowCredentials: true,  // required for the refresh token cookie
    MaxAge:           300,
})
```

### 11.3 Rate limiting

Token-bucket limiter via `golang.org/x/time/rate`. Two limits:
- Global: 100 req/s per IP (configurable via `RATE_LIMIT_REQUESTS_PER_SECOND`).
- Auth endpoints: 10 req/s per IP (hard-coded; protects against credential stuffing).

Limits are stored in an in-memory map (not Redis by default). Multi-replica setups should set `REDIS_URL` to enable a Redis-backed adapter.

### 11.4 SSRF prevention

`pkg/egress.Guard` wraps `net.DialContext`. Before establishing a connection:
1. Resolve the hostname to IP.
2. If `EGRESS_MODE=public-only`: reject RFC 1918, loopback, link-local, and multicast addresses.
3. If `EGRESS_MODE=allowlist`: only allow IPs matching the configured CIDR allowlist.
4. If `EGRESS_MODE=allow-private` (default): permit all (suitable for internal deployments).

All outbound HTTP via `pkg/httpclient` uses this guarded dialer.

---

## 12. Deployment Topology

### 12.1 Single-replica (recommended for most deployments)

```
                       ┌────────────────┐
Internet  ──HTTPS──► │   Reverse Proxy │ (nginx / Caddy / ALB)
                       │  TLS termination│
                       └───────┬────────┘
                               │HTTP
                       ┌───────▼────────┐
                       │  Data Explorer │ (single Go binary)
                       │  :8080         │
                       │  + scheduler   │
                       └───────┬────────┘
                               │
                       ┌───────▼────────┐
                       │  PostgreSQL    │
                       └────────────────┘
```

### 12.2 Multi-replica

Same topology but multiple Data Explorer instances behind a load balancer. Requires:
- `REDIS_URL` for distributed rate limiting.
- PostgreSQL advisory locks for scheduler deduplication (built in).
- Sticky sessions not required (stateless API; JWT in Authorization header).

### 12.3 Docker Compose (development/quick-start)

```yaml
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: dataexplorer
      POSTGRES_USER: dataexplorer
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
    volumes:
      - pgdata:/var/lib/postgresql/data

  backend:
    build: ./backend
    environment:
      DATABASE_URL: postgres://dataexplorer:${POSTGRES_PASSWORD}@postgres:5432/dataexplorer
      JWT_SECRET: ${JWT_SECRET}
      CONNECTION_ENCRYPTION_KEY: ${CONNECTION_ENCRYPTION_KEY}
      APP_ENV: development
      HTTP_ALLOWED_ORIGINS: http://localhost:5173
    ports:
      - "8080:8080"
    depends_on: [postgres]

  frontend:
    build: ./frontend
    ports:
      - "5173:5173"
    environment:
      VITE_API_BASE_URL: http://localhost:8080/api/v1
```

---

## 13. Risk Register

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Scheduler duplicate execution on multi-replica | High (if running > 1 replica) | Data duplication | PostgreSQL advisory locks (built in); document requirement |
| Connection secret leakage via logs | Medium | Critical | `slog` structured fields prevent secret interpolation; review middleware strips secrets from error messages |
| SQL injection via user-supplied queries | Medium | Critical | `sqlguard` keyword analysis; parameterized queries for all internal SQL |
| SSRF via connector | High (no egress control) | High | `pkg/egress.Guard`; document `EGRESS_MODE=public-only` for internet-facing deployments |
| JWT secret compromise | Low | Critical | Short-lived tokens (15 min); document rotation procedure |
| Encryption key compromise | Low | Critical | AES-GCM provides ciphertext integrity; document key rotation script |
| Memory exhaustion from large query results | Medium | High | `MaxRowsPerNode`, `MaxRows`, response size cap at every layer |
| Cron expression evaluation DoS | Low | Medium | `robfig/cron/v3` evaluation is O(1); no user-controllable loops |
| React Flow canvas performance with large DAGs | Medium | Low | Node/edge count limits (`MaxNodes=50`, `MaxEdges=200`) |
