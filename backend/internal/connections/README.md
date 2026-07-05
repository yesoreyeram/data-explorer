# internal/connections

## What this package does

`internal/connections` is the **central package for data source connection management**. It owns:

- CRUD for `Connection` rows (name, type, non-secret config, encrypted secret)
- Secret encryption/decryption via `platform/crypto` (the **only** code path that decrypts a secret)
- Connector registry and dispatch
- Per-connection rate limiting
- Health check invocation and error classification
- Query execution (persisted connections and ad-hoc/temporary connections)

## Key types

### Connector interface

```go
type Connector interface {
    Test(ctx context.Context, config json.RawMessage, secret map[string]string) error
    Execute(ctx context.Context, spec QuerySpec, config json.RawMessage, secret map[string]string) (*dataframe.Frame, error)
}
```

Every connector (Postgres, MySQL, REST, GraphQL, AWS, GCP, Azure) implements this interface. The registry maps `domain.ConnectionType` → `Connector`. Adding a new source type means implementing this interface and registering it in `cmd/server/main.go`.

### QuerySpec

```go
type QuerySpec struct {
    SQL    string           // for postgres/mysql
    REST   *RESTQuerySpec   // for rest
    GraphQL *GraphQLQuerySpec
    Cloud  *CloudQuerySpec  // for aws/gcp/azure
}
```

One spec type covers all connector variants; connectors only read the field relevant to their type.

### HealthError / Classify

`Classify(err)` converts raw driver/SDK errors into a structured `HealthError`:

```go
type HealthError struct {
    Code         string  // stable: "timeout", "auth_failed", "permission_denied", etc.
    Message      string  // plain language for the UI
    Remediation  string  // concrete next step
    Detail       string  // original error, for logs
}
```

Classification handles:
- `*pgconn.PgError` SQLSTATE codes
- `*mysql.MySQLError` numbers
- `smithy.APIError` (AWS SDK)
- `*azcore.ResponseError` (Azure SDK)
- `*googleapi.Error` (GCP SDK)
- `net.Error` (timeout, DNS, connection-refused)
- Substring fallback for edge cases

Applied in exactly one place — `Service.Test/Query/QueryAdhoc` — not in each connector.

### Rate limiter

Per-connection, sliding-window rate limiter. Each `Query` call checks the connection's configured rate limit before executing. Protects upstream systems from being hammered by a misbehaving workflow.

## Service: the encryption boundary

`connections.Service` is the **only** code that decrypts secrets. The flow:

1. `Service.Query(ctx, connectionID, spec)` loads the `Connection` row.
2. Calls `crypto.Decrypt(key, conn.SecretEncrypted)` in-memory.
3. Passes the decrypted `map[string]string` to the connector's `Execute` call.
4. The decrypted map is discarded after the call; it is never stored, never returned.

Secrets are never present in API responses, log lines, or error messages.

## Architecture decisions (ADRs)

| Decision | Rationale |
|---|---|
| `Service` is the sole decryption path | A single choke point makes audit and review tractable; no connector can accidentally log a secret |
| `Classify` applied at service layer, not per-connector | Every connector's errors are normalized before reaching the API layer; adding a new connector doesn't require writing classification logic |
| Per-connection rate limit | Protects upstreams from individual misbehaving pipelines without a global bottleneck |
| Ad-hoc `QueryAdhoc` with inline credentials | Never touches the database; credentials are used once and discarded — zero persistence risk for temporary connections |

## Scope and responsibilities

- CRUD for `Connection` entities.
- Encrypt secrets on write; decrypt in-memory for use.
- Register and dispatch connectors.
- Test connections and persist health status.
- Execute queries against saved or ad-hoc connections.
- Rate-limit per connection.
- Classify all connector errors into `HealthError`.

## Limitations and todos

- [ ] Rate limiter state is in-process; multi-replica deployments share no state (each instance has its own window).
- [ ] No connection pooling across requests for HTTP-based connectors (REST/GraphQL each open a new client per call).
- [ ] `secret_encrypted` rotation requires a manual script; no automatic key-rotation tooling.
- [ ] Connection sharing / visibility (e.g. private vs team-shared connections) is not yet modeled.
