# internal/api/middleware

## What this package does

`internal/api/middleware` contains the **ordered HTTP middleware chain** applied to every incoming request. Each middleware is a small, focused function; their composition is defined in `internal/api/router.go`.

## Middleware (in execution order)

### requestid (`requestid.go`)

- Reads `X-Request-Id` from the incoming request header; generates a new UUID v4 if absent.
- Attaches the ID to the request context (`platform/logger.WithRequestID`).
- Echoes it back in the `X-Request-Id` response header.
- Every subsequent middleware and handler can read it for structured log correlation.

### recover (`recover.go`)

- Catches any `panic` from downstream handlers.
- Logs the stack trace via the structured logger.
- Returns a `500 Internal Server Error` JSON response.
- Prevents a single panicking handler from crashing the entire server process.

### securityheaders (`securityheaders.go`)

Sets defensive HTTP response headers on every response:

| Header | Value |
|---|---|
| `Content-Security-Policy` | Restrictive; blocks inline scripts, external images |
| `X-Content-Type-Options` | `nosniff` |
| `X-Frame-Options` | `DENY` |
| `Referrer-Policy` | `strict-origin-when-cross-origin` |
| `Permissions-Policy` | Disables camera, microphone, geolocation |
| `Strict-Transport-Security` | `max-age=63072000; includeSubDomains` (production only) |

### CORS (`router.go` via `chi/cors`)

- Explicit allow-list from `HTTP_ALLOWED_ORIGINS` configuration.
- Never `*`; wildcards are disallowed.
- Credentials flag enabled for the refresh-token cookie.

### accesslog (`accesslog.go`)

- Writes one structured `slog` log line per request after the response is sent.
- Fields: `request_id`, `method`, `path`, `status`, `duration_ms`, `actor` (user ID if authenticated), `route` (normalized pattern, not raw path).
- Records a Prometheus histogram observation for `http_request_duration_seconds` and increments `http_requests_total`.

### auth (`auth.go`)

- Parses the `Authorization: ****** <jwt>` header if present.
- Verifies the JWT signature and expiry.
- Builds an `rbac.Principal` from the JWT claims and attaches it to the context.
- **Does not reject** unauthenticated requests — public routes (`/healthz`, `/api/v1/auth/login`, etc.) share the same chain. Rejection happens at `RequirePermission`.

### ratelimit (`ratelimit.go`)

Two separate token-bucket limiters:

| Limiter | Scope | Default |
|---|---|---|
| General | Per IP, all routes | Configurable via `RATE_LIMIT_REQUESTS_PER_SECOND` |
| Auth | Per IP, `/auth/login`, `/auth/register`, `/auth/refresh` | Stricter; configurable separately |

Returns `429 Too Many Requests` with a `Retry-After` header when the limit is exceeded.

### RequirePermission (`router.go`)

- Reads the `rbac.Principal` from the request context (set by `auth` middleware).
- Returns `401 Unauthorized` if no principal is present (unauthenticated request hitting a protected route).
- Returns `403 Forbidden` if the principal lacks the required permission.
- Applied per-route in `router.go` — one permission code per route.

## Architecture decisions (ADRs)

| Decision | Rationale |
|---|---|
| `Authenticate` does not reject | Separation of concerns: authentication (who are you?) is separate from authorization (are you allowed?); public routes share one chain |
| `RequirePermission` at router, not in handlers | Auditable; `router.go` is the single source of truth for access control |
| Per-IP rate limiting | Stateless; no session or user identity needed; effective against unauthenticated abuse |
| Security headers on every response | Defense in depth; no handler can accidentally omit them |

## Scope and responsibilities

- Implement each middleware function.
- Emit structured log lines and Prometheus metrics.
- Handle authentication (JWT parsing) and rate limiting.
- Set security response headers.

## Limitations and todos

- [ ] Rate limiter state is in-memory; restarting the process resets all buckets; multi-replica deployments have independent limits per instance.
- [ ] No distributed rate limiting (Redis-backed).
- [ ] `Recover` logs the panic but does not emit a Prometheus counter — useful for alerting on panic rates.
- [ ] CORS allow-list requires a process restart to change.
