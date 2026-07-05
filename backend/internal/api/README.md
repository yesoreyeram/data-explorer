# internal/api

## What this package does

`internal/api` is the **HTTP layer**: the middleware chain, handler implementations, and route table. It is a thin adapter between the HTTP transport and the service packages; no business logic lives here. Handlers call services; services call repositories; handlers never call repositories directly.

## Sub-packages

### middleware/

The ordered middleware chain applied to every request. See [`middleware/README.md`](middleware/README.md).

### handlers/

One handler file per resource area. See [`handlers/README.md`](handlers/README.md).

### router.go

The **route table**: maps every URL pattern to a handler and a permission guard. This is the single place to see "what routes exist and what permission each one requires."

```go
r.With(mw.RequirePermission(rbac.PermConnectionsWrite)).
    Post("/api/v1/connections", h.CreateConnection)
```

Every route that mutates state or exposes sensitive data is gated by exactly one `RequirePermission` call. There is no "if admin, skip" escape hatch.

## Request lifecycle (summary)

```
HTTP request
  → RequestID         (assign/propagate X-Request-Id)
  → Recover           (panic → 500)
  → SecurityHeaders   (CSP, X-Frame-Options, etc.)
  → CORS              (allow-list from HTTP_ALLOWED_ORIGINS)
  → AccessLog         (structured log line + Prometheus observation)
  → Authenticate      (parse JWT → attach rbac.Principal to ctx; not rejecting)
  → RateLimit         (per-IP global + stricter on /auth/*)
  → RequirePermission (route-specific authorization; rejects 401/403)
  → Handler           (calls service; writes JSON response)
```

## JSON conventions

All requests and responses use JSON.

- Request bodies are decoded with `httpx.DecodeJSON` — enforces `DisallowUnknownFields` and a 1 MB body size cap.
- Error responses use the standard envelope: `{"error": {"code": "snake_case", "message": "human string"}}`.
- Success responses with optional `remediation`/`detail` fields via `httpx.WriteErrorDetailed`.

## Architecture decisions (ADRs)

| Decision | Rationale |
|---|---|
| Handlers are thin adapters | Business logic in services is testable without an HTTP server |
| One permission per route in `router.go` | Readable, auditable; no permission logic scattered across handlers |
| `httpx.DecodeJSON` everywhere | Consistent input validation; 1 MB cap and `DisallowUnknownFields` applied automatically |
| Chi router | Lightweight, idiomatic; supports middleware chains and route groups cleanly |

## Scope and responsibilities

- Define and register all API routes with their permission guards.
- Apply the middleware chain to every request.
- Decode requests and encode responses with consistent JSON conventions.
- Delegate all business logic to service packages.
- Record audit log entries for mutating actions.

## Limitations and todos

- [ ] No versioning beyond the `/api/v1/` prefix; breaking changes require a new prefix.
- [ ] No OpenAPI/Swagger spec is auto-generated from the route table.
- [ ] No request validation beyond `DisallowUnknownFields` and 1 MB cap; field-level validation is done per-service.
- [ ] No GraphQL API surface (only REST).
