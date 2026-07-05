# internal/adapters

## What this package does

`internal/adapters` contains **infrastructure adapter implementations** that can be swapped out based on deployment configuration. Currently it holds the Redis-backed rate limiter adapter used when `REDIS_URL` is configured, as an alternative to the in-process rate limiter in `api/middleware`.

## Sub-packages

### adapters/ratelimit

- `redis.go` — a Redis-backed sliding-window rate limiter using `go-redis/redis`.
- Implements the same interface as the in-process rate limiter so the middleware layer is unaware of which backend is active.
- Enables rate limit state to be shared across multiple application replicas.

## Architecture decisions (ADRs)

| Decision | Rationale |
|---|---|
| Adapter pattern | The middleware depends on an interface, not a concrete implementation; the in-process and Redis backends are interchangeable |
| Optional Redis dependency | The application runs without Redis (in-process fallback); Redis is an opt-in for multi-replica deployments |
| Separate `adapters` package | Infrastructure details (Redis, future external adapters) are isolated from business logic |

## Scope and responsibilities

- Provide infrastructure adapters for cross-cutting concerns.
- Implement the interfaces defined in the appropriate `internal/*` packages.
- Be used only by `cmd/server/main.go` (wiring).

## Limitations and todos

- [ ] Redis is the only external adapter today; future candidates include distributed quota tracking and distributed lock (for scheduler leader election).
- [ ] No connection retry or circuit-breaker for the Redis client; a Redis outage would cause rate-limit checks to fail open or closed depending on configuration.
- [ ] No metrics on Redis adapter latency or error rate.
