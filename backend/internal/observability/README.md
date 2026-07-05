# internal/observability

## What this package does

`internal/observability` provides the **Prometheus metrics registry** for the application. It defines and registers all counters and histograms that the application exposes at `GET /metrics`, consumed by Prometheus and visualized in Grafana or compatible tools.

## Metrics defined

| Metric name | Type | Labels | Description |
|---|---|---|---|
| `http_requests_total` | Counter | `method`, `route`, `status` | Total HTTP requests, labelled by method, normalized route, and response status code |
| `http_request_duration_seconds` | Histogram | `method`, `route` | HTTP request latency distribution |
| `connector_query_duration_seconds` | Histogram | `connector_type` | Latency of connector `Execute` calls, by connection type |
| `workflow_executions_total` | Counter | `status` | Workflow execution outcomes (`success` / `failure`) |
| `workflow_execution_duration_seconds` | Histogram | — | Full workflow execution latency |

Metrics are recorded by:
- `api/middleware/accesslog.go` — HTTP metrics after every request
- `connections.Service` — connector query latency
- `workflow.Service.Execute` — workflow execution outcomes and duration

## Architecture decisions (ADRs)

| Decision | Rationale |
|---|---|
| Single package owns all metric registrations | No scattered `prometheus.MustRegister` across packages; one place to audit all metrics |
| Normalized route label (not raw path) | Cardinality safety: `GET /connections/{id}` instead of `GET /connections/abc-123-…` for every unique ID |
| Histogram bucket defaults | Standard Prometheus defaults unless connector-specific latency is observed to need finer resolution |

## Usage

```go
// In cmd/server/main.go:
obs := observability.NewRegistry()

// Pass obs to services that record metrics:
connections.NewService(…, obs)
workflow.NewService(…, obs)

// Register /metrics route:
router.Handle("/metrics", promhttp.HandlerFor(obs.Registry, promhttp.HandlerOpts{}))
```

## Scope and responsibilities

- Define and register all Prometheus metrics.
- Expose the registry to services that need to record observations.
- Serve `/metrics` (wired in `api/router.go`).

## Limitations and todos

- [ ] No SLO / alerting rules are shipped alongside the metrics definitions; these must be configured externally in Prometheus/Alertmanager.
- [ ] No distributed tracing (no OpenTelemetry spans); spans would improve debuggability for multi-node workflow executions.
- [ ] No custom business metrics (e.g., number of connections per type, active workflow schedules).
- [ ] `/metrics` is currently unauthenticated; in a hardened deployment it should be behind network policy or basic auth.
- [ ] No exemplars on histograms for correlation with traces.
