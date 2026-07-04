# FR-11 — Observability, Guardrails & Reliability

## Overview

Data Explorer treats **runtime observability** and **safety guardrails**
as product features, not afterthoughts. Every mutating action, external
call, and workflow node emits **structured logs**, contributes to
**Prometheus metrics**, and is bounded by **hard resource limits** so a
single misbehaving request can never take down the platform. A
dedicated **health/readiness surface** allows the platform to be
deployed behind any orchestrator that speaks HTTP probes. Combined,
these controls let operators run Data Explorer at scale with confidence
and let end users trust that the tool won't quietly hang, silently drop
data, or exhaust the server.

## Product goals

- Give operators a **first-class Prometheus story**: any deployment can
  scrape `/metrics` and get useful golden-signal data without a plugin.
- Ensure **every log line is structured**, carries a request id, and
  is safe to ingest into ELK / Loki / Datadog / CloudWatch without
  ad-hoc parsers.
- Enforce **hard, non-bypassable resource limits** on every dimension
  a user can influence: request bodies, redirects, pagination pages,
  rows per node, run duration.
- Expose a **health / readiness probe** distinct from `/metrics` so
  container platforms (Kubernetes, ECS, Nomad) can drive rolling
  restarts safely.
- Make **failure taxonomy uniform**: the same 8 error codes drive
  audit, connection health, run failures, and export failures.

## User personas

| Persona            | Description                                                                                        |
| ------------------ | -------------------------------------------------------------------------------------------------- |
| **SRE / Operator** | Runs the platform; needs Prometheus, logs, and probes to be integratable and reliable.             |
| **Admin**          | Wants to know when things are healthy or not and get pointed at what to fix.                       |
| **Editor / Analyst** | Benefits indirectly: guardrails prevent a mistake from wedging the platform.                     |
| **Security team**  | Verifies that misuse (huge payloads, redirect loops, credential stuffing) cannot succeed at scale. |

## User stories

- **US-11.1** As an SRE, I want a `/metrics` endpoint exposing Go
  runtime, HTTP, database, workflow, and scheduler metrics, so I can
  scrape it with Prometheus.
- **US-11.2** As an operator, I want structured JSON log lines with a
  request-id, so I can grep by request across services.
- **US-11.3** As an SRE, I want `/healthz` and `/readyz` endpoints, so
  I can drive Kubernetes probes.
- **US-11.4** As a security engineer, I want a hard cap on outbound
  HTTP response size so a malicious source cannot serve us a
  25GB body.
- **US-11.5** As an SRE, I want the platform to reject requests with
  more than N redirects, so we can't be walked into an SSRF trap or
  a redirect loop.
- **US-11.6** As an SRE, I want per-connection rate limits so a
  runaway workflow cannot hammer an external API.
- **US-11.7** As an admin, I want any error surfaced anywhere in the
  product to be one of a small, closed set of codes with a
  human-readable remediation, so I can triage without a debugger.
- **US-11.8** As an SRE, I want workflow runs to time out and abort
  after 2 minutes, so a stuck run doesn't hold a worker forever.

## Functional requirements

### FR-11.1 — Metrics endpoint

`GET /metrics` SHALL expose a Prometheus-formatted metrics dump
including at least:

- Go runtime metrics via `prometheus.NewGoCollector`.
- Process metrics via `prometheus.NewProcessCollector`.
- HTTP metrics: `http_requests_total{method,route,status}`,
  `http_request_duration_seconds{method,route}` histogram.
- Database metrics: `db_connections_open`, `db_query_duration_seconds`
  histogram labelled by operation.
- Auth metrics: `auth_login_total{outcome}`,
  `auth_refresh_total{outcome}`.
- Connections metrics: `connection_health_check_total{code}`,
  `connection_health_check_duration_seconds`.
- Workflow metrics: `workflow_run_started_total`,
  `workflow_run_completed_total{status}`,
  `workflow_run_duration_seconds` histogram,
  `workflow_node_duration_seconds{type}` histogram,
  `workflow_run_row_count` histogram.
- Scheduler metrics: `scheduler_dispatch_total`,
  `scheduler_skip_total{reason}`, `scheduler_lag_seconds` histogram.

Metrics endpoint SHALL be unauthenticated (typical Prometheus
practice) but MAY be firewalled to the metrics scraper by
deployment configuration.

### FR-11.2 — Structured logs

Every log line SHALL:

- Be JSON-encoded via `slog`.
- Include `time` (RFC 3339), `level`, `msg`, `request_id`
  (when in HTTP context), `user_id` (when authenticated),
  `route`, and event-specific fields.
- NOT include plaintext secrets, tokens, decrypted credentials, or
  full row payloads.
- Use `slog.String("key", val)` style — no `fmt.Sprintf` templating
  inside log calls (see repository style).

### FR-11.3 — Request id middleware

Every incoming HTTP request SHALL be assigned an X-Request-Id
either:

- from the trusted `X-Request-Id` header (if configured to trust
  it), or
- newly generated as a UUID.

The request id SHALL be propagated to the context and included in
every downstream log line, every audit event, and every response
via the `X-Request-Id` header.

### FR-11.4 — Health and readiness probes

- `GET /healthz` — SHALL return `200 OK` as long as the process is
  running. Not authenticated.
- `GET /readyz` — SHALL return `200 OK` when the database is
  reachable and migrations have completed; otherwise `503`.

Both endpoints SHALL respond in ≤ 100ms at p99.

### FR-11.5 — HTTP outbound guardrails

The shared outbound HTTP client (`pkg/httpclient`) SHALL enforce:

- **25MB response-body cap** — abort with `invalid_config` if a
  single response exceeds this cap.
- **5-redirect cap** — abort with `invalid_config` if the redirect
  chain exceeds this depth.
- **20-page pagination cap** — abort with `invalid_config` if a
  paginated call exceeds this many pages.
- **Configurable per-request timeout** (default 30s).
- **DNS rebinding protection** for connectors that resolve targets
  by hostname.

### FR-11.6 — Workflow execution guardrails

Workflow runs SHALL enforce:

- 2-minute whole-run timeout.
- 60-second per-node timeout.
- 100,000 row cap per node output.
- 200 nodes / 500 edges structural cap (enforced at save time).
- Per-connection rate limit shared with Explore and Connections
  test.

### FR-11.7 — Rate limits

The platform SHALL apply the following rate limits, each measured
in a rolling window (default 1 minute):

- **Login**: N failed attempts per user before back-off; N failed
  attempts per IP before back-off. Defaults documented in
  [`../SECURITY.md`](../SECURITY.md).
- **Per-connection**: N test/explore/source-node requests per
  connection per minute (default sized to the connector's known
  provider limits).
- **Global**: N requests per client IP per minute (crude DDoS
  break).

When any limit is exceeded, the response SHALL be `429 Too Many
Requests` with a `Retry-After` header and error code
`rate_limited`.

### FR-11.8 — Error taxonomy

Every user-visible error SHALL classify to one of these codes,
matching the taxonomy defined in FR-05:

- `timeout`
- `network_unreachable`
- `auth_failed`
- `permission_denied`
- `not_found`
- `rate_limited`
- `invalid_config`
- `unknown`

Each classification SHALL carry a `message` (short, technical) and
a `remediation` (user-facing next step).

### FR-11.9 — Graceful shutdown

On receiving SIGTERM the server SHALL:

- Stop accepting new HTTP connections.
- Drain in-flight requests up to a configurable timeout (default
  30 seconds).
- Cancel any running workflow executions (they end with
  `cancelled` status).
- Persist scheduler state so restart safety holds.
- Close database connections cleanly.

### FR-11.10 — Panic recovery

Every HTTP handler and every workflow node executor SHALL be
wrapped in a panic-recovery middleware that:

- Logs the panic (including a truncated stack trace) at `error`
  level with the request id.
- Emits a `system.panic` audit event with a truncated message.
- Returns `500 Internal Server Error` with error code `unknown`
  and a generic remediation message that does not leak stack
  contents.

## UI/UX requirements

- The admin's Dashboard shows an "System health" tile including
  overall status, DB connectivity, scheduler lag, and open
  connection count — see
  [`docs/screenshots/02-dashboard-light.png`](../screenshots/02-dashboard-light.png)
  and
  [`docs/screenshots/03-dashboard-dark.png`](../screenshots/03-dashboard-dark.png).
- Rate-limit and guardrail errors surface with the standard
  error-panel component and clear remediation copy (e.g.
  "You have run too many queries against this connection recently
  — wait a moment and retry.").
- 429 responses in the client trigger a toast with the
  `Retry-After` seconds counted down.
- The connection health panel exposes the last error's classification
  as a color-coded chip — see
  [`docs/screenshots/29-connection-health-panel.png`](../screenshots/29-connection-health-panel.png).

## Acceptance criteria

- [ ] `GET /metrics` returns Prometheus text with at least the
  metric families listed in FR-11.1.
- [ ] Every response includes an `X-Request-Id` header whose value
  appears in the corresponding server log entry.
- [ ] `GET /healthz` returns 200 in under 100ms.
- [ ] `GET /readyz` returns 503 when the DB is unreachable and 200
  when it is.
- [ ] A REST connection whose upstream returns a 30MB body fails
  with error code `invalid_config` and a remediation citing the
  25MB cap.
- [ ] A REST connection whose upstream redirects 6 times fails with
  error code `invalid_config` citing the redirect cap.
- [ ] A workflow run whose total wall-clock exceeds 2 minutes ends
  with status `timeout`.
- [ ] A workflow node that runs longer than 60 seconds fails with
  `timeout`; downstream nodes are not executed.
- [ ] A node whose output would exceed 100K rows is clamped and the
  run is flagged with a row-cap warning.
- [ ] Repeated failed login attempts for the same user return `429`
  with `Retry-After` after the configured threshold.
- [ ] A panic in a handler produces a `500` with error code
  `unknown` and a `system.panic` audit event with a request-id
  matching the response header.
- [ ] SIGTERM drains in-flight requests before shutdown (verified
  by an integration test that sends a request during termination).
- [ ] Every error surfaced in the UI includes one of the 8
  taxonomy codes.

## Edge cases & error handling

- **Clock skew across pods**: The scheduler tolerates bounded skew
  because it dispatches at most one "catch-up" run per schedule
  (see FR-08).
- **Prometheus scrape flood**: `/metrics` is not authenticated but
  is subject to the global rate limit if a mis-configured scraper
  polls too frequently.
- **Excessive concurrent workflows**: The engine's worker pool
  bounds concurrency; excess dispatches queue in memory up to a
  configurable ceiling, past which the scheduler emits a `capacity`
  skip.
- **Memory pressure**: The engine streams row batches and enforces
  the 100K-row cap per node so memory usage stays bounded.
- **Log injection**: Structured `slog` calls SHALL never format
  user-supplied strings into the message field; user input goes
  into a distinct field.
- **Metric cardinality explosion**: Label values (e.g. `route`)
  SHALL be the templated route (`/api/v1/connections/{id}`), not
  the resolved id, to bound cardinality.
- **DB connection exhaustion**: `db_connections_open` is exported;
  connection acquisition timeout is configurable.
- **Panic in the panic handler**: The recovery middleware is
  wrapped by the standard Go `defer/recover` in the top-level
  server, so a nested panic still terminates the request rather
  than the process.

## Non-functional requirements

- **Latency overhead**: Structured logging + request id middleware
  SHALL add < 1ms to a request at p99.
- **Metric availability**: `/metrics` SHALL never return an error
  under normal operation; a scrape failure MUST be treated as a
  bug.
- **Metric freshness**: All histograms SHALL be observed at the
  time of the event, not lazily on scrape.
- **Log volume**: Log-level configurable via env; default `info`.
  Debug logs SHALL NOT be enabled by default in production.
- **Isolation**: Guardrail enforcement lives in the service /
  engine / httpclient layers, not in the frontend — clients cannot
  disable them.
- **Backward compatibility**: Metric names SHALL follow Prometheus
  naming conventions and SHALL NOT change name/label semantics
  without a documented migration.

## Market context & differentiation

| Product           | Observability & guardrail story                                       | Notes                                                                            |
| ----------------- | --------------------------------------------------------------------- | -------------------------------------------------------------------------------- |
| **Grafana**       | Native Prometheus + Tempo + Loki integration                          | Best-in-class, but Grafana itself is the observability tool.                     |
| **Retool**        | Basic health page, self-hosted telemetry                              | Not first-class Prometheus.                                                      |
| **Airflow**       | StatsD / Prometheus exporter (external)                               | Requires a sidecar exporter for Prometheus.                                      |
| **n8n**           | Metrics via env flag (community); enterprise for full observability   | Feature-gated.                                                                   |
| **Airbyte**       | OpenTelemetry / Prometheus                                            | Reasonable coverage; sync-focused metrics.                                       |
| **Metabase**      | Basic health; enterprise for full audit / logs                        | Tiered.                                                                          |
| **Superset**      | Flask health; metrics via community plugin                            | External work.                                                                   |
| **Postman**       | SaaS-only observability                                               | Not self-hosted.                                                                 |

Data Explorer's differentiators for observability & guardrails:

- **Prometheus, request-id logging, health probes, and hard
  guardrails on day one.** No paid tier, no plugin, no sidecar.
- **A single 8-code error taxonomy.** From health probes to
  workflow runs to exports to audit failures — every error surface
  speaks the same vocabulary.
- **Guardrails are enforced by the platform, not documented as
  suggestions.** Body size, redirects, pagination pages, node rows,
  and run duration all have server-side hard limits.
- **Metrics labels bounded by design.** Route templates avoid the
  classic per-id cardinality explosion.
- **Panic recovery + audit event.** A crash never disappears
  quietly.
- **SIGTERM drain + workflow cancellation.** Rolling restarts on
  Kubernetes are safe by default.

## Future enhancements (out of scope)

- OpenTelemetry tracing integration.
- Configurable per-endpoint rate limits via admin UI.
- Persistent alert rules with dispatch to email / Slack / PagerDuty.
- Log sampling for high-cardinality event families.
- Circuit-breaker metrics per external service.
- CPU / heap flamegraphs from `/debug/pprof` gated on `admin`.
- Sentry-style error grouping UI.
- SLO / burn-rate dashboards shipped as importable Grafana JSON.

## Cross-references

- [FR-01 Authentication & Session Management](./FR-01-authentication-and-sessions.md)
- [FR-05 Connection Health Monitoring](./FR-05-connection-health-monitoring.md) —
  shares the error taxonomy.
- [FR-07 Visual Workflow Builder](./FR-07-visual-workflow-builder.md) —
  guardrails on run duration, per-node timeouts, node rows.
- [FR-08 Workflow Scheduling & Automation](./FR-08-workflow-scheduling.md) —
  scheduler metrics and skip behaviour.
- [FR-10 Audit Logging & Compliance](./FR-10-audit-logging.md) —
  panic events, meta-audit.
- [`../SECURITY.md`](../SECURITY.md) — the guardrail policy source of truth.
