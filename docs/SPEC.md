# Data Explorer — Project Specification

> **GitHub Spec Kit** — This document is the complete, self-contained specification for Data Explorer. It is intended to serve as the single source of truth for rebuilding the project from scratch. It covers all functional requirements, non-functional requirements, security model, observability, auditability, code quality standards, design principles, and UI guidelines.

---

## Table of Contents

1. [Project Overview](#1-project-overview)
2. [Personas and User Stories](#2-personas-and-user-stories)
3. [Functional Requirements](#3-functional-requirements)
4. [Non-Functional Requirements](#4-non-functional-requirements)
5. [Security Requirements](#5-security-requirements)
6. [Observability and Auditability Requirements](#6-observability-and-auditability-requirements)
7. [Data Model](#7-data-model)
8. [API Contract](#8-api-contract)
9. [Architecture Principles](#9-architecture-principles)
10. [Code Quality Standards](#10-code-quality-standards)
11. [UI / UX Guidelines](#11-ui--ux-guidelines)
12. [Design System](#12-design-system)
13. [Testing Requirements](#13-testing-requirements)
14. [Deployment and Operations](#14-deployment-and-operations)
15. [Known Limitations and Future Work](#15-known-limitations-and-future-work)

---

## 1. Project Overview

**Data Explorer** is a self-hosted, enterprise-grade data exploration and pipeline platform. It lets users connect to databases and APIs, explore data ad-hoc, and build visual data pipelines — with enterprise-grade RBAC, audit logging, and observability built in from day one.

### Core value propositions

| Value | Description |
|---|---|
| **Connect** | Centrally managed, encrypted-at-rest connections to PostgreSQL, MySQL, REST APIs, GraphQL APIs, AWS, GCP, and Azure |
| **Explore** | Ad-hoc queries against saved or temporary (never-persisted) connections; no pipeline required |
| **Build** | Drag-and-drop visual pipeline builder: source → filter → transform → join → aggregate → output |
| **Schedule** | Cron-based workflow scheduling with the same execution path and guardrails as manual runs |
| **Govern** | Fine-grained RBAC, append-only audit trail, row/size/rate guardrails at every layer |
| **Observe** | Prometheus metrics, structured logging, health probes, classified connection errors |

### Technology stack

| Layer | Technology |
|---|---|
| Backend | Go 1.25+, chi router, pgx v5, JWT (HS256), Prometheus |
| Frontend | React 19, TypeScript, Vite, React Flow, TanStack Query, Zustand |
| Database | PostgreSQL (system of record) |
| Transform | JSONata (`blues/jsonata-go`) |
| Tabular data | `pkg/dataframe` (standalone pandas-style library) |
| HTTP client | `pkg/httpclient` (standalone; pluggable auth + pagination) |
| SSRF guard | `pkg/egress` (standalone; DNS-resolve-then-dial) |
| Cloud SDKs | `aws-sdk-go-v2`, `cloud.google.com/go/{bigquery,storage}`, `azure-sdk-for-go` |
| Scheduling | `robfig/cron/v3` + in-process poll loop |

---

## 2. Personas and User Stories

### Personas

| Persona | Description |
|---|---|
| **Admin** | Full control: manages users, roles, connections, workflows, and views the audit log |
| **Editor** | Creates and manages connections and workflows; can run ad-hoc queries |
| **Viewer** | Read-only: can browse connections and view workflow results, but cannot create or modify |
| **Analyst** | Heavy user of ad-hoc exploration and workflow builder; may be editor or viewer |
| **SRE / Operator** | Monitors system health, reviews metrics and guardrail status |
| **Auditor / Compliance Officer** | Reviews the audit log for compliance and investigation |

### Key user stories

- As an **admin**, I can create and manage users and assign roles so that access is appropriately scoped.
- As an **editor**, I can create a connection to a PostgreSQL database and run a query so that I can explore its data.
- As an **analyst**, I can build a visual pipeline that joins data from a REST API and a PostgreSQL table so that I can produce a unified report.
- As an **editor**, I can schedule a workflow to run nightly so that the report is always up to date.
- As a **viewer**, I can see workflow execution results without being able to modify them.
- As an **auditor**, I can filter the audit log by actor and date range so that I can investigate an incident.
- As an **SRE**, I can view Prometheus metrics and health probes so that I can monitor the service and set up alerts.
- As a **user**, I can log in via OIDC (SSO) so that I don't need to manage a separate password.

---

## 3. Functional Requirements

### FR-01: Authentication and Session Management

- **FR-01.1** Users register with email, password, and display name. New accounts receive the `viewer` role.
- **FR-01.2** Login returns a short-lived JWT access token (15 min default) and an `httpOnly` `SameSite=Strict` refresh token cookie.
- **FR-01.3** The refresh token rotates on every use (old revoked, new issued atomically).
- **FR-01.4** Login responses are timing-uniform: "user not found" and "wrong password" return the same generic error.
- **FR-01.5** Users can change their password (requires current password verification).
- **FR-01.6** Users can log out (revokes the refresh token).
- **FR-01.7** OIDC login supports Authorization Code + PKCE flow; `email_verified` claim is required.
- **FR-01.8** First OIDC login provisions a `viewer` user; subsequent logins match by email.

### FR-02: Role-Based Access Control (RBAC)

- **FR-02.1** Permissions are fine-grained strings: `users:read`, `users:write`, `roles:read`, `roles:write`, `connections:read`, `connections:write`, `connections:test`, `workflows:read`, `workflows:write`, `workflows:execute`, `audit:read`.
- **FR-02.2** Three system roles ship by default: `admin` (all permissions), `editor` (connections + workflows), `viewer` (read-only).
- **FR-02.3** Admins can assign and remove roles from users.
- **FR-02.4** Every API route is gated by exactly one permission code. No "if admin, skip" escape hatch.
- **FR-02.5** Permission checks are server-side; the frontend `<PermissionGate>` is a UX nicety only.
- **FR-02.6** Permissions are embedded in the JWT and resolved in-memory; no per-request database lookup.

### FR-03: Connection Management

- **FR-03.1** Connections are stored with non-secret config (JSONB) and AES-256-GCM encrypted credentials.
- **FR-03.2** Seven connection types: `postgres`, `mysql`, `rest`, `graphql`, `aws`, `gcp`, `azure`.
- **FR-03.3** Credentials are never returned in API responses, logs, or error messages.
- **FR-03.4** Connection test returns `errorCode` / `errorRemediation` / `checkDurationMs`.
- **FR-03.5** Each connection type supports its full authentication matrix (see §5 for the auth schemes).
- **FR-03.6** Per-connection rate limiting protects upstream systems.
- **FR-03.7** Admins and editors can create, edit, and delete connections; viewers can list and view them.

### FR-04: Integration Catalog

- **FR-04.1** A static catalog of ~20 well-known integrations prefills a new `rest`/`graphql` connection's type, base URL, and auth config.
- **FR-04.2** Catalog entries never supply credentials; the user always provides their own.
- **FR-04.3** Catalog is searchable by name/description.
- **FR-04.4** No external registry is consulted at runtime; the catalog is fully offline.

### FR-05: Connection Health Monitoring

- **FR-05.1** Every connection test classifies the error into a stable `HealthError` code: `timeout`, `network_unreachable`, `auth_failed`, `permission_denied`, `not_found`, `rate_limited`, `invalid_config`, `unknown`.
- **FR-05.2** The health panel shows: status badge, error code, plain-language message, concrete remediation step.
- **FR-05.3** The health panel shows a recent-checks history (from the audit log scoped to this connection).
- **FR-05.4** Last check timestamp, duration, and structured error fields are stored on the connection row.

### FR-06: Ad-Hoc Data Exploration

- **FR-06.1** Users can query a saved connection without building a workflow.
- **FR-06.2** Users can query a temporary (never-persisted) connection: credentials go into the request body and are discarded after use.
- **FR-06.3** Both modes return a `DataFrame` wire format: schema, rows, metadata.
- **FR-06.4** Results can be exported as CSV or JSON.
- **FR-06.5** Recent queries against saved connections are remembered client-side (localStorage).
- **FR-06.6** Results can be charted (line, bar, scatter) and saved to the dashboard.
- **FR-06.7** Ad-hoc queries with inline credentials additionally require `connections:test`.

### FR-07: Visual Workflow Builder

- **FR-07.1** Workflows are DAGs with six node types: `source`, `filter`, `transform`, `join`, `aggregate`, `output`.
- **FR-07.2** The canvas is built on React Flow; node positions are stored in the definition JSONB.
- **FR-07.3** Validation rejects cycles, disconnected graphs, and DAGs exceeding `MaxNodes`/`MaxEdges`.
- **FR-07.4** Execution is topologically ordered (Kahn's algorithm).
- **FR-07.5** After each node, output is capped at `MaxRowsPerNode` (100,000 rows).
- **FR-07.6** Execution stops at the first failing node; partial results are reported.
- **FR-07.7** Every execution (success or failure) is persisted with per-node timings, row counts, and errors.
- **FR-07.8** The builder shows an execution history panel.

### FR-08: Workflow Scheduling

- **FR-08.1** Workflows can be scheduled with a standard 5-field cron expression.
- **FR-08.2** `schedule_next_run` is pre-computed at schedule-save time; the scheduler uses a cheap indexed query.
- **FR-08.3** Scheduled executions use the same engine path as manual runs.
- **FR-08.4** `triggered_by = "scheduler"` in the execution record; no synthetic user required.
- **FR-08.5** A UI preset list (hourly, daily, weekly, etc.) assists cron authoring.

### FR-09: Query Result Export

- **FR-09.1** Every `DataFrame` result can be exported as CSV (RFC 4180) or JSON (array of objects).
- **FR-09.2** Export is client-side (no server round trip for the export itself).
- **FR-09.3** Truncated results include a warning in the export.

### FR-10: Audit Logging

- **FR-10.1** Every mutating action and sensitive read records an audit log entry.
- **FR-10.2** Audit entries include: actor_id, actor_email, action, resource_type, resource_id, outcome, ip_address, user_agent, request_id, metadata, created_at.
- **FR-10.3** The audit log is append-only; no update or delete endpoint exists.
- **FR-10.4** The audit log is queryable with filters: actor, action, resource, outcome, date range.
- **FR-10.5** The `request_id` field ties an audit entry to the structured log line for the same request.

### FR-11: Observability and Guardrails

- **FR-11.1** Structured JSON logs with `request_id`, `actor`, `route`, `status`, `duration_ms` per request.
- **FR-11.2** Prometheus metrics: HTTP requests (counter + histogram), connector query latency (histogram), workflow execution outcomes and duration.
- **FR-11.3** `/healthz` liveness probe (no DB dependency) and `/readyz` readiness probe (pings DB).
- **FR-11.4** Row limit per connector call (configurable, default 10,000).
- **FR-11.5** Response size cap for HTTP connectors (25 MB default).
- **FR-11.6** Redirect cap for HTTP connectors (5 redirects).
- **FR-11.7** Bounded retry with full jitter on 429/502/503/504.
- **FR-11.8** `MaxExecutionDuration` (2 min) on every workflow run.
- **FR-11.9** Per-IP rate limiting on all routes; stricter limits on auth endpoints.
- **FR-11.10** Per-user hourly quota for explore runs and workflow runs (configurable per role).

### FR-12: User Interface

- **FR-12.1** Near-monochrome design system; success/warning/danger/info are the only hues.
- **FR-12.2** Light / dark / system theme switcher; preference persisted to localStorage.
- **FR-12.3** Collapsible sidebar navigation.
- **FR-12.4** `<PermissionGate>` hides UI for actions the user cannot perform.
- **FR-12.5** All interactive elements have accessible labels and keyboard support.

---

## 4. Non-Functional Requirements

### Performance

| Requirement | Target |
|---|---|
| API response time (p95, non-query endpoints) | < 200 ms |
| Query execution (small results, < 1,000 rows) | < 2 s |
| Workflow execution (simple 3-node DAG) | < 5 s |
| UI initial load (Lighthouse Performance) | > 80 |
| UI time-to-interactive | < 3 s on a typical corporate laptop |

### Scalability

| Requirement | Note |
|---|---|
| Single-replica baseline | The application runs correctly as a single instance without Redis or external coordination |
| Horizontal scaling | Stateless API; multiple replicas require Redis for shared rate limiting and a distributed lock for the scheduler |
| Database | PostgreSQL 14+; connection pool size configurable |

### Reliability

| Requirement | Note |
|---|---|
| Graceful shutdown | In-flight requests drain; scheduler stops; no request truncation on SIGTERM |
| Automatic schema migration | Schema is always up-to-date on boot; no manual migration step |
| Health probes | `/healthz` and `/readyz` for orchestrator liveness/readiness checks |
| Panic recovery | A panicking handler returns 500; the server process continues |

### Maintainability

- Build passes: `go build ./...`, `npx tsc -b`, `npm run lint`, `npm run build`.
- Zero `go vet` warnings required.
- Zero TypeScript compilation errors required.
- All tests pass: `go test ./... -race` and frontend test suite.

---

## 5. Security Requirements

### Authentication

- Passwords hashed with **Argon2id** (memory-hard; parameters must not be weakened).
- Access tokens: **HS256 JWT**, 15-minute TTL, signed with `JWT_SECRET` (≥ 32 bytes).
- Refresh tokens: **opaque high-entropy random values**; only SHA-256 hash stored.
- Refresh tokens in `httpOnly`, `SameSite=Strict` cookie scoped to `/api/v1/auth`.
- Refresh tokens rotate on every use.
- OIDC: Authorization Code + PKCE; `email_verified` required; CSRF `state` + PKCE verifier in `SameSite=Lax` cookies.

### Authorization

- Every mutating and sensitive-read route gated by exactly one `rbac.RequirePermission` call in `router.go`.
- No "if admin, skip" escape hatch anywhere in the handler layer.
- Authorization is always enforced server-side; frontend gates are UX only.

### Secrets at rest

- Connection credentials encrypted with **AES-256-GCM** before database write.
- Fresh random nonce per encryption.
- Encryption key: 32-byte env var (`CONNECTION_ENCRYPTION_KEY`); never committed; required in production.
- Secrets decrypted in-memory only, immediately before connector use; never returned, logged, or persisted in plaintext.

### Input validation

- All request bodies decoded with `httpx.DecodeJSON`: `DisallowUnknownFields`, 1 MB cap.
- All user-supplied SQL validated by `sqlguard.EnsureReadOnlySQL` (keyword guard, read-only enforcement).
- All outbound HTTP uses `pkg/httpclient` (size cap, redirect cap, retry).
- All outbound dials use `pkg/egress.Guard` (SSRF prevention: resolve-then-dial).

### HTTP security headers

Every response sets: `Content-Security-Policy`, `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: strict-origin-when-cross-origin`, `Permissions-Policy`, and `Strict-Transport-Security` (production only).

### CORS

Explicit allow-list from `HTTP_ALLOWED_ORIGINS`. Wildcard `*` is never used.

### Rate limiting

Per-IP token-bucket limiter on all routes; stricter limiter on `/auth/login`, `/auth/register`, `/auth/refresh`.

---

## 6. Observability and Auditability Requirements

### Structured logging

- All log output is structured JSON in production (text in development).
- Every request log line includes: `request_id`, `method`, `path`, `status`, `duration_ms`, `actor`.
- No secrets, credentials, or decrypted values appear in any log line.
- Log level configurable via `LOG_LEVEL` env var.

### Prometheus metrics

| Metric | Type | Labels |
|---|---|---|
| `http_requests_total` | Counter | method, route, status |
| `http_request_duration_seconds` | Histogram | method, route |
| `connector_query_duration_seconds` | Histogram | connector_type |
| `workflow_executions_total` | Counter | status |
| `workflow_execution_duration_seconds` | Histogram | — |

- `/metrics` endpoint serves Prometheus text format.
- Route labels are normalized (pattern, not raw path) to control cardinality.

### Health probes

| Endpoint | Check | Use |
|---|---|---|
| `GET /healthz` | Returns `200 OK` always | Liveness (no DB) |
| `GET /readyz` | Pings PostgreSQL | Readiness |

### Audit trail

- Every mutating action records an audit entry (actor, action, resource, outcome, IP, user agent, request ID, timestamp, metadata).
- The audit log is append-only; no update or delete.
- Queryable via `/api/v1/audit-logs` with filters.
- Audit entries must be written for: login, register, logout, password change, OIDC login, connection CRUD and test, workflow CRUD and execute, schedule changes, user management actions.

### Request correlation

- `X-Request-Id` is generated or propagated on every request.
- The same ID appears in: the access log line, every child log statement for that request, and the audit log entry.

---

## 7. Data Model

### Core tables

```sql
users (
    id UUID PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    display_name TEXT NOT NULL,
    password_hash TEXT,           -- NULL for OIDC-only users
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
)

roles (
    id UUID PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    description TEXT,
    is_system BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
)

permissions (
    id UUID PRIMARY KEY,
    code TEXT UNIQUE NOT NULL,
    description TEXT
)

user_roles (
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    role_id UUID REFERENCES roles(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, role_id)
)

role_permissions (
    role_id UUID REFERENCES roles(id) ON DELETE CASCADE,
    permission_id UUID REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
)

refresh_tokens (
    id UUID PRIMARY KEY,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT UNIQUE NOT NULL,   -- SHA-256 of the opaque token
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
)

connections (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    description TEXT,
    config JSONB NOT NULL DEFAULT '{}',
    secret_encrypted TEXT,             -- AES-256-GCM ciphertext; NULL for ambient-credential connections
    status TEXT NOT NULL DEFAULT 'unverified',
    last_tested_at TIMESTAMPTZ,
    last_error TEXT,
    last_error_code TEXT,
    last_error_remediation TEXT,
    last_check_duration_ms INTEGER,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
)

workflows (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    definition JSONB NOT NULL,         -- { nodes: [...], edges: [...] }
    schedule_cron TEXT,
    schedule_enabled BOOLEAN NOT NULL DEFAULT false,
    schedule_next_run TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
)

workflow_executions (
    id UUID PRIMARY KEY,
    workflow_id UUID REFERENCES workflows(id) ON DELETE CASCADE,
    status TEXT NOT NULL,              -- 'running' | 'success' | 'failure'
    triggered_by TEXT NOT NULL,        -- user UUID or "scheduler"
    duration_ms INTEGER,
    node_results JSONB,                -- per-node timings, row counts, errors
    error TEXT,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
)

audit_logs (
    id UUID PRIMARY KEY,
    actor_id TEXT NOT NULL,            -- user UUID, "scheduler", or "system"
    actor_email TEXT,
    action TEXT NOT NULL,
    resource_type TEXT,
    resource_id TEXT,
    outcome TEXT NOT NULL,             -- 'success' | 'failure'
    ip_address TEXT,
    user_agent TEXT,
    request_id TEXT,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
)
```

---

## 8. API Contract

### Base

- Base path: `/api/v1/`
- Authentication: `Authorization: ****** <jwt>` (15-min) + rotating `httpOnly` refresh cookie
- Content-Type: `application/json`
- Request body limit: 1 MB
- Error shape: `{"error": {"code": "snake_case", "message": "human string", "remediation"?: "…", "detail"?: "…"}}`

### Routes (summary)

| Method | Path | Permission | Description |
|---|---|---|---|
| POST | `/auth/register` | public | Register a new user |
| POST | `/auth/login` | public | Login; returns JWT + refresh cookie |
| POST | `/auth/refresh` | public | Rotate refresh token; returns new JWT |
| POST | `/auth/logout` | authenticated | Revoke refresh token |
| POST | `/auth/change-password` | authenticated | Change password |
| GET | `/auth/oidc/{provider}` | public | Initiate OIDC login |
| GET | `/auth/oidc/{provider}/callback` | public | OIDC callback |
| GET | `/users` | `users:read` | List users |
| PATCH | `/users/{id}` | `users:write` | Update user (status, display name) |
| POST | `/users/{id}/roles` | `users:write` | Assign role |
| DELETE | `/users/{id}/roles/{roleId}` | `users:write` | Remove role |
| GET | `/roles` | `roles:read` | List roles with permissions |
| GET | `/connections` | `connections:read` | List connections |
| POST | `/connections` | `connections:write` | Create connection |
| GET | `/connections/{id}` | `connections:read` | Get connection |
| PUT | `/connections/{id}` | `connections:write` | Update connection |
| DELETE | `/connections/{id}` | `connections:write` | Delete connection |
| POST | `/connections/{id}/test` | `connections:test` | Test connection |
| GET | `/connections/{id}/query` | `connections:read` | Query connection |
| GET | `/catalog` | `connections:read` | List catalog entries |
| GET | `/catalog/{id}` | `connections:read` | Get catalog entry |
| POST | `/explore/query` | `connections:read` [+`connections:test` for inline] | Ad-hoc query |
| GET | `/workflows` | `workflows:read` | List workflows |
| POST | `/workflows` | `workflows:write` | Create workflow |
| GET | `/workflows/{id}` | `workflows:read` | Get workflow |
| PUT | `/workflows/{id}` | `workflows:write` | Update workflow |
| DELETE | `/workflows/{id}` | `workflows:write` | Delete workflow |
| POST | `/workflows/{id}/execute` | `workflows:execute` | Execute workflow |
| GET | `/workflows/{id}/executions` | `workflows:read` | List executions |
| PUT | `/workflows/{id}/schedule` | `workflows:write` | Set/clear schedule |
| GET | `/audit-logs` | `audit:read` | Query audit log |
| GET | `/search` | authenticated | Global search |
| GET | `/guardrails` | authenticated | Current guardrail status |
| GET | `/healthz` | public | Liveness probe |
| GET | `/readyz` | public | Readiness probe |
| GET | `/metrics` | public (network-gated in prod) | Prometheus metrics |

---

## 9. Architecture Principles

### Layering rule (inward-only dependency)

```
cmd/server             ← wiring only; no business logic
  internal/api         ← HTTP adapters; calls services, never repositories
    internal/*         ← domain services (auth, connections, workflow, audit, …)
      internal/domain  ← pure entity structs; no imports from internal/*
      pkg/*            ← standalone libraries; no internal/* imports
```

No layer may import from a layer above it. `go build ./...` must succeed with zero circular imports.

### Single binary

One Go binary: API server + scheduler + migrator. No separate worker process, queue, or cache required for a single-replica deployment.

### Constructor injection

All services are constructed with explicit dependencies. No `init()` functions. No package-level mutable state outside `cmd/server/main.go`.

### Handlers are thin adapters

Handlers decode requests, call services, and encode responses. No business logic, no SQL, no encryption in handlers.

### Repository pattern

Each service has a `Repository` (SQL access only, no business rules) and a `Service` (business rules, calls the repository). Handlers call services; services call repositories.

### Secrets never leave the service layer

`connections.Service` is the only code that decrypts a secret. Decrypted secrets are never returned in API responses, logged, or persisted.

### Append-only audit log

The `audit_logs` table has no update or delete endpoint. It is the evidentiary record.

### Every connector speaks `*dataframe.Frame`

The frame is the universal data contract. Connectors produce it; nodes consume and produce it; the API wire format is its JSON serialization.

---

## 10. Code Quality Standards

### Go

- `go vet ./...` — zero warnings required before every commit.
- `go test ./... -race` — all tests must pass with the race detector.
- Table-driven tests with `t.Run` subtests for every function with more than one interesting input.
- `errors.Is` / `errors.As` for error assertions; never compare error strings directly.
- Structured log fields: `slog.String("key", val)` — no `fmt.Sprintf` in log calls.
- `context.Context` as the first parameter of every function that may block or call external systems.
- No `init()` functions, no global mutable state outside the DI wiring in `cmd/server`.
- All user-supplied SQL through `sqlguard.EnsureReadOnlySQL`.
- All external HTTP through `pkg/httpclient`.
- All outbound dials through `pkg/egress.Guard`.

### TypeScript / React

- `npx tsc -b` — zero type errors required.
- `npm run lint` (Oxlint) — zero lint errors required.
- Always use `components/ui/` primitives; never raw HTML elements or ad-hoc class strings.
- Always use `var(--token-name)` for colors, spacing, and radius; never hard-coded hex or px.
- Use `<PermissionGate>` for permission-conditional UI; never duplicate RBAC logic in components.
- `useQuery` for server data; Zustand for client-side UI state.
- Mock API calls at the module boundary in tests; never make real HTTP requests.

---

## 11. UI / UX Guidelines

### Information density

Data Explorer is a dense, information-first tool. Favor compact layouts with tighter line heights over spacious marketing-site layouts. Default body size: 12.5px. Default row height: ~26px. Gutters on a 4-based scale.

### Navigation

- Left collapsible sidebar for primary navigation.
- Fixed topbar with breadcrumbs and user actions.
- Keyboard shortcut `⌘K` / `Ctrl+K` opens a command palette.

### Feedback patterns

- Loading states: skeleton loaders for list pages; spinner for buttons.
- Error states: inline error message with error code badge and remediation hint where available.
- Success states: brief toast notification; list refreshes automatically.
- Empty states: illustration + call-to-action, not just "No data."

### Accessibility

- All interactive elements have accessible names (button text or `aria-label`).
- Focus rings are visible in all themes (use `--focus-ring` token).
- Color is never the sole indicator of status; always pair with text or icon.
- Keyboard navigation for all modals (focus trap, `Escape` to close).
- Semantic HTML (`<button>` not `<div onClick>`).

### Responsiveness

- Optimized for desktop (1280px+); sidebar collapses at narrower viewports.
- Minimum supported viewport: 1024px wide.
- Not designed for mobile; responsive accommodations are secondary.

---

## 12. Design System

### Philosophy

1. **Near-monochrome by default.** Structural colors (surfaces, borders, text, accent) are grayscale. The accent is ink: near-black on light, near-white on dark.
2. **Status hues are the only color.** `success` (green), `warning` (amber), `danger` (red), `info` (blue) — desaturated; confined to 6px status dots and trend deltas, never filled backgrounds.
3. **One source of truth.** Every color, spacing, radius, shadow, and motion value is a CSS custom property in `frontend/src/styles/tokens.css`. Hard-coded values are bugs.
4. **Themes via `data-theme`.** Toggling `data-theme` on `<html>` re-themes the entire app; zero per-component branching.

### Token categories

- **Typography**: `--font-size-{xs,sm,md,lg,xl,2xl,3xl}`, `--font-weight-{regular,medium,semibold,bold}`
- **Color / Surface**: `--color-bg`, `--color-surface`, `--color-border`, `--color-text`, `--color-text-muted`, `--color-accent`
- **Status**: `--color-success`, `--color-warning`, `--color-danger`, `--color-info`
- **Spacing**: `--space-{1,2,3,4,6,8,12,16,24}`
- **Radius**: `--radius-{sm,md,lg,full}`
- **Layout**: `--sidebar-width`, `--topbar-height`
- **Focus**: `--focus-ring`

### Component library (`src/components/ui/`)

`Button`, `IconButton`, `Field`, `Input`, `Select`, `Textarea`, `Badge`, `Card`/`CardHeader`/`CardBody`, `StatTile`.

All new UI must use these components. Raw HTML elements or ad-hoc class names in feature code are non-conforming.

---

## 13. Testing Requirements

### Backend (Go)

| Scope | Requirement |
|---|---|
| Every public service method | At least one happy-path and one error-path unit test |
| New connector | Config-validation tests; no real network calls; `sqlguard` enforcement if SQL-accepting |
| New workflow node | Engine integration test; guardrail test (MaxRowsPerNode cap) |
| New `HealthError` classification path | Table-driven test case in `healtherror_test.go` |
| New API route | Permission enforcement test |
| Integration tests needing PostgreSQL | Use `DATABASE_URL` env var; `t.Skip` if not set |
| Race conditions | All tests pass with `-race` |

### Frontend (TypeScript / React)

| Scope | Requirement |
|---|---|
| UI components | React Testing Library tests for user-visible behaviour |
| Utilities and hooks | Vitest unit tests |
| API calls | Mocked at module boundary; no real HTTP requests |
| TypeScript | Zero errors from `npx tsc -b` |
| Lint | Zero errors from `npm run lint` |

### End-to-end (Playwright)

Critical paths covered in `frontend/e2e/`:
- Login and logout
- Creating a connection and testing it
- Running an ad-hoc query; exporting CSV
- Building a 3-node workflow and executing it
- Viewing execution history
- Viewing the audit log

---

## 14. Deployment and Operations

### Environment variables (required)

| Variable | Description |
|---|---|
| `DATABASE_URL` | PostgreSQL DSN |
| `JWT_SECRET` | ≥ 32-byte string for JWT signing |
| `CONNECTION_ENCRYPTION_KEY` | Base64-encoded 32-byte AES key (required in production) |
| `APP_ENV` | `development` \| `production` |
| `HTTP_ALLOWED_ORIGINS` | Comma-separated CORS allow-list |

### Environment variables (optional)

| Variable | Default | Description |
|---|---|---|
| `HTTP_PORT` | `8080` | API server port |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `RATE_LIMIT_REQUESTS_PER_SECOND` | `100` | Per-IP global rate limit |
| `OIDC_PROVIDERS` | — | JSON array of OIDC provider configs |
| `REDIS_URL` | — | Enable Redis-backed rate limiting (multi-replica) |
| `EGRESS_MODE` | `allow-private` | `allow-private`, `public-only`, `allowlist` |

### Docker Compose (quick start)

```bash
cp deploy/.env.example deploy/.env
docker compose -f deploy/docker-compose.yml --env-file deploy/.env up --build
```

Frontend: http://localhost:5173 — API: http://localhost:8080

### Production checklist

- [ ] `CONNECTION_ENCRYPTION_KEY` set (32 random bytes, base64-encoded).
- [ ] `JWT_SECRET` set (≥ 32 random bytes).
- [ ] `HTTP_ALLOWED_ORIGINS` set to actual frontend origin(s).
- [ ] TLS termination at reverse proxy (nginx, ALB, Cloudflare).
- [ ] `/metrics` behind network policy or basic auth.
- [ ] `APP_ENV=production`.
- [ ] PostgreSQL backup and point-in-time recovery configured.
- [ ] Log aggregation configured (JSON logs to stdout).
- [ ] Prometheus scraping configured; alert rules for error rate, p95 latency, scheduler failures.

---

## 15. Known Limitations and Future Work

### Scaling

- [ ] In-process scheduler causes duplicate executions when running multiple replicas. Needs distributed lock (e.g. `pg_try_advisory_lock`) or leader election.
- [ ] In-process rate limiter and quota store are not shared across replicas; Redis adapter partially addresses rate limiting but not quota.

### Workflow engine

- [ ] Execution is synchronous (max 2 min); long-running pipelines need an async execution model (queue + worker).
- [ ] No parallel node execution; fan-in/fan-out DAGs execute serially.
- [ ] No retry logic for individual failing nodes.
- [ ] JSONata is the only expression language; no SQL-over-dataframe or Python option.

### Authentication

- [ ] No MFA (TOTP / WebAuthn).
- [ ] No account lockout after repeated failed login attempts.
- [ ] No device/session management UI.
- [ ] Encryption key rotation requires a manual script.

### Connectors

- [ ] Object storage (S3/GCS/Azure Blob) capped at 50 MB; no streaming for larger objects.
- [ ] Async polling (Athena, CloudWatch) blocks the request goroutine.
- [ ] No connection pooling across requests for HTTP connectors.
- [ ] Kerberos ticket acquisition has no context timeout.

### UI

- [ ] Not designed for mobile.
- [ ] Saved charts and explore history are per-browser (localStorage), not synced.
- [ ] Workflow builder auto-save generates many API calls without debouncing.

### Observability

- [ ] No distributed tracing (OpenTelemetry spans).
- [ ] No SLO definitions or alerting rules shipped with the project.
- [ ] `/metrics` is unauthenticated; should be behind network policy in hardened deployments.
