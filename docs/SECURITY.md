# Security

This document describes the security model, the controls currently in
place, and known limitations. Data Explorer handles third-party credentials
and potentially sensitive data, so this is treated as a first-class design
concern, not an afterthought.

If you find a vulnerability, please report it privately rather than opening
a public issue.

## Identity and sessions

- **Passwords** are hashed with **Argon2id** (`internal/platform/crypto`),
  the current recommended algorithm for password storage - memory-hard,
  tunable, resistant to GPU/ASIC cracking. Parameters live in one place and
  can be raised as hardware improves without changing the stored format.
- **Access tokens** are short-lived (15 minutes by default) signed JWTs
  (HS256) carrying the caller's resolved permission set, so authorization
  never needs a database round trip. A stolen access token is only useful
  for a short window.
- **Refresh tokens** are opaque, high-entropy random values. Only their
  SHA-256 hash is stored server-side - a stolen database dump does not by
  itself grant sessions. They live in an `httpOnly`, `SameSite=Strict`
  cookie scoped to `/api/v1/auth`, so they are inaccessible to JavaScript
  (mitigating XSS token theft) and are rotated (old token revoked, new one
  issued) on every use.
- **Login responses are timing-uniform** on the "user not found" vs. "wrong
  password" paths (`auth.Service.Login`) and return the same generic error,
  so the API doesn't leak which emails are registered.
- New self-registered accounts get the **`viewer`** role only (least
  privilege by default); elevation is an explicit admin action.

## Authorization (RBAC)

- Permissions are fixed, fine-grained strings (see
  `internal/rbac/rbac.go`); every mutating and every sensitive-read route is
  gated by exactly one permission code (`internal/api/router.go`) - there is
  no "if admin, skip the check" escape hatch anywhere in the handler layer.
- Authorization is enforced **server-side only**. The frontend's
  `<PermissionGate>` hides UI a user can't use, but every API call is
  re-checked independently; hiding a button is a UX nicety, not a security
  boundary.

## Secrets at rest

- Connection credentials (DB passwords, API keys, bearer tokens) are
  encrypted with **AES-256-GCM** (authenticated encryption - confidentiality
  *and* integrity) before being written to `connections.secret_encrypted`.
  Each encryption uses a fresh random nonce.
  See `internal/platform/crypto.Encryptor`.
- The encryption key (`CONNECTION_ENCRYPTION_KEY`) is a 32-byte key supplied
  via environment variable, never committed, never derived from anything
  guessable. Config validation refuses to start in `APP_ENV=production`
  without one explicitly set (`internal/config/config.go`). Generate one
  with `openssl rand -base64 32`.
- Secrets are decrypted **in-memory only**, exclusively inside
  `connections.Service`, immediately before a connector dials out. They are
  never included in an API response (`domain.Connection` has no secret
  field), never logged, and never echoed back in error messages.
- Rotate the encryption key by re-encrypting all `secret_encrypted` values
  with the new key in a maintenance script; there is currently no automatic
  key-rotation tooling (see Known limitations).

## Data source access (SQL connectors)

- **Read-only by construction, twice over.** First, operators are expected
  to provision a read-only database role for each connection - this is the
  primary control. Second, `internal/connections/connectors/sqlguard.go`
  independently rejects anything that isn't a single `SELECT`/`WITH`
  statement at the application layer (blocks `INSERT`/`UPDATE`/`DELETE`/
  `DROP`/... by keyword, and rejects stacked statements) as defense in
  depth - not a substitute for least-privilege credentials.
- **Bounded resource usage.** Every query result is capped
  (`connections.MaxRowLimit` = 10,000 rows hard ceiling,
  `DefaultRowLimit` = 1,000), and Postgres queries set a 25s
  `statement_timeout` server-side in addition to the client-side context
  timeout, so one runaway exploration query can't exhaust memory or hang a
  connection indefinitely.
- **Parameterized queries.** User-supplied `params` are always passed
  through the driver's parameter binding (`pgx`/`database/sql`), never
  string-concatenated into SQL.

## REST connector

- `baseUrl` must be `http`/`https`; secrets are attached per the connection's
  configured `authType` (`bearer`/`apiKey`/`basic`) and never exposed to the
  frontend.
- Response bodies are capped at 25MB (`io.LimitReader`) to bound memory use
  against a misbehaving or malicious upstream.

## Transport and HTTP hardening

- `internal/api/middleware/securityheaders.go` sets `X-Content-Type-Options:
  nosniff`, `X-Frame-Options: DENY`, a locked-down `Content-Security-Policy`
  (the API never serves HTML, so this can be maximally strict), and HSTS
  when served over TLS.
- CORS is an explicit allow-list (`HTTP_ALLOWED_ORIGINS`), not a wildcard,
  with credentials enabled only for those origins.
- Per-IP rate limiting is applied globally and more aggressively on
  `/auth/login`, `/auth/register`, and `/auth/refresh` to blunt
  credential-stuffing and brute-force attempts
  (`internal/api/middleware/ratelimit.go`).
- Request bodies are size-capped (1MB) and decoded with
  `DisallowUnknownFields`, rejecting unexpected/oversized payloads early.

## Audit trail

Every mutating action and every security-sensitive read (connection tests,
ad-hoc/workflow queries) is recorded to an **append-only** `audit_logs`
table - actor, action, resource, outcome, IP, user agent, and a metadata
blob (`internal/audit`). There are intentionally no update/delete endpoints
for audit entries, to preserve evidentiary integrity. Audit writes are
best-effort and detached from request cancellation (`context.WithoutCancel`)
so an audit-log outage degrades observability, not the availability of the
feature being audited.

## Known limitations / not yet implemented

Being upfront about what this is *not*, yet:

- **No automatic secret/key rotation tooling.** Rotating
  `CONNECTION_ENCRYPTION_KEY` or `JWT_SIGNING_KEY` today requires a manual
  migration script and, for the JWT key, invalidates all outstanding
  sessions.
- **No SSO/OIDC integration.** Authentication is local email+password only.
- **No per-column/row data masking.** Anyone with `connections:read` and
  access to a connection can see all columns/rows that connection's
  credentials expose, subject to the row limit.
- **Rate limiting is in-process and per-instance.** Behind a
  horizontally-scaled deployment, front the API with a shared limiter (e.g.
  at a load balancer/API gateway) in addition to the built-in one.
- **The `EnsureReadOnlySQL` guard is a keyword/prefix check, not a full SQL
  parser.** It is a defense-in-depth layer, not the primary control - always
  use least-privilege, read-only database credentials for connections.
