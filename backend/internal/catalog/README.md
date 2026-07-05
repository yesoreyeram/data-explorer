# internal/catalog

## What this package does

`internal/catalog` is a **small, static, in-memory registry of ~20 well-known API integrations** (GitHub, Stripe, Slack, Twilio, and others). Its sole purpose is to prefill a new `rest` or `graphql` connection's type, base URL/endpoint, auth type, and non-secret auth config — saving the user the "what's the base URL and auth scheme for X?" lookup. It never supplies credentials; the connection form's secret fields are always left blank.

## Design: prefilling, not proxying

| What the catalog does | What it does NOT do |
|---|---|
| Provides static entries authored by hand | Fetch from any external registry at runtime |
| Prefills `ConnectionType`, base URL, auth type | Store or handle credentials |
| Surfaces `docsUrl` as a hint for where to get an API key | Validate that the prefilled URL is reachable |
| Filters entries in-memory | Make any network request |

This is a deliberate constraint: the catalog has no external dependencies, no cache, no failure modes, and no rate limiting. Connection creation never depends on a third party being up.

## Entry structure

```go
type Entry struct {
    ID          string
    Name        string
    Description string
    LogoURL     string
    DocsURL     string
    Type        domain.ConnectionType  // "rest" or "graphql"
    BaseURL     string
    AuthType    string                 // matches httpauth.AuthType vocabulary
    AuthConfig  map[string]string      // non-secret auth config (token URL, scopes, etc.)
}
```

Entries are authored in `catalog/seed.go` in terms of `domain.ConnectionType` and the same `AuthType`/`AuthConfig` vocabulary `connectors/httpauth.go` already speaks — no translation layer.

## API surface

`GET /api/v1/catalog` — returns the full list; no `catalog:read` permission is needed (it reuses `connections:read` as a convenience layered on the connection-creation flow).

`GET /api/v1/catalog?q=github` — in-memory keyword search on `Name` and `Description`.

`GET /api/v1/catalog/{id}` — returns a single entry.

## Architecture decisions (ADRs)

| Decision | Rationale |
|---|---|
| Static seed data, no external registry | Zero runtime dependencies; catalog entries don't change frequently enough to warrant live synchronization |
| Reuses `connections:read` | The catalog is a convenience for creating connections, not a separate resource class; a dedicated permission would be over-engineering |
| `docsUrl` instead of secret fields | The catalog should never ship credentials; the user always provides their own |
| Entries expressed in `httpauth` vocabulary | No translation layer; a catalog pick maps directly onto connection form fields |

## Scope and responsibilities

- Store the static catalog in memory.
- Filter/search entries by keyword.
- Return entries by ID.
- Nothing else.

## Limitations and todos

- [ ] Catalog entries are hand-authored; adding a new integration requires a code change and redeploy.
- [ ] No versioning or change history for catalog entries.
- [ ] ~20 entries today; the list is representative, not exhaustive.
- [ ] No thumbnail/logo hosting; `LogoURL` points to external CDNs which may go stale.
- [ ] No automatic import from OpenAPI specs or Postman collections.
