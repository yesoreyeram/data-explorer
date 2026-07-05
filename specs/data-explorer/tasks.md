# Data Explorer — Implementation Tasks

> **Spec-Kit artifact** — This is the actionable task list for implementing Data Explorer. Tasks are derived from `spec.md` (requirements) and `plan.md` (technical approach). Each task has a clear definition of done. For governing principles, see `.specify/memory/constitution.md`.
>
> **Persistence model**: Living Spec — when requirements change, update `spec.md` first, then revise this list.

---

## Phase 0: Project Foundation

### T-001 — Go module and project scaffold

- Initialize `go.mod` with module path `github.com/yesoreyeram/data-explorer/backend`.
- Create `cmd/server/main.go` with wiring skeleton.
- Add `Makefile` with targets: `build`, `test`, `vet`, `lint`.
- **Done when**: `cd backend && go build ./...` succeeds with zero errors.

### T-002 — Database migration infrastructure

- Embed SQL migration files via `//go:embed db/migrations/*.sql`.
- Implement `internal/platform/migrator` using `golang-migrate`.
- Run migrations on startup in `cmd/server/main.go` before serving.
- **Done when**: Empty PostgreSQL database is fully migrated to latest schema on startup; `go test ./...` passes migration round-trip tests.

### T-003 — Configuration loading

- Implement `internal/config` using environment variable parsing.
- Required vars: `DATABASE_URL`, `JWT_SECRET`, `CONNECTION_ENCRYPTION_KEY`, `APP_ENV`, `HTTP_ALLOWED_ORIGINS`.
- Optional vars: `HTTP_PORT`, `LOG_LEVEL`, `RATE_LIMIT_REQUESTS_PER_SECOND`, `REDIS_URL`, `EGRESS_MODE`, `OIDC_PROVIDERS`.
- Fail fast on startup if any required var is missing.
- **Done when**: Missing required var causes startup failure with a clear error message naming the missing variable.

### T-004 — Structured logging setup

- Implement `internal/platform/logger` using `slog`.
- JSON handler in production (`APP_ENV=production`), text handler in development.
- **Done when**: `go test ./...` passes; log output is valid JSON in production mode.

### T-005 — HTTP server and middleware chain

- Set up chi router.
- Wire middleware chain: `RequestID`, `RealIP`, `Logger`, `Recoverer`, `RateLimiter`, `SecurityHeaders`, `CORS`.
- Implement `GET /healthz` and `GET /readyz`.
- **Done when**: `curl http://localhost:8080/healthz` returns `{"status":"ok"}` with status 200; `/readyz` returns 503 when DB is down and 200 when DB is up.

### T-006 — Frontend project scaffold

- Initialize React + TypeScript + Vite project in `frontend/`.
- Install dependencies: `react`, `react-router-dom`, `@tanstack/react-query`, `zustand`, `reactflow`.
- Configure Oxlint.
- Set up `src/styles/tokens.css` with all design tokens.
- **Done when**: `cd frontend && npm ci && npx tsc -b && npm run lint && npm run build` all pass with zero errors.

### T-007 — Design system component library

- Implement `src/components/ui/`: `Button`, `IconButton`, `Field`, `Input`, `Select`, `Textarea`, `Badge`, `Card`/`CardHeader`/`CardBody`, `StatTile`.
- All components use CSS custom properties only; no hard-coded hex values.
- **Done when**: All components render in Storybook or a component demo page; `npx tsc -b` passes; Oxlint passes.

### T-008 — Theme system

- Implement `ThemeSwitcher` component; light/dark/system modes.
- Persist preference to `localStorage`.
- Apply `data-theme` attribute to `<html>` on load.
- **Done when**: Switching themes updates all design-token-based colors immediately; preference survives page reload.

---

## Phase 1: Authentication and Session Management

### T-009 — Crypto primitives

- Implement `internal/platform/crypto`: Argon2id hash/verify, AES-256-GCM encrypt/decrypt, random token generation.
- Table-driven tests for all functions including edge cases.
- **Done when**: `go test ./internal/platform/crypto/... -race` passes with ≥ 10 test cases covering correctness, wrong key, tampered ciphertext, and empty inputs.

### T-010 — User registration

- Implement `POST /auth/register`.
- Validate: email format, password ≥ 8 chars, display name non-empty.
- Hash password with Argon2id; insert user; assign `viewer` role; write audit log.
- **Done when**: Registration succeeds with valid input (201); duplicate email returns 409; weak password returns 400; new user has `viewer` role.

### T-011 — User login

- Implement `POST /auth/login`.
- Constant-time password verification; uniform error for wrong email and wrong password.
- Issue JWT (15 min TTL, permissions embedded); generate refresh token; set `httpOnly SameSite=Strict` cookie.
- Write audit log for success and failure.
- **Done when**: Login returns JWT + cookie; wrong credentials return 401 with generic message; timing of wrong-email vs wrong-password responses differs by < 10 ms (measured in tests).

### T-012 — Refresh token rotation

- Implement `POST /auth/refresh`.
- Atomic rotation: delete old token, insert new, issue new JWT.
- Replay: second use of the same refresh token returns 401.
- **Done when**: Refresh works once; second use of same token returns 401; new JWT has fresh 15-min expiry.

### T-013 — Logout

- Implement `POST /auth/logout`.
- Revoke refresh token from DB; clear cookie.
- Write audit log.
- **Done when**: After logout, `POST /auth/refresh` with the revoked cookie returns 401.

### T-014 — Password change

- Implement `POST /auth/change-password`.
- Require current password; hash new password; update user row.
- Write audit log.
- **Done when**: Valid request succeeds; wrong current password returns 400; new password is accepted at next login.

### T-015 — JWT authentication middleware

- Implement `internal/rbac` with `Principal` struct and `RequirePermission` middleware.
- Extract and validate JWT on every authenticated request; inject `Principal` into context.
- **Done when**: Missing or invalid JWT returns 401; expired JWT returns 401; valid JWT with correct permission passes through.

### T-016 — OIDC login

- Implement `GET /auth/oidc/{provider}` and `GET /auth/oidc/{provider}/callback`.
- PKCE (S256); CSRF state cookie; `email_verified` required.
- Upsert user on first login; issue JWT + refresh cookie.
- **Done when**: Full OIDC flow works with a test provider; `email_verified=false` is rejected; second login with same email matches existing user.

### T-017 — Login / registration UI pages

- Implement `LoginPage.tsx` and `RegisterPage.tsx` using UI component library.
- Zustand `auth` store: accessToken (in-memory), user object, permissions array.
- Auto-refresh on token expiry; redirect to login on 401.
- **Done when**: Login stores token in React state (not `localStorage`); page refresh requires re-login or succeeds via refresh cookie; 401 from any API call triggers silent refresh attempt.

---

## Phase 2: RBAC and User Management

### T-018 — RBAC permission seeding

- DB migration: insert `admin`, `editor`, `viewer` roles and all 11 permission codes.
- Seed `role_permissions` join table.
- **Done when**: After migration, `SELECT * FROM roles` returns 3 rows; `admin` role has 11 permissions.

### T-019 — User management API

- Implement `GET /users`, `PATCH /users/{id}`, `POST /users/{id}/roles`, `DELETE /users/{id}/roles/{roleId}`.
- Gate all routes with appropriate permissions.
- Write audit log for every mutating action.
- **Done when**: Admin can list, update, assign, and remove roles; viewer calling these endpoints returns 403; role assignment reflected in next JWT issued for that user.

### T-020 — Users page (UI)

- Implement `UsersPage.tsx`: user list with roles, role assignment, status toggle.
- Use `<PermissionGate permission="users:write">` to hide action buttons for non-admins.
- **Done when**: Admin sees action buttons; viewer does not; role changes reflected immediately.

---

## Phase 3: Connection Management

### T-021 — Connection CRUD API

- Implement `GET /connections`, `POST /connections`, `GET /connections/{id}`, `PUT /connections/{id}`, `DELETE /connections/{id}`.
- Encrypt credentials on write; never return secret in response.
- Write audit log for every mutating action.
- **Done when**: Create returns connection without secret; decrypt cycle works internally; viewer can list/get but not create/update/delete (403).

### T-022 — Connection test endpoint

- Implement `POST /connections/{id}/test`.
- Decrypt secret; call connector; classify error; persist last-check fields.
- **Done when**: Test on a valid connection updates `last_tested_at` and returns `checkDurationMs`; test on invalid config returns a structured `HealthError` with a stable `errorCode`.

### T-023 — Connector implementations

Implement each connector package. For each:
- `Validate(config)` — structural config validation (no network calls).
- `Query(ctx, config, query)` → `*dataframe.Frame` — execute and return data.
- Error classification via `connections.Classify`.
- Rate limiting respect.
- Add `connector_query_duration_seconds` Prometheus observation.

| Connector | Key Notes |
|---|---|
| `postgres` | Use `pgx`; enforce `sqlguard`; IAM auth via RDS helper |
| `mysql` | Standard driver; enforce `sqlguard` |
| `rest` | All auth methods; size cap; redirect cap; retry; SSRF guard |
| `graphql` | POST to `/graphql`; same auth matrix as REST |
| `aws` | Athena, DynamoDB, CloudWatch, S3 via `aws-sdk-go-v2`; ambient credential chain |
| `gcp` | BigQuery, GCS via `cloud.google.com/go`; ambient credential chain |
| `azure` | Log Analytics, Blob Storage via `azure-sdk-for-go`; managed identity support |

- **Done when**: Each connector has config-validation tests and at least one mock-network query test; no real network calls in CI tests; `sqlguard` tests for SQL-accepting connectors.

### T-024 — Integration catalog

- Implement static catalog as embedded JSON in `internal/catalog`.
- ≥ 15 entries covering common REST/GraphQL APIs (GitHub, Stripe, Slack, Salesforce, etc.).
- Implement `GET /catalog` and `GET /catalog/{id}` with name/description search.
- **Done when**: Catalog works offline; `GET /catalog?q=github` returns only matching entries; no credentials in any entry.

### T-025 — Connections UI

- Implement `ConnectionsPage.tsx` (list + create modal) and `ConnectionDetailPage.tsx` (edit + health panel).
- Health panel shows: status badge, error code, error message, remediation, last-checked timestamp.
- **Done when**: Create, edit, delete, and test all work; secret fields are write-only (not pre-populated on edit); health panel updates after test.

---

## Phase 4: Ad-Hoc Exploration

### T-026 — DataFrame package

- Implement `pkg/dataframe`: `Frame` struct, `Schema` struct, serialization to/from JSON wire format.
- Helper functions: `Truncate(n int)`, `ToCSV()`, column type inference.
- **Done when**: Round-trip JSON serialization preserves all column types; `go test ./pkg/dataframe/...` passes.

### T-027 — Explore query API

- Implement `POST /explore/query`.
- Support saved connectionId and inline credentials (inline requires `connections:test`).
- Inline credentials: decrypt, use, discard — never persisted.
- Cap at `MaxRows`.
- Write audit log.
- **Done when**: Query against saved connection works; inline credential query works; inline connection not persisted; result truncated at `MaxRows`; wrong permission returns 403.

### T-028 — Explore UI

- Implement `ExploreQueryPage.tsx`: connection selector, query input, result table, export buttons, chart selector.
- Persist recent queries in `localStorage`.
- **Done when**: Query runs and displays results; CSV/JSON export downloads correct file; recent query history populates on revisit; chart renders for numeric columns.

### T-029 — DataFrameView component

- Implement `DataFrameView.tsx`: sortable/filterable column headers, pagination, export buttons.
- Implement `DataTable.tsx`: virtualized rendering for large row counts.
- Implement basic charting (recharts or similar): line, bar, scatter.
- **Done when**: Table renders 10,000 rows without UI freeze; export produces correct file; chart axes auto-scaled.

---

## Phase 5: Workflow Builder and Engine

### T-030 — Workflow CRUD API

- Implement `GET /workflows`, `POST /workflows`, `GET /workflows/{id}`, `PUT /workflows/{id}`, `DELETE /workflows/{id}`.
- Validate definition JSONB on save (DAG validation: cycle check, disconnected nodes, MaxNodes, MaxEdges).
- Write audit log for every mutating action.
- **Done when**: Creating a workflow with a cycle returns 400 with the cycle path; valid workflow saves and retrieves correctly.

### T-031 — Workflow engine

- Implement `internal/workflow/engine.go` (Kahn's topological sort + node dispatch loop).
- Apply `MaxRowsPerNode` cap after each node.
- Apply `MaxExecutionDuration` timeout.
- Stop at first failing node; persist partial results.
- **Done when**: 3-node DAG executes in correct order; row cap enforced; timeout kills long-running nodes; execution record created with per-node timings.

### T-032 — Workflow node implementations

Implement each node type in `internal/workflow/nodes/`:

| Node | Logic |
|---|---|
| `source` | Call connector `Query`; return frame |
| `filter` | Apply JSONata predicate row-by-row; return matching rows |
| `transform` | Apply JSONata expression to each row or the whole frame |
| `join` | In-memory hash join; support inner, left, right |
| `aggregate` | Group-by with sum, count, avg, min, max |
| `output` | Write frame back to a connection (optional) |

- **Done when**: Each node has a unit test with a known input and expected output; `MaxRowsPerNode` cap tested per node type.

### T-033 — Workflow execute and execution history API

- Implement `POST /workflows/{id}/execute` (returns execution ID immediately; runs synchronously for v1).
- Implement `GET /workflows/{id}/executions` (list) and `GET /workflows/{id}/executions/{execId}` (detail).
- **Done when**: Execution runs and returns status; execution record contains `node_results`; viewer can read but not execute.

### T-034 — Workflow scheduling API

- Implement `PUT /workflows/{id}/schedule`.
- Validate cron expression; compute `schedule_next_run`.
- Implement in-process scheduler poll loop.
- **Done when**: Scheduling a workflow with a valid cron sets `schedule_next_run` correctly; scheduler picks it up and creates an execution with `triggered_by="scheduler"`; invalid cron returns 400.

### T-035 — Workflow builder UI

- Implement `WorkflowBuilderPage.tsx` with React Flow canvas.
- Register all 6 node type components.
- Sidebar: execution history panel with per-node detail.
- Auto-save on node/edge change (debounced, 1 s).
- **Done when**: All 6 node types renderable; adding, connecting, and deleting nodes persists correctly; execution triggered from UI shows real-time status; history panel updates on completion.

---

## Phase 6: Audit Log

### T-036 — Audit log writer and query service

- Implement `internal/audit`: `Write(ctx, entry)` and `Query(ctx, filter)`.
- Write is called in every handler that performs a mutating action or sensitive read.
- **Done when**: Every action in the audited-actions list creates a log entry; entries include all required fields; no UPDATE or DELETE endpoint exists.

### T-037 — Audit log API

- Implement `GET /audit-logs` with filters: `actor`, `action`, `resource_type`, `resource_id`, `outcome`, `from`, `to`.
- Gate with `audit:read`.
- **Done when**: Each filter parameter works independently and in combination; viewer returns 403; auditor (admin) sees all entries.

### T-038 — Audit log UI

- Implement `AuditLogPage.tsx`: filterable table with all audit fields.
- Date range picker; outcome badge (success/failure).
- **Done when**: All filter controls work; entries render with correct actor, action, resource, and outcome display.

---

## Phase 7: Observability and Hardening

### T-039 — Prometheus metrics

- Implement `internal/observability`: register all 5 required metrics.
- Instrument HTTP middleware (request counter + histogram).
- Instrument each connector (query duration histogram).
- Instrument workflow engine (execution counter + duration histogram).
- **Done when**: `GET /metrics` returns all 5 metrics; running a query increments `connector_query_duration_seconds`.

### T-040 — Per-user quota

- Implement `internal/quota`: in-memory (and optionally Redis-backed) quota store.
- Configurable per-role hourly quotas for explore runs and workflow runs.
- Return 429 when quota exceeded.
- **Done when**: Exceeding the configured quota returns 429 with a `Retry-After` header; quota resets after one hour.

### T-041 — pkg/httpclient

- Implement `pkg/httpclient` with pluggable auth (none, API key, basic, Bearer, OAuth 2.0 client credentials).
- Size cap (25 MB), redirect cap (5), retry with full-jitter backoff on 429/502/503/504.
- All outbound HTTP in connectors uses this client.
- **Done when**: Retry logic tested with a mock server that returns 429 then 200; size cap enforced; no connector bypasses this client.

### T-042 — pkg/egress (SSRF guard)

- Implement `pkg/egress.Guard` wrapping `net.DialContext`.
- Modes: `allow-private` (default), `public-only` (block RFC 1918 + loopback), `allowlist`.
- **Done when**: In `public-only` mode, dialing `192.168.1.1` returns an error; dialing a public IP succeeds; `go test ./pkg/egress/...` covers all modes.

### T-043 — Global search

- Implement `GET /search?q=` that searches connections (by name/description) and workflows (by name/description).
- Results respect RBAC (only return resources the user can `read`).
- **Done when**: Search returns relevant results; user without `connections:read` does not see connections in results.

### T-044 — Guardrails status endpoint

- Implement `GET /guardrails` returning current configured limits: `maxRows`, `maxNodes`, `maxEdges`, `maxExecutionDurationSecs`, `maxResponseSizeBytes`.
- **Done when**: Response contains all guardrail values; any authenticated user can call it.

---

## Phase 8: Frontend Polish and Accessibility

### T-045 — Navigation and routing

- Implement `Sidebar.tsx`: collapsible with icon-only mode; keyboard navigable.
- Implement `ProtectedRoute.tsx`: redirect to login if unauthenticated.
- Implement breadcrumb navigation in topbar.
- **Done when**: Sidebar collapses and expands; state survives page reload; unauthenticated user is redirected to login; breadcrumbs reflect current page path.

### T-046 — Dashboard page

- Implement `DashboardPage.tsx`: stat tiles (connection count, workflow count, recent executions), quick-access links.
- **Done when**: Dashboard loads without errors; stat tiles show correct counts from API.

### T-047 — Loading, error, and empty states

- Loading: skeleton loaders on list pages; spinner on action buttons.
- Error: inline error message with error code badge and remediation hint.
- Empty: illustration + call-to-action for all empty list views.
- **Done when**: All list pages show skeleton on initial load; API error shows error code + remediation; empty connection/workflow lists show CTA.

### T-048 — Accessibility audit

- Run Axe or similar against all main pages.
- Fix any critical violations: missing `aria-label`, missing focus ring, color-only status indicators.
- **Done when**: Axe reports zero critical violations on Login, Dashboard, Connections, Explore, Workflow Builder, Audit Log pages.

---

## Phase 9: Testing and Quality Assurance

### T-049 — Backend integration tests

- Integration tests for auth flows (registration, login, refresh, logout) against a real DB.
- Integration tests for connection CRUD and test endpoint.
- Integration tests for workflow CRUD and execute.
- Integration tests for audit log filtering.
- All tests use `DATABASE_URL` env var; `t.Skip` if not set.
- **Done when**: All integration tests pass with a local PostgreSQL instance; `go test ./... -race` passes.

### T-050 — Frontend component tests

- React Testing Library tests for: `LoginPage`, `ConnectionsPage`, `WorkflowBuilderPage`, `DataFrameView`, `PermissionGate`, `ThemeSwitcher`.
- All API calls mocked at module boundary.
- **Done when**: `npm test` passes with zero failures; coverage on tested components ≥ 80%.

### T-051 — End-to-end tests (Playwright)

- E2E tests in `frontend/e2e/`:
  - Login and logout.
  - Creating a connection and testing it.
  - Running an ad-hoc query; exporting CSV.
  - Building a 3-node workflow and executing it.
  - Viewing execution history.
  - Viewing and filtering the audit log.
- **Done when**: All 6 E2E tests pass against a running dev stack; tests are runnable in CI with `npm run test:e2e`.

### T-052 — Security review

- Verify no secrets appear in any log line (grep logs for known test credentials after running integration tests).
- Verify every route in `router.go` has a `RequirePermission` call (automated check or manual audit).
- Verify CSP header is set on all responses.
- Verify `secret_encrypted` is never returned in any API response (automated assertion in connection CRUD tests).
- **Done when**: All 4 checks pass; any failure is a blocking bug.

---

## Phase 10: Documentation and Deployment

### T-053 — README update

- Update `README.md` with quick-start instructions, Docker Compose, environment variable reference.
- Link to `docs/SPEC.md`, `docs/ARCHITECTURE.md`, `docs/SECURITY.md`.
- **Done when**: A new developer can follow the README and have a running instance in < 10 minutes.

### T-054 — Docker Compose deployment

- Create `deploy/docker-compose.yml` and `deploy/.env.example`.
- Services: `postgres`, `backend`, `frontend`.
- **Done when**: `docker compose up --build` produces a working stack at `http://localhost:5173`.

### T-055 — CI/CD pipeline

- GitHub Actions workflow (`.github/workflows/ci.yml`).
- Backend job: `go build`, `go vet`, `go test -race`.
- Frontend job: `npm ci`, `npx tsc -b`, `npm run lint`, `npm run build`.
- **Done when**: All CI jobs pass on every push to main and on every PR; any failure blocks merge.

### T-056 — Screenshots for documentation

- Capture screenshots of all main pages using Playwright.
- Store in `docs/screenshots/` with `NN-kebab-name.png` naming.
- **Done when**: Screenshots exist for: login, dashboard, connections list, connection detail, explore query, workflow builder, workflow execution, audit log, users page.
