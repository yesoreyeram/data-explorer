# pkg/httpclient

## What this package does

`pkg/httpclient` is a **standalone, guardrailed, pluggably-authenticated HTTP client** used by the REST and GraphQL connectors. It handles every authentication scheme those APIs actually require, plus five pagination strategies, plus safety guardrails (response size cap, redirect cap, bounded retry with full jitter). Like `pkg/dataframe`, it has **zero imports from this module's `internal/*` packages** and can be used outside this application.

## Authentication schemes

| Scheme | Implementation | Notes |
|---|---|---|
| Basic | Static `Authorization: Basic …` header | Username + password from encrypted secret |
| ****** Static `Authorization: ****** header | Token from encrypted secret |
| API key | Header or query-parameter injection | Key name configurable |
| Digest (RFC 7616) | `RoundTripperAuthenticator` challenge-response | Sends unauthenticated first; reads `WWW-Authenticate`; resends with computed response — the only scheme that must see a response before it can authenticate |
| OAuth2 client credentials | `golang.org/x/oauth2`'s `TokenSource` | Caches and auto-refreshes; token URL + client ID/secret from encrypted secret |
| OAuth2 refresh token | `golang.org/x/oauth2`'s `TokenSource` | Refresh token from encrypted secret |
| Self-signed JWT | Mints and caches short-lived HS256/RS256 JWT | Re-signs shortly before expiry; signing key from encrypted secret |
| Workload identity federation | RFC 8693 Token Exchange | Standards-based mechanism underlying AWS/GCP/Azure workload identity; not tied to one SDK |
| Kerberos / SPNEGO | `github.com/jcmturner/gokrb5` | Pure Go, no cgo or system Kerberos library needed |

Credentials come exclusively from the connection's encrypted secret map. The mapping from `AuthType` to secret keys lives in `internal/connections/connectors/httpauth.go`.

## Pagination strategies

| Strategy | When to use |
|---|---|
| `OffsetLimitPaginator` | `?offset=N&limit=M` style APIs |
| `PagePaginator` | `?page=N` style APIs |
| `CursorPaginator` | Cursor embedded in response body (JSON path configurable) |
| `LinkHeaderPaginator` | RFC 8288 `Link: <url>; rel="next"` header |
| `GraphQLRelayPaginator` | Relay Cursor Connections: `edges { node }` + `pageInfo { hasNextPage, endCursor }` |

`Client.DoPaginated` owns the `MaxPages` guardrail so every strategy gets it automatically. Default maximum: 20 pages; hard ceiling: 500 pages. A misconfigured "next page" field that never terminates still stops.

## Safety guardrails

| Guardrail | Default | Description |
|---|---|---|
| Response size cap | 25 MB | Rejects responses larger than `DefaultMaxResponseBytes` |
| Redirect cap | 5 | Stops redirect chains that loop |
| Retry with exponential backoff + full jitter | 3 attempts | Retries on 429/502/503/504; full jitter prevents retry thundering herds |

## Key interfaces

```go
type Authenticator interface {
    ApplyAuth(req *http.Request) error
}

type RoundTripperAuthenticator interface {
    Authenticator
    // Digest must see the 401 response before it can sign the re-request
    HandleResponse(resp *http.Response, req *http.Request) (*http.Request, error)
}

type Paginator interface {
    NextPage(resp *http.Response, body []byte) (*http.Request, bool, error)
}
```

## Architecture decisions (ADRs)

| Decision | Rationale |
|---|---|
| Standalone package, no `internal/*` imports | Reusable outside this app; no circular import risk |
| `RoundTripperAuthenticator` for Digest | Digest requires a challenge-response cycle; a simple `ApplyAuth` isn't sufficient; modelling as a round-tripper extension is the clean separation |
| `MaxPages` owned by `DoPaginated` | Every pagination strategy gets the guardrail for free — a new paginator cannot bypass it |
| Full jitter on retry | Fixed-interval retries cause all failing clients to retry simultaneously (thundering herd); full jitter distributes load |
| `golang.org/x/oauth2` for OAuth2 | Mature, well-tested token-source abstraction with built-in caching and refresh |

## Limitations and todos

- [ ] Kerberos ticket acquisition lacks a context timeout — a slow or unreachable KDC can block a goroutine indefinitely.
- [ ] No HTTP/2 push support.
- [ ] Workload identity federation coverage is generic (RFC 8693) but is not exercised by every cloud provider in tests.
- [ ] `GraphQLRelayPaginator` only supports the `edges { node }` / `pageInfo` shape; non-Relay cursor conventions require `CursorPaginator` plus manual configuration.
- [ ] No streaming response support; large paginated results accumulate in memory across pages.
