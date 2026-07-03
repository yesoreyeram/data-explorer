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

## Outbound HTTP (REST/GraphQL connectors and `pkg/httpclient`)

The REST and GraphQL connectors are built on `backend/pkg/httpclient`, a
standalone HTTP client library (see `docs/ARCHITECTURE.md`) that centralizes
every outbound-call guardrail so no connector has to reimplement them:

- `baseUrl`/`endpoint` must be `http`/`https`.
- **Every response body is capped** (`httpclient.DefaultMaxResponseBytes` =
  25MB) - the excess is discarded, not buffered, so a malicious or
  misbehaving upstream can't exhaust memory by streaming an unbounded
  response.
- **Redirects are capped** (`MaxRedirects` = 5) to stop a redirect loop
  (accidental or adversarial) from hanging a request indefinitely.
- **Retries are bounded and jittered**: up to `RetryPolicy.MaxAttempts`
  attempts with exponential backoff *and* full jitter (never a fixed
  interval, which would let many failing clients retry in lockstep and
  amplify load on a struggling upstream), and the whole sequence is still
  subject to the client's overall `Timeout`.
- **Pagination is capped independently of row count**: `MaxPages` (default
  20, hard ceiling 500 - `httpclient.DefaultMaxPages`/`HardMaxPages`) stops a
  paginated fetch even if a misconfigured "next page" field never goes
  empty, in addition to the row-limit cap that already applies per query.
- Credentials for every auth scheme (Basic, Bearer, API key, JWT, OAuth2,
  Digest, workload identity federation, Kerberos - see
  `internal/connections/connectors/httpauth.go`) come exclusively from the
  connection's encrypted secret map, never from the plaintext `config`, and
  are attached to the outgoing request only - never logged, never returned
  by the API.
- OAuth2 (`golang.org/x/oauth2`) and workload-identity token exchange (RFC
  8693) tokens are cached in-memory and refreshed just before expiry, so a
  compromise of the API process at rest never exposes a long-lived token
  that wasn't already about to be re-minted.

## Cloud provider connectors (AWS / GCP / Azure)

The `aws`, `gcp`, and `azure` connection types query native cloud services
(Athena, CloudWatch Logs Insights, DynamoDB, S3; BigQuery, Cloud Storage;
Log Analytics, Blob Storage) through each provider's official Go SDK rather
than hand-rolled HTTP calls, so credential handling, retries, and request
signing all get the SDK's own scrutiny rather than this project's.

- **Credentials are optional in the connection's secret, by design.** If a
  connection doesn't have static credentials (AWS access key,
  GCP service-account JSON, Azure client secret) in its encrypted secret,
  the connector falls back to that cloud's *ambient* identity: the AWS SDK's
  default credential chain (an IAM role on the EC2 instance/ECS
  task/EKS pod), GCP Application Default Credentials (a GCE/GKE Workload
  Identity-bound service account), or Azure's `DefaultAzureCredential`
  (managed identity / AKS workload identity / `az login` session). Run this
  server inside the target cloud with the right role attached and no
  long-lived cloud credential needs to be stored in this database at all -
  the same principle the generic `workloadIdentity` REST/GraphQL auth
  scheme applies manually, native here via each SDK.
- **SQL query engines (Athena, BigQuery) go through the same
  `EnsureReadOnlySQL` guard as Postgres/MySQL** - a `SELECT`/`WITH` query
  only, no stacked statements. CloudWatch Logs Insights and Log Analytics
  use their own read-only query languages (Logs Insights QL, KQL) which
  have no mutation surface to guard.
- **Async query APIs (Athena, CloudWatch Logs Insights) are polled with a
  hard wall-clock cap** (`connectors.AsyncQueryMaxWait`, 55s) - both are
  start-then-poll APIs with no native blocking "wait" call, so without this
  a query stuck `RUNNING` would hold the request open indefinitely.
- **Object storage reads (S3, GCS, Blob Storage) are capped**
  (`connectors.MaxObjectBytes`, 50MB) before being parsed as
  CSV/JSON/NDJSON, for the same reason REST responses are capped -
  someone pointing a connection at a multi-gigabyte file shouldn't be able
  to bring the process down.
- **DynamoDB access uses the caller-supplied key condition/filter
  expressions verbatim** (passed straight to the SDK's expression
  parameters, never string-concatenated) - the same parameterized-query
  discipline as the SQL connectors, just via DynamoDB's own expression
  syntax rather than SQL placeholders.

## Guardrails at every layer

Defense in depth in one place - each layer assumes the ones "below" it
might fail:

| Layer | Guardrail | Where |
| --- | --- | --- |
| HTTP request in | 1MB body cap, rejects unknown fields | `httpx.DecodeJSON` |
| Auth endpoints | stricter per-IP rate limit (credential stuffing) | `api/middleware/ratelimit.go` |
| All endpoints | general per-IP rate limit | `api/middleware/ratelimit.go` |
| Per connection | per-connection-ID rate limit (protects the *downstream* system) | `connections.Service` |
| SQL connectors | read-only statement guard, parameterized queries, statement timeout | `connectors/sqlguard.go`, `connectors/postgres.go` |
| REST/GraphQL connectors | response size cap, redirect cap, bounded jittered retry | `pkg/httpclient` |
| Cloud query engines (Athena, CloudWatch Logs Insights) | bounded async poll wait (55s) | `connectors.AsyncQueryMaxWait` |
| Cloud object storage (S3, GCS, Blob Storage) | object size cap (50MB) | `connectors.MaxObjectBytes` |
| Pagination | max pages (any strategy, including GraphQL relay) | `pkg/httpclient/pagination.go` |
| Query results | row limit (1,000 default / 10,000 hard cap) | `connections.EffectiveRowLimit` |
| Every dataframe | oversized single-cell truncation | `dataframe.Frame.TruncateCells` (applied centrally in `connections.Service.Query`) |
| Every workflow node | per-node output row cap (defense against join fan-out) | `workflow.MaxRowsPerNode` |
| Workflow definitions | max nodes / max edges | `workflow.MaxNodes` / `MaxEdges` |
| Workflow execution | overall wall-clock timeout (2 min) | `workflow.MaxExecutionDuration` |
| Sessions | short-lived access tokens, rotating refresh tokens | `internal/auth` |

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
- Request bodies are size-capped (1MB, `httpx.MaxRequestBodyBytes`) and
  decoded with `DisallowUnknownFields`; an oversized body fails fast with a
  clear `413 payload_too_large` rather than a confusing parse error from a
  silently truncated read.

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
- **Kerberos ticket acquisition has no explicit timeout wrapper.** Unlike
  every other auth scheme in `pkg/httpclient`, a KDC that hangs (rather than
  erroring) during `client.Login()`/`SetSPNEGOHeader` can block that
  request beyond the connector's usual bounds - `gokrb5` (the underlying
  pure-Go Kerberos implementation) doesn't accept a context today. Operators
  using Kerberos should ensure their KDC is reliably reachable from the
  server.
- **Per-connection rate limiting is in-memory and per-instance**, the same
  caveat as the per-IP limiter above: a horizontally-scaled deployment needs
  a shared limiter to enforce a single global budget per connection.
- **Cloud connector least-privilege is the operator's responsibility, not
  enforced here.** Same principle as "use a read-only DB role" for
  Postgres/MySQL: the IAM role/service account/service principal behind an
  `aws`/`gcp`/`azure` connection should be scoped to exactly the
  read-only actions it needs (e.g. `athena:GetQueryResults` +
  `s3:GetObject` on the results bucket, not a broad `s3:*`). This project
  has no way to inspect or constrain what a cloud credential can actually
  do once it authenticates.
- **Cloud connectors can't be exercised end-to-end in CI** the way the SQL
  and REST/GraphQL connectors can (no local Postgres/httptest-server
  equivalent for Athena/BigQuery/Log Analytics) - their config validation
  and credential-selection logic is unit tested, but the actual SDK calls
  are only verified by hand against real cloud accounts.
