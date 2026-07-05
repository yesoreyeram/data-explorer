# Data Explorer — Product Specification

> **Spec-Kit artifact** — This is the product requirements document (PRD) for Data Explorer. It defines *what* to build and *why*, with explicit acceptance criteria for each requirement. For *how* to build it, see `plan.md`. For the implementation task list, see `tasks.md`. For governing principles, see `.specify/memory/constitution.md`.

---

## Table of Contents

1. [Problem Statement](#1-problem-statement)
2. [Goals and Non-Goals](#2-goals-and-non-goals)
3. [Personas](#3-personas)
4. [User Stories](#4-user-stories)
5. [Functional Requirements](#5-functional-requirements)
6. [Non-Functional Requirements](#6-non-functional-requirements)
7. [Data Model](#7-data-model)
8. [API Contract](#8-api-contract)
9. [Out of Scope (v1)](#9-out-of-scope-v1)
10. [Open Questions](#10-open-questions)

---

## 1. Problem Statement

Data teams and engineers in enterprise environments need to:
- Query multiple heterogeneous data sources (databases, REST APIs, cloud services) from a single interface.
- Build repeatable data pipelines without writing code from scratch each time.
- Maintain governance: access control, audit trails, and guardrails on what data can be queried.

Existing tools either require heavy infrastructure (full-scale ETL platforms), lack security primitives (ad-hoc notebook tools), or don't support the breadth of connectors needed. Teams end up with fragmented scripts, undocumented connection credentials, and no audit trail.

**Data Explorer solves this** by providing a self-hosted platform that combines ad-hoc data exploration, a visual pipeline builder, enterprise RBAC, an append-only audit log, and built-in observability — deployable as a single binary backed by PostgreSQL.

---

## 2. Goals and Non-Goals

### Goals

- **Connect** — Centrally manage encrypted-at-rest connections to PostgreSQL, MySQL, REST APIs, GraphQL APIs, AWS (Athena, CloudWatch, DynamoDB, S3), GCP (BigQuery, GCS), and Azure (Log Analytics, Blob Storage).
- **Explore** — Ad-hoc queries against saved or temporary (never-persisted) connections; no pipeline required.
- **Build** — Drag-and-drop visual pipeline builder: source → filter → transform → join → aggregate → output.
- **Schedule** — Cron-based workflow scheduling with the same execution path and guardrails as manual runs.
- **Govern** — Fine-grained RBAC, append-only audit trail, row/size/rate guardrails at every layer.
- **Observe** — Prometheus metrics, structured logging, health probes, classified connection errors.

### Non-Goals (v1)

- Mobile browser support (desktop-first, minimum 1024px).
- Multi-tenancy or org-level isolation (single-tenant deployment).
- Native mobile apps.
- Embedded analytics (Data Explorer is a tool for data teams, not an embeddable widget).
- Real-time streaming data sources.
- Built-in data storage or data warehouse functionality.

---

## 3. Personas

| Persona | Role | Primary Tasks |
|---|---|---|
| **Admin** | Full platform control | Manage users, roles, connections, workflows; view audit log; configure OIDC |
| **Editor** | Data engineer / analyst | Create and manage connections and workflows; run ad-hoc queries; schedule pipelines |
| **Viewer** | Stakeholder / consumer | Browse connections; view workflow execution results; cannot create or modify |
| **Analyst** | Heavy exploration user | Ad-hoc data exploration; visual pipeline building; data export |
| **SRE / Operator** | Platform reliability | Monitor health, metrics, guardrail status; manage deployment |
| **Auditor / Compliance Officer** | Governance | Review the audit log for compliance investigations |

---

## 4. User Stories

### Authentication

- **US-01** As a **new user**, I can register with email and password so that I can access the platform. *(Admin approves; new accounts get `viewer` role.)*
- **US-02** As a **returning user**, I can log in with email and password so that I can access my work.
- **US-03** As a **user**, I can log in via SSO (OIDC) so that I don't need to manage a separate password.
- **US-04** As a **logged-in user**, I can change my password so that I can maintain my account security.
- **US-05** As a **logged-in user**, I can log out so that my session is terminated.

### Access Control

- **US-06** As an **admin**, I can assign and remove roles from users so that access is appropriately scoped.
- **US-07** As an **admin**, I can view all users and their current roles so that I can audit access.
- **US-08** As a **user**, I can only see and perform actions that my role permits so that data access is governed.

### Connection Management

- **US-09** As an **editor**, I can create a connection to a PostgreSQL database so that I can query its data.
- **US-10** As an **editor**, I can create a connection to a REST API with various auth methods so that I can pull data from web services.
- **US-11** As an **editor**, I can test a connection and see a structured error with remediation advice if it fails so that I can diagnose problems quickly.
- **US-12** As an **editor**, I can edit or delete a connection so that I can keep the catalog current.
- **US-13** As a **viewer**, I can list connections and view their (non-secret) config so that I know what data sources are available.
- **US-14** As an **editor**, I can browse the integration catalog to prefill common REST/GraphQL connection configs so that I don't have to look up base URLs.

### Data Exploration

- **US-15** As an **analyst**, I can run an ad-hoc query against a saved connection and see results in a table so that I can explore data without building a workflow.
- **US-16** As an **analyst**, I can run an ad-hoc query against temporary inline credentials (never persisted) so that I can explore without saving sensitive credentials.
- **US-17** As an **analyst**, I can export query results as CSV or JSON so that I can use the data in other tools.
- **US-18** As an **analyst**, I can chart query results (line, bar, scatter) so that I can visualize data directly in the UI.
- **US-19** As an **analyst**, recent queries against saved connections are remembered so that I can quickly re-run common queries.

### Workflow Builder

- **US-20** As an **editor**, I can create a visual pipeline with source, filter, transform, join, aggregate, and output nodes so that I can build multi-step data pipelines.
- **US-21** As an **editor**, I can connect nodes with edges and the system validates the DAG for cycles and disconnected subgraphs so that invalid pipelines are caught before execution.
- **US-22** As an **editor**, I can execute a workflow and see per-node results, timings, and row counts so that I can understand how my pipeline behaves.
- **US-23** As an **editor**, I can view execution history and see details of past runs so that I can debug failures.
- **US-24** As a **viewer**, I can view workflows and their execution history but cannot modify them so that I have read-only visibility.

### Scheduling

- **US-25** As an **editor**, I can schedule a workflow with a cron expression so that it runs automatically on a schedule.
- **US-26** As an **editor**, I can use preset schedule options (hourly, daily, weekly) so that I don't need to write cron syntax manually.
- **US-27** As an **editor**, I can enable or disable a schedule without deleting it so that I can pause automation temporarily.

### Audit Log

- **US-28** As an **auditor**, I can view the audit log filtered by actor, action, resource, and date range so that I can investigate incidents.
- **US-29** As an **auditor**, I can see that every mutating action and sensitive read is recorded with actor, IP, outcome, and request ID so that the audit trail is complete.

### Observability

- **US-30** As an **SRE**, I can access Prometheus metrics for HTTP request rates, latency, workflow execution rates, and connector query latency so that I can set up alerts and dashboards.
- **US-31** As an **SRE**, I can use `/healthz` and `/readyz` probes so that my orchestrator can manage the service lifecycle.

---

## 5. Functional Requirements

### FR-01: Authentication and Session Management

| ID | Requirement | Acceptance Criteria |
|---|---|---|
| FR-01.1 | Users register with email, password (≥ 8 chars), and display name | Registration succeeds and returns 201; duplicate email returns 409; weak password returns 400 |
| FR-01.2 | New accounts receive the `viewer` role | Newly registered user has exactly `viewer` role; can call `GET /connections` but not `POST /connections` |
| FR-01.3 | Login returns a short-lived JWT (15 min TTL) and an `httpOnly` `SameSite=Strict` refresh token cookie | Login response contains `access_token`; `Set-Cookie` header is present with `HttpOnly`, `SameSite=Strict`; JWT exp is ~15 min from issue |
| FR-01.4 | Refresh token rotates on every use | After refresh, old token returns 401 on next use; new token works |
| FR-01.5 | "User not found" and "wrong password" return the same generic error | Both paths return identical error JSON and take the same time (≤ 10 ms difference, enforced by constant-time comparison) |
| FR-01.6 | Users can change their password | Requires current password verification; new password is hashed; old refresh tokens are not automatically revoked |
| FR-01.7 | Users can log out | Refresh token is revoked; subsequent refresh attempts with the revoked token return 401 |
| FR-01.8 | OIDC login supports Authorization Code + PKCE | State parameter present in auth redirect; code verifier stored in `SameSite=Lax` cookie; `email_verified` must be true or login fails |
| FR-01.9 | First OIDC login provisions a `viewer` user | New user created with `viewer` role and OIDC email; subsequent logins match by email |

### FR-02: Role-Based Access Control

| ID | Requirement | Acceptance Criteria |
|---|---|---|
| FR-02.1 | Fine-grained permission codes | 11 permission codes defined: `users:read`, `users:write`, `roles:read`, `roles:write`, `connections:read`, `connections:write`, `connections:test`, `workflows:read`, `workflows:write`, `workflows:execute`, `audit:read` |
| FR-02.2 | Three system roles by default | `admin` (all permissions), `editor` (connections + workflows), `viewer` (read-only) — seeded on first boot |
| FR-02.3 | Admins assign and remove roles | `POST /users/{id}/roles` and `DELETE /users/{id}/roles/{roleId}` work for admin; return 403 for non-admin |
| FR-02.4 | Every route gated by exactly one permission | Automated test verifies that removing the middleware call on any route causes the permission test to fail |
| FR-02.5 | Permissions embedded in JWT | No per-request DB lookup for permission check; permissions resolved from JWT claims |

### FR-03: Connection Management

| ID | Requirement | Acceptance Criteria |
|---|---|---|
| FR-03.1 | Connections store non-secret config and AES-256-GCM encrypted credentials | Secret is not returned in any API response; `secret_encrypted` column contains ciphertext |
| FR-03.2 | Seven connection types | `postgres`, `mysql`, `rest`, `graphql`, `aws`, `gcp`, `azure` are all accepted; unknown type returns 400 |
| FR-03.3 | Per-connection rate limiting | Connector calls exceeding the rate limit return a `rate_limited` health error |
| FR-03.4 | Connection test returns structured error | Response includes `errorCode`, `errorRemediation`, `checkDurationMs`; error code is one of the defined `HealthError` values |
| FR-03.5 | Viewers can list and get connections | `GET /connections` and `GET /connections/{id}` succeed for viewer; no secret fields in response |
| FR-03.6 | Only editors and admins can create/edit/delete | `POST /connections` with viewer role returns 403 |

### FR-04: Integration Catalog

| ID | Requirement | Acceptance Criteria |
|---|---|---|
| FR-04.1 | ≥ 15 well-known integrations | Catalog contains at least 15 entries covering common REST/GraphQL APIs |
| FR-04.2 | Catalog is searchable | `GET /catalog?q=github` returns only entries whose name or description contains "github" |
| FR-04.3 | No external registry at runtime | Catalog endpoint works with no internet access |
| FR-04.4 | Catalog entries never supply credentials | No API key or secret appears in any catalog entry |

### FR-05: Connection Health Monitoring

| ID | Requirement | Acceptance Criteria |
|---|---|---|
| FR-05.1 | Errors classified into stable codes | Error code is one of: `timeout`, `network_unreachable`, `auth_failed`, `permission_denied`, `not_found`, `rate_limited`, `invalid_config`, `unknown` |
| FR-05.2 | Health panel shows status badge, error code, message, and remediation | All four fields are present in the test-connection response |
| FR-05.3 | Last check data persisted on connection row | `last_tested_at`, `last_error`, `last_error_code`, `last_error_remediation`, `last_check_duration_ms` updated after each test |

### FR-06: Ad-Hoc Data Exploration

| ID | Requirement | Acceptance Criteria |
|---|---|---|
| FR-06.1 | Query saved connections | `POST /explore/query` with a saved `connectionId` returns a DataFrame result |
| FR-06.2 | Query with inline credentials (never persisted) | `POST /explore/query` with inline credentials works; no connection row created; requires `connections:test` permission |
| FR-06.3 | Results in DataFrame wire format | Response shape: `{ schema: [...], rows: [...], metadata: {...} }` |
| FR-06.4 | Export as CSV and JSON | Download button produces valid CSV (RFC 4180) and valid JSON array; truncated results include a warning |
| FR-06.5 | Recent query history (client-side) | Browser localStorage stores last N queries per saved connection; history survives page reload |
| FR-06.6 | Charting results | Line, bar, scatter chart types available when result has ≥ 2 columns |

### FR-07: Visual Workflow Builder

| ID | Requirement | Acceptance Criteria |
|---|---|---|
| FR-07.1 | Six node types | `source`, `filter`, `transform`, `join`, `aggregate`, `output` — all renderable on canvas |
| FR-07.2 | React Flow canvas | Node positions stored in definition JSONB; layout survives save/reload |
| FR-07.3 | Cycle detection | Creating a cycle returns a validation error before save |
| FR-07.4 | Disconnected graph detection | Saving a workflow with a disconnected node returns 400 with the disconnected node ID |
| FR-07.5 | Node count/edge count limits | Exceeding `MaxNodes` or `MaxEdges` returns 400 |
| FR-07.6 | Topological execution order | Execution runs nodes in Kahn's-algorithm order; log shows correct ordering |
| FR-07.7 | Row cap per node | Node output truncated at `MaxRowsPerNode` (100,000); truncation noted in node result |
| FR-07.8 | Stop at first failure | Execution halts at the first failing node; downstream nodes are skipped; partial results reported |
| FR-07.9 | Execution persisted with per-node details | `workflow_executions` row contains `node_results` JSONB with per-node timings, row counts, and errors |
| FR-07.10 | Execution history panel in builder | Last N executions visible in sidebar; clicking shows per-node breakdown |

### FR-08: Workflow Scheduling

| ID | Requirement | Acceptance Criteria |
|---|---|---|
| FR-08.1 | Standard 5-field cron expression | Invalid cron expression returns 400 |
| FR-08.2 | `schedule_next_run` pre-computed at save | Field is set correctly after `PUT /workflows/{id}/schedule` |
| FR-08.3 | Scheduled executions use the same engine path | `triggered_by = "scheduler"` in execution record; same guardrails apply |
| FR-08.4 | Scheduler poll is cheap | Scheduler query uses indexed `schedule_next_run` column; no full table scan |
| FR-08.5 | Preset schedule options in UI | Dropdown offers: every 15 min, hourly, daily at midnight, weekly on Monday, monthly on the 1st |

### FR-09: Query Result Export

| ID | Requirement | Acceptance Criteria |
|---|---|---|
| FR-09.1 | CSV export (RFC 4180) | Exported file passes RFC 4180 validation; special characters properly escaped |
| FR-09.2 | JSON export (array of objects) | Exported file is valid JSON array; each object has the same keys |
| FR-09.3 | Client-side export | Export does not make a new server request; uses existing in-memory data |
| FR-09.4 | Truncation warning | If result was truncated, exported file includes a comment/header noting the truncation |

### FR-10: Audit Logging

| ID | Requirement | Acceptance Criteria |
|---|---|---|
| FR-10.1 | Every mutating action and sensitive read is logged | Comprehensive list of audited actions defined in `audit/writer.go`; gaps are security bugs |
| FR-10.2 | Audit entry fields | Entry contains: `actor_id`, `actor_email`, `action`, `resource_type`, `resource_id`, `outcome`, `ip_address`, `user_agent`, `request_id`, `metadata`, `created_at` |
| FR-10.3 | Append-only | No `UPDATE` or `DELETE` endpoint exists for `audit_logs`; migration must not add such endpoints |
| FR-10.4 | Filterable | `GET /audit-logs?actor=&action=&resource_type=&outcome=&from=&to=` all filter correctly |
| FR-10.5 | Request correlation | `request_id` in audit entry matches the `X-Request-Id` header and the structured log line for that request |

### FR-11: Observability and Guardrails

| ID | Requirement | Acceptance Criteria |
|---|---|---|
| FR-11.1 | Structured request logs | Every request produces one JSON log line with `request_id`, `actor`, `route`, `status`, `duration_ms` |
| FR-11.2 | Prometheus metrics | All 5 required metrics present at `/metrics`; cardinality controlled (routes normalized) |
| FR-11.3 | Health probes | `GET /healthz` returns 200 always; `GET /readyz` returns 200 only when DB is reachable |
| FR-11.4 | Row limit | Connector calls cap at `MaxRows` (default 10,000); truncation noted in response metadata |
| FR-11.5 | Response size cap | HTTP connectors cap response at 25 MB; larger responses return an error |
| FR-11.6 | Redirect cap | HTTP connectors follow at most 5 redirects |
| FR-11.7 | Retry with backoff | 429/502/503/504 retried with full-jitter exponential backoff |
| FR-11.8 | Max execution duration | Workflow run cancelled after 2 minutes; error recorded in execution record |
| FR-11.9 | Per-IP rate limiting | Global token-bucket limiter per IP; stricter limiter on auth endpoints |
| FR-11.10 | Per-user hourly quota | Configurable explore and workflow run quotas per role; quota exceeded returns 429 |

### FR-12: User Interface

| ID | Requirement | Acceptance Criteria |
|---|---|---|
| FR-12.1 | Near-monochrome design system | No hard-coded hex colors; all color via CSS custom properties; `success`/`warning`/`danger`/`info` confined to status dots |
| FR-12.2 | Light / dark / system theme | Theme switcher changes `data-theme` on `<html>`; preference persisted to localStorage; system theme detected on first load |
| FR-12.3 | Collapsible sidebar | Sidebar collapses to icon-only mode; state persisted |
| FR-12.4 | `<PermissionGate>` for restricted UI | Buttons/links for restricted actions are hidden (not just disabled) for users without the required permission |
| FR-12.5 | Accessible interactive elements | Axe/accessibility scan reports zero critical violations on all main pages |

---

## 6. Non-Functional Requirements

### Performance

| Requirement | Target | Measurement |
|---|---|---|
| API response time (p95, non-query endpoints) | < 200 ms | Load test with k6 at 50 concurrent users |
| Query execution (small results, < 1,000 rows) | < 2 s | Measured from request sent to first byte |
| Workflow execution (simple 3-node DAG) | < 5 s | End-to-end measured from `POST /execute` to status `success` |
| UI initial load (Lighthouse Performance score) | > 80 | Lighthouse CI in production build |
| UI time-to-interactive | < 3 s | Measured on a 2021-era corporate laptop at 25 Mbps |

### Scalability

| Requirement | Notes |
|---|---|
| Single-replica baseline | Correct operation as a single instance without Redis or external coordination |
| Horizontal scaling path | Stateless API; multiple replicas require Redis for rate limiting and a distributed lock for the scheduler |
| Database | PostgreSQL 14+; connection pool size configurable |

### Reliability

| Requirement | Notes |
|---|---|
| Graceful shutdown | In-flight requests drain; scheduler stops; no request truncation on SIGTERM |
| Automatic schema migration | Schema always up-to-date on boot; no manual migration step |
| Health probes | `/healthz` and `/readyz` for orchestrator liveness/readiness checks |
| Panic recovery | Panicking handler returns 500; server process continues |

### Security

- All security requirements in the project constitution §3 apply.
- OWASP Top 10 must be addressed: SQL injection prevention via parameterized queries, XSS prevention via CSP headers, CSRF protection via `SameSite` cookies, broken access control prevented by per-route RBAC middleware.

---

## 7. Data Model

### Core tables

```sql
users (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email          TEXT UNIQUE NOT NULL,
    display_name   TEXT NOT NULL,
    password_hash  TEXT,               -- NULL for OIDC-only users
    status         TEXT NOT NULL DEFAULT 'active',  -- 'active' | 'suspended'
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
)

roles (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT UNIQUE NOT NULL,
    description TEXT,
    is_system   BOOLEAN NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
)

permissions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code        TEXT UNIQUE NOT NULL,
    description TEXT
)

user_roles (
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    role_id UUID REFERENCES roles(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, role_id)
)

role_permissions (
    role_id       UUID REFERENCES roles(id) ON DELETE CASCADE,
    permission_id UUID REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
)

refresh_tokens (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT UNIQUE NOT NULL,   -- SHA-256 of the opaque token
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
)

connections (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                    TEXT NOT NULL,
    type                    TEXT NOT NULL,    -- 'postgres'|'mysql'|'rest'|'graphql'|'aws'|'gcp'|'azure'
    description             TEXT,
    config                  JSONB NOT NULL DEFAULT '{}',
    secret_encrypted        TEXT,             -- AES-256-GCM; NULL for ambient-credential connections
    status                  TEXT NOT NULL DEFAULT 'unverified',
    last_tested_at          TIMESTAMPTZ,
    last_error              TEXT,
    last_error_code         TEXT,
    last_error_remediation  TEXT,
    last_check_duration_ms  INTEGER,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
)

workflows (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name             TEXT NOT NULL,
    description      TEXT,
    definition       JSONB NOT NULL,         -- { nodes: [...], edges: [...] }
    schedule_cron    TEXT,
    schedule_enabled BOOLEAN NOT NULL DEFAULT false,
    schedule_next_run TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
)

workflow_executions (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id  UUID REFERENCES workflows(id) ON DELETE CASCADE,
    status       TEXT NOT NULL,              -- 'running' | 'success' | 'failure'
    triggered_by TEXT NOT NULL,              -- user UUID or "scheduler"
    duration_ms  INTEGER,
    node_results JSONB,                      -- per-node timings, row counts, errors
    error        TEXT,
    started_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
)

audit_logs (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id      TEXT NOT NULL,             -- user UUID, "scheduler", or "system"
    actor_email   TEXT,
    action        TEXT NOT NULL,
    resource_type TEXT,
    resource_id   TEXT,
    outcome       TEXT NOT NULL,             -- 'success' | 'failure'
    ip_address    TEXT,
    user_agent    TEXT,
    request_id    TEXT,
    metadata      JSONB,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
)
```

### Key indexes

```sql
CREATE INDEX idx_refresh_tokens_user_id      ON refresh_tokens(user_id);
CREATE INDEX idx_refresh_tokens_expires_at   ON refresh_tokens(expires_at);
CREATE INDEX idx_connections_type            ON connections(type);
CREATE INDEX idx_workflow_executions_wf      ON workflow_executions(workflow_id, started_at DESC);
CREATE INDEX idx_workflows_schedule          ON workflows(schedule_next_run) WHERE schedule_enabled = true;
CREATE INDEX idx_audit_logs_actor            ON audit_logs(actor_id, created_at DESC);
CREATE INDEX idx_audit_logs_resource         ON audit_logs(resource_type, resource_id, created_at DESC);
CREATE INDEX idx_audit_logs_action           ON audit_logs(action, created_at DESC);
```

---

## 8. API Contract

### Base

- Base path: `/api/v1/`
- Authentication: `Authorization: ****** (15-min) + rotating `httpOnly` refresh cookie
- Content-Type: `application/json`
- Request body limit: 1 MB (`httpx.DecodeJSON`)
- Error shape:
  ```json
  {
    "error": {
      "code": "snake_case_code",
      "message": "human-readable string",
      "remediation": "optional fix hint",
      "detail": "optional technical detail"
    }
  }
  ```

### Routes

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
| GET | `/connections` | `connections:read` | List connections (no secrets) |
| POST | `/connections` | `connections:write` | Create connection |
| GET | `/connections/{id}` | `connections:read` | Get connection (no secrets) |
| PUT | `/connections/{id}` | `connections:write` | Update connection |
| DELETE | `/connections/{id}` | `connections:write` | Delete connection |
| POST | `/connections/{id}/test` | `connections:test` | Test connection health |
| GET | `/connections/{id}/query` | `connections:read` | Query connection (ad-hoc) |
| GET | `/catalog` | `connections:read` | List integration catalog entries |
| GET | `/catalog/{id}` | `connections:read` | Get catalog entry |
| POST | `/explore/query` | `connections:read` [+ `connections:test` for inline creds] | Ad-hoc query with optional inline credentials |
| GET | `/workflows` | `workflows:read` | List workflows |
| POST | `/workflows` | `workflows:write` | Create workflow |
| GET | `/workflows/{id}` | `workflows:read` | Get workflow |
| PUT | `/workflows/{id}` | `workflows:write` | Update workflow |
| DELETE | `/workflows/{id}` | `workflows:write` | Delete workflow |
| POST | `/workflows/{id}/execute` | `workflows:execute` | Execute workflow |
| GET | `/workflows/{id}/executions` | `workflows:read` | List executions |
| PUT | `/workflows/{id}/schedule` | `workflows:write` | Set or clear schedule |
| GET | `/audit-logs` | `audit:read` | Query audit log |
| GET | `/search` | authenticated | Global search |
| GET | `/guardrails` | authenticated | Current guardrail status |
| GET | `/healthz` | public | Liveness probe |
| GET | `/readyz` | public | Readiness probe |
| GET | `/metrics` | public (network-gated in prod) | Prometheus metrics |

### DataFrame wire format

```json
{
  "schema": [
    { "name": "column_name", "type": "string|int64|float64|bool|timestamp" }
  ],
  "rows": [
    ["value1", 42, 3.14, true, "2024-01-01T00:00:00Z"]
  ],
  "metadata": {
    "rowCount": 100,
    "truncated": false,
    "durationMs": 42
  }
}
```

---

## 9. Out of Scope (v1)

| Item | Reason |
|---|---|
| Multi-factor authentication (TOTP / WebAuthn) | Complexity; deferred to v2 |
| Account lockout after repeated login failures | Rate limiting on auth endpoints provides partial protection |
| Device / session management UI | Deferred to v2 |
| Encryption key rotation tooling | Requires a manual script; documented as known limitation |
| Object storage > 50 MB | No streaming implementation; known limitation |
| Distributed tracing (OpenTelemetry) | Structured logs + Prometheus sufficient for v1 |
| SLO definitions or alerting rules | Left to operator |
| Mobile browser support | Desktop-first tool |
| Parallel node execution in workflows | Synchronous execution sufficient for v1 |
| Async/long-running workflow execution | 2-minute limit covers most cases in v1 |
| Kerberos auth for connectors | Low demand; deferred |
| Connection pooling across requests for HTTP connectors | Each request establishes its own connection |

---

## 10. Open Questions

| # | Question | Owner | Status |
|---|---|---|---|
| OQ-01 | Should the explore history be synced server-side (per user) rather than localStorage? | Product | Open |
| OQ-02 | Should scheduled workflow results be surfaced in a dedicated dashboard view? | Product | Open |
| OQ-03 | What is the retention policy for audit logs? Should they be auto-purged after N days? | Legal / Compliance | Open |
| OQ-04 | Should connection health checks run on a background schedule, or only on-demand? | Engineering | Open |
| OQ-05 | Should the `/metrics` endpoint require authentication, even in development mode? | Security | Open |
| OQ-06 | Is there a requirement for webhook/notification delivery when a scheduled workflow fails? | Product | Open |
