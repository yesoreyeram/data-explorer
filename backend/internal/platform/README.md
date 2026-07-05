# internal/platform

## What this package does

`internal/platform` is a collection of **small, focused infrastructure sub-packages** that the rest of the application depends on but that have no dependency on each other or on application-level packages (`auth`, `connections`, `workflow`, etc.). It provides the building blocks: logging, cryptography, database pool, HTTP helpers, migrations, memory, and JSON utilities.

## Sub-packages

### platform/logger

Structured `slog`-based logging with context propagation.

- `New(level, format)` → `*slog.Logger` (JSON by default, text in development)
- `WithRequestID(ctx, id)` / `RequestIDFromContext(ctx)` — attaches and retrieves the `X-Request-Id` correlation ID
- Every log call uses `slog.String("key", value)` — no `fmt.Sprintf` interpolation in structured log fields

### platform/crypto

Cryptographic primitives for password hashing and secret encryption.

| Function | Algorithm | Notes |
|---|---|---|
| `HashPassword(password)` | Argon2id | Parameters in one place; memory-hard, GPU/ASIC resistant |
| `VerifyPassword(hash, password)` | Argon2id | Constant-time comparison |
| `Encrypt(key, plaintext)` | AES-256-GCM | Fresh random nonce per call; authenticated encryption |
| `Decrypt(key, ciphertext)` | AES-256-GCM | Returns error on tampered or truncated ciphertext |

The 32-byte encryption key is supplied via `CONNECTION_ENCRYPTION_KEY` and never committed or logged.

### platform/dbx

`pgx/v5` connection pool setup.

- `NewPool(ctx, databaseURL)` → `*pgxpool.Pool`
- Configures pool size, acquire timeout, and health check
- Returns an error immediately if the database is unreachable (fast-fail at boot)

### platform/migrator

Embedded SQL migration runner.

- `Run(ctx, pool, fs)` — applies all pending migrations from an `embed.FS` in order
- Uses a `schema_migrations` table to track applied files
- Idempotent: a file already in `schema_migrations` is skipped

### platform/httpx

HTTP response/request helpers for the API layer.

- `WriteJSON(w, status, body)` — encodes body as JSON with the correct `Content-Type`
- `WriteError(w, status, code, message)` — standard `{"error": {"code": …, "message": …}}` error envelope
- `WriteErrorDetailed(w, status, code, message, remediation, detail)` — error envelope with optional `remediation` and `detail` fields
- `DecodeJSON(r, dst)` — decodes JSON with `DisallowUnknownFields` + 1 MB body size cap; enforces the 1 MB request body guardrail

### platform/memory

In-process LRU / TTL cache used for lightweight caching (e.g., OIDC JWKS). Has no external dependency.

### platform/safejson

JSON encoding/decoding helpers that guard against `NaN`/`Inf` float values (which are invalid JSON) and other edge cases in data that originates from external sources.

## Architecture decisions (ADRs)

| Decision | Rationale |
|---|---|
| No ORM | Full control over query shape, index use, and connection lifecycle via `pgx` directly |
| Argon2id for passwords | Current best-practice; memory-hard; tunable without changing the stored format |
| AES-256-GCM (authenticated encryption) | Provides both confidentiality and integrity; nonce-per-encrypt prevents ciphertext reuse |
| `DecodeJSON` enforces 1 MB cap | Every handler gets the request size guardrail for free without having to remember to add it |

## Scope and responsibilities

- Provide reusable, infrastructure-level utilities with no application-level knowledge.
- Be importable by any `internal/*` package without creating cycles.
- Own the cryptographic primitives so business logic never needs to touch `crypto/*` stdlib directly.

## Limitations and todos

- [ ] `platform/memory` cache does not persist across restarts; suitable only for short-lived tokens and derived data.
- [ ] No metrics emitted from the `dbx` pool (pool saturation, acquire latency) — worth adding Prometheus observations.
- [ ] `crypto` Argon2id parameters are hard-coded; a future iteration should allow them to be tuned via config without changing the hash storage format.
- [ ] `httpx` does not support multipart form decoding; file upload support would require extending it.
