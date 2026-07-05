# internal/api/handlers

## What this package does

`internal/api/handlers` contains the **thin HTTP handler functions** for every API endpoint. Handlers are responsible for decoding requests, calling services, encoding responses, and recording audit log entries. No business logic belongs here.

## Handler files

| File | Resource | Key endpoints |
|---|---|---|
| `auth.go` | Authentication | `POST /auth/register`, `POST /auth/login`, `POST /auth/refresh`, `POST /auth/logout`, `POST /auth/change-password` |
| `connections.go` | Connections | CRUD for connections, `POST .../test`, `GET .../query` |
| `explore.go` | Ad-hoc exploration | `POST /explore/query` (saved connection or inline temp connection) |
| `workflows.go` | Workflows | CRUD for workflows, `POST .../execute`, `GET .../executions` |
| `catalog.go` | Integration catalog | `GET /catalog`, `GET /catalog/{id}` |
| `audit.go` | Audit log | `GET /audit-logs` with filter parameters |
| `users.go` | User management | `GET /users`, `PATCH /users/{id}`, role assignment |
| `health.go` | Health probes | `GET /healthz` (liveness), `GET /readyz` (readiness, pings DB) |
| `guardrails.go` | Guardrail status | `GET /guardrails` — current limits and quota status |
| `search.go` | Global search | `GET /search?q=…` — cross-resource search |
| `oidc.go` | OIDC login | `GET /auth/oidc/{provider}`, `GET /auth/oidc/{provider}/callback` |
| `handlers.go` | Shared helpers | `recordAudit`, `principal`, JSON helpers shared by all handlers |
| `helpers.go` | Request parsing | Pagination params, filter parsing, ID extraction |

## Patterns

### Request decoding

```go
var req CreateConnectionRequest
if err := httpx.DecodeJSON(r, &req); err != nil {
    httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error())
    return
}
```

Every handler uses `httpx.DecodeJSON` — enforces `DisallowUnknownFields` and the 1 MB body cap.

### Audit recording

```go
defer h.recordAudit(r.Context(), audit.Entry{
    Action:       "connection.create",
    ResourceType: "connection",
    ResourceID:   conn.ID,
    Outcome:      outcome,  // set to success or failure
})
```

Handlers call `recordAudit` for every mutating action. The `defer` pattern ensures the audit entry is written even if the handler returns early on error.

### Error responses

- `400 Bad Request` — invalid input (`invalid_request`)
- `401 Unauthorized` — missing or invalid token (`unauthorized`)
- `403 Forbidden` — insufficient permissions (`forbidden`)
- `404 Not Found` — resource does not exist (`not_found`)
- `409 Conflict` — duplicate resource (`conflict`)
- `422 Unprocessable Entity` — business rule violation (`validation_error`)
- `429 Too Many Requests` — rate limit or quota exceeded (`rate_limited`)
- `500 Internal Server Error` — unexpected error (`internal_error`)

## Architecture decisions (ADRs)

| Decision | Rationale |
|---|---|
| Handlers never touch repositories | Keeps authorization and validation logic testable without HTTP infrastructure |
| `defer recordAudit` pattern | Ensures audit entries are written for all outcomes, including early returns |
| Shared `handlers.go` helpers | Avoids duplication of principal extraction, audit recording, and response writing |
| One file per resource area | Easy to navigate; changes to connections don't require opening unrelated files |

## Scope and responsibilities

- Decode HTTP request bodies and URL parameters.
- Call the appropriate service method.
- Encode service results as JSON responses.
- Record audit log entries for every mutating or sensitive-read action.
- Return well-formed error responses with stable codes.

## Limitations and todos

- [ ] No response envelope versioning; adding fields to responses is non-breaking, but removing them requires a new API version.
- [ ] Pagination is offset/limit only; cursor-based pagination would be more efficient for large audit log queries.
- [ ] `explore.go` requires two permission checks (`connections:read` + conditional `connections:test`) because the query mode (saved vs. ad-hoc) is determined by the request body, not the route — documented as the only exception to "one permission per route".
