# Data Explorer — Project Constitution

> This document establishes the governing principles, non-negotiable rules, and development guidelines for the Data Explorer project. All contributors and AI coding agents must follow these principles. Every code change, architectural decision, and new feature must be consistent with this constitution.

---

## 1. Core Mission

Data Explorer is a **self-hosted, enterprise-grade data exploration and pipeline platform**. Its purpose is to let users connect to databases and APIs, explore data ad-hoc, and build visual data pipelines — with enterprise-grade RBAC, audit logging, and observability built in from day one.

The system must never compromise on **security**, **auditability**, or **data governance** in exchange for development convenience.

---

## 2. Architecture Principles

### 2.1 Layered architecture (inward-only dependency)

```
cmd/server             ← wiring only; no business logic
  internal/api         ← HTTP adapters; calls services, never repositories
    internal/*         ← domain services (auth, connections, workflow, audit, …)
      internal/domain  ← pure entity structs; no imports from internal/*
      pkg/*            ← standalone libraries; no internal/* imports
```

- No layer may import from a layer above it.
- `go build ./...` must pass with zero circular imports.
- Violations of the dependency rule are build-blocking bugs.

### 2.2 Single binary

One Go binary serves as: API server + in-process scheduler + schema migrator. No external worker process, message queue, or cache required for a single-replica deployment.

### 2.3 Constructor injection everywhere

All services are constructed with explicit dependencies via function parameters. No `init()` functions. No package-level mutable state outside `cmd/server/main.go`.

### 2.4 Handlers are thin adapters

Handlers decode requests, call services, and encode responses. **No business logic, SQL, or encryption in handlers.** Any business logic found in a handler is a bug.

### 2.5 Repository pattern

Each domain service has:
- A `Repository` interface: SQL access only, no business rules.
- A `Service`: business rules, calls the repository.

Handlers call services; services call repositories. Services never call other services' repositories directly.

### 2.6 Secrets never leave the service layer

`connections.Service` is the only code that decrypts a secret. Decrypted secrets are **never** returned in API responses, written to logs, or persisted in plaintext anywhere.

### 2.7 Append-only audit log

The `audit_logs` table has no update or delete endpoint. It is the evidentiary record of every mutating action and sensitive read. Removing an audit entry is not permitted under any circumstances.

### 2.8 Universal data contract

Every connector produces a `*dataframe.Frame`. Nodes consume and produce frames. The API wire format is the JSON serialization of a frame. No connector or node is allowed to bypass this contract.

---

## 3. Security Mandates

These rules are **non-negotiable**. Any change that violates them must be reverted before merging.

### 3.1 Authentication

- Passwords hashed with **Argon2id** (memory-hard). Parameters must not be weakened.
- Access tokens: **HS256 JWT**, 15-minute TTL, signed with `JWT_SECRET` (≥ 32 bytes).
- Refresh tokens: opaque high-entropy random values; only the SHA-256 hash is stored.
- Refresh tokens delivered in an `httpOnly`, `SameSite=Strict` cookie scoped to `/api/v1/auth`.
- Refresh tokens rotate on every use (old token revoked, new token issued atomically).
- OIDC: Authorization Code + PKCE; `email_verified` claim required; CSRF `state` + PKCE verifier in `SameSite=Lax` cookies.

### 3.2 Authorization (RBAC)

- Every mutating and sensitive-read route is gated by exactly one `rbac.RequirePermission` call in `router.go`.
- No "if admin, skip" escape hatches in any handler.
- Authorization is always enforced server-side; frontend gates are UX-only conveniences.
- Permission codes are fine-grained strings: `users:read`, `users:write`, `roles:read`, `roles:write`, `connections:read`, `connections:write`, `connections:test`, `workflows:read`, `workflows:write`, `workflows:execute`, `audit:read`.

### 3.3 Secrets at rest

- Connection credentials encrypted with **AES-256-GCM** before any database write.
- Fresh random nonce per encryption operation.
- Encryption key: 32-byte env var (`CONNECTION_ENCRYPTION_KEY`); never committed to source control; required in production.
- Secrets decrypted in-memory only, immediately before connector use; never returned, logged, or persisted in plaintext.

### 3.4 Input validation

- All request bodies decoded with `httpx.DecodeJSON`: `DisallowUnknownFields`, 1 MB cap.
- All user-supplied SQL validated by `sqlguard.EnsureReadOnlySQL` (keyword guard, read-only enforcement).
- SQL is never constructed by string concatenation with user input. Parameterized queries only.
- All outbound HTTP uses `pkg/httpclient` (size cap, redirect cap, retry).
- All outbound dials use `pkg/egress.Guard` (SSRF prevention: resolve-then-dial).

### 3.5 HTTP security headers

Every response sets: `Content-Security-Policy`, `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: strict-origin-when-cross-origin`, `Permissions-Policy`, and `Strict-Transport-Security` (production only).

### 3.6 CORS

Explicit allow-list from `HTTP_ALLOWED_ORIGINS`. Wildcard `*` is never used in any environment.

### 3.7 Rate limiting

Per-IP token-bucket limiter on all routes; stricter limits on `/auth/login`, `/auth/register`, `/auth/refresh`.

### 3.8 Audit obligation

Every mutating action and every sensitive read must emit an audit log entry before responding. Missing audit entries are security bugs, not documentation gaps.

---

## 4. Code Quality Standards

### 4.1 Go

- `go vet ./...` — zero warnings required before every commit.
- `go test ./... -race` — all tests must pass with the race detector.
- Table-driven tests with `t.Run` subtests for every function with more than one interesting input.
- `errors.Is` / `errors.As` for error assertions; never compare error strings directly.
- Structured log fields: `slog.String("key", val)` — no `fmt.Sprintf` in log calls.
- `context.Context` as the first parameter of every function that may block or call external systems.
- No `init()` functions; no global mutable state outside the DI wiring in `cmd/server`.
- All user-supplied SQL through `sqlguard.EnsureReadOnlySQL`.
- All external HTTP through `pkg/httpclient`.
- All outbound dials through `pkg/egress.Guard`.

### 4.2 TypeScript / React

- `npx tsc -b` — zero type errors required.
- `npm run lint` (Oxlint) — zero lint errors required.
- Always use `src/components/ui/` primitives; never raw HTML elements or ad-hoc class strings.
- Always use `var(--token-name)` for colors, spacing, and radius; never hard-coded hex or `px`.
- Use `<PermissionGate>` for permission-conditional UI; never duplicate RBAC logic in components.
- `useQuery` (TanStack Query) for server data; Zustand for client-side UI state.
- Mock API calls at the module boundary in tests; never make real HTTP requests in tests.

### 4.3 Build gates (all must pass before merge)

| Gate | Command |
|---|---|
| Go build | `cd backend && go build ./...` |
| Go vet | `cd backend && go vet ./...` |
| Go tests | `cd backend && go test ./... -race` |
| TypeScript types | `cd frontend && npx tsc -b` |
| Frontend lint | `cd frontend && npm run lint` |
| Frontend build | `cd frontend && npm run build` |

---

## 5. Testing Requirements

### 5.1 Go

| Scope | Requirement |
|---|---|
| Every public service method | At least one happy-path and one error-path unit test |
| New connector | Config-validation tests; no real network calls; `sqlguard` enforcement if SQL-accepting |
| New workflow node | Engine integration test; guardrail test (`MaxRowsPerNode` cap) |
| New `HealthError` classification path | Table-driven test case in `healtherror_test.go` |
| New API route | Permission enforcement test |
| Integration tests needing PostgreSQL | Use `DATABASE_URL` env var; `t.Skip` if not set |
| Race conditions | All tests pass with `-race` |

### 5.2 TypeScript / React

| Scope | Requirement |
|---|---|
| UI components | React Testing Library tests for user-visible behaviour |
| Utilities and hooks | Vitest unit tests |
| API calls | Mocked at module boundary; no real HTTP requests |
| TypeScript | Zero errors from `npx tsc -b` |
| Lint | Zero errors from `npm run lint` |

### 5.3 End-to-end (Playwright)

Critical paths covered in `frontend/e2e/`:
- Login and logout
- Creating a connection and testing it
- Running an ad-hoc query; exporting CSV
- Building a 3-node workflow and executing it
- Viewing execution history
- Viewing the audit log

---

## 6. UI / Design System Standards

### 6.1 Design philosophy

1. **Near-monochrome by default.** Structural colors (surfaces, borders, text, accent) are grayscale. The accent is ink: near-black on light, near-white on dark.
2. **Status hues are the only color.** `success` (green), `warning` (amber), `danger` (red), `info` (blue) — desaturated; confined to 6px status dots and trend deltas. Never filled backgrounds.
3. **One source of truth.** Every color, spacing, radius, shadow, and motion value is a CSS custom property in `frontend/src/styles/tokens.css`. Hard-coded values are bugs.
4. **Themes via `data-theme`.** Toggling `data-theme` on `<html>` re-themes the entire app; zero per-component branching.

### 6.2 Component usage rules

- Use `src/components/ui/` primitives for all new UI: `Button`, `IconButton`, `Field`, `Input`, `Select`, `Textarea`, `Badge`, `Card`/`CardHeader`/`CardBody`, `StatTile`.
- Raw HTML elements or ad-hoc class names in feature code are non-conforming and will be rejected in review.
- Use `<PermissionGate permission="…">` for all permission-conditional UI.

### 6.3 Information density

Data Explorer is a dense, information-first tool. Favor compact layouts over spacious marketing-site layouts. Default body size: 12.5px. Default row height: ~26px. Gutters on a 4-based scale.

### 6.4 Accessibility

- All interactive elements have accessible names (button text or `aria-label`).
- Focus rings are visible in all themes (use `--focus-ring` token).
- Color is never the sole indicator of status; always pair with text or icon.
- Keyboard navigation for all modals (focus trap, `Escape` to close).
- Semantic HTML (`<button>` not `<div onClick>`).

### 6.5 Screenshots for UI changes

Every PR that adds or changes any visible UI must:
1. Capture screenshots of every new or changed screen using Playwright.
2. Store PNGs in `docs/screenshots/` using the `NN-kebab-name.png` naming convention.
3. Delete or overwrite stale screenshots from replaced screens.
4. Embed screenshots in the PR description.

---

## 7. Observability Standards

### 7.1 Structured logging

- All log output is structured JSON in production (text in development).
- Every request log line includes: `request_id`, `method`, `path`, `status`, `duration_ms`, `actor`.
- No secrets, credentials, or decrypted values appear in any log line — ever.
- Log level configurable via `LOG_LEVEL` env var.
- `slog.String("key", val)` always; never `fmt.Sprintf` in log calls.

### 7.2 Prometheus metrics (required metrics)

| Metric | Type | Labels |
|---|---|---|
| `http_requests_total` | Counter | method, route, status |
| `http_request_duration_seconds` | Histogram | method, route |
| `connector_query_duration_seconds` | Histogram | connector_type |
| `workflow_executions_total` | Counter | status |
| `workflow_execution_duration_seconds` | Histogram | — |

- Every new connector must add a `connector_query_duration_seconds` observation.
- Every new workflow node type must propagate timing data.
- Route labels use normalized patterns (not raw paths) to control cardinality.

### 7.3 Health probes

| Endpoint | Check | Use |
|---|---|---|
| `GET /healthz` | Returns `200 OK` always | Liveness (no DB) |
| `GET /readyz` | Pings PostgreSQL | Readiness |

### 7.4 Request correlation

- `X-Request-Id` is generated or propagated on every request.
- The same ID appears in the access log line, every child log statement for that request, and the audit log entry.

---

## 8. Deployment and Production Requirements

### 8.1 Required environment variables

| Variable | Description |
|---|---|
| `DATABASE_URL` | PostgreSQL DSN |
| `JWT_SECRET` | ≥ 32-byte string for JWT signing |
| `CONNECTION_ENCRYPTION_KEY` | Base64-encoded 32-byte AES key |
| `APP_ENV` | `development` or `production` |
| `HTTP_ALLOWED_ORIGINS` | Comma-separated CORS allow-list |

### 8.2 Production checklist (all items required before launch)

- [ ] `CONNECTION_ENCRYPTION_KEY` set (32 random bytes, base64-encoded).
- [ ] `JWT_SECRET` set (≥ 32 random bytes).
- [ ] `HTTP_ALLOWED_ORIGINS` set to actual frontend origin(s).
- [ ] TLS termination at reverse proxy (nginx, ALB, Cloudflare).
- [ ] `/metrics` behind network policy or basic auth.
- [ ] `APP_ENV=production`.
- [ ] PostgreSQL backup and point-in-time recovery configured.
- [ ] Log aggregation configured (JSON logs to stdout).
- [ ] Prometheus scraping configured; alert rules for error rate, p95 latency, scheduler failures.

### 8.3 Reliability requirements

- Graceful shutdown: in-flight requests drain; scheduler stops; no request truncation on SIGTERM.
- Automatic schema migration: schema is always up-to-date on boot; no manual migration step.
- Panic recovery: a panicking handler returns 500; the server process continues.

---

## 9. Development Process

### 9.1 Spec-driven workflow

This project follows **Spec-Driven Development (SDD)** as defined by the [GitHub spec-kit](https://github.com/github/spec-kit). New features follow this workflow:

1. Update or create `specs/<feature>/spec.md` first (requirements and acceptance criteria).
2. Create `specs/<feature>/plan.md` (technical approach and design decisions).
3. Create `specs/<feature>/tasks.md` (actionable implementation tasks).
4. Implement, then run `/speckit.converge` to verify completion.

### 9.2 Artifact maintenance model

This project uses the **Living Spec** model: `spec.md` is the contract; `plan.md` and `tasks.md` are derived from it. When intended behavior changes, update `spec.md` first, then revise downstream artifacts.

### 9.3 PR requirements

- All build gates pass (§4.3).
- No new `go vet` warnings.
- No new TypeScript errors.
- Audit log entries added for any new mutating action or sensitive read.
- RBAC permission check added for any new mutating or sensitive-read route.
- Screenshots captured for any UI change (§6.5).
- Tests added for new public service methods, connectors, workflow nodes, and API routes.

### 9.4 Connector additions

New connectors must:
1. Implement the `connections.Connector` interface.
2. Add config-validation tests (no real network calls).
3. Enforce `sqlguard` if the connector accepts SQL.
4. Add a `connector_query_duration_seconds` Prometheus observation.
5. Document all supported auth schemes.
6. Classify all possible errors through `connections.Classify`.

### 9.5 Workflow node additions

New workflow nodes must:
1. Implement the `workflow.Node` interface.
2. Respect `MaxRowsPerNode` cap.
3. Have an engine integration test.
4. Have a guardrail test verifying the row cap.
5. Propagate timing data for the execution record.
