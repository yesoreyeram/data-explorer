# pkg/egress

## What this package does

`pkg/egress` is a **standalone SSRF (Server-Side Request Forgery) prevention guard**. Because Data Explorer dials third-party databases and APIs on a user's behalf, a malicious or misconfigured connection could point it at a cloud metadata endpoint (`169.254.169.254`), loopback, or an internal-only service. This package resolves DNS itself, validates every resulting IP against a policy, and then supplies a custom `net.Dialer` that connects only to the validated literal IP — eliminating the DNS-rebinding / TOCTOU window between the check and the dial.

**Stdlib only, zero dependencies from `internal/*`.** Can be imported and used in any Go project.

## Modes

| Mode | Behavior |
|---|---|
| `allow-private` | Permits RFC 1918/private targets (for on-prem databases) but always denies cloud metadata endpoints, loopback, and link-local. Default hardening mode. |
| `allowlist` | Permits only hosts matching a configured allowlist. Always-denied targets still apply. |
| `public-only` | Denies all private ranges in addition to the always-denied set. For deployments that only reach public APIs. |

## Always-denied targets (all modes)

- Cloud metadata endpoints: `169.254.169.254`, `fd00:ec2::254`, `metadata.google.internal`, `169.254.169.253` (Azure)
- Loopback: `127.0.0.0/8`, `::1`
- Link-local unicast: `169.254.0.0/16`, `fe80::/10`
- Unspecified: `0.0.0.0/8`, `::/128`

## Key type

```go
type Guard struct { … }

// New returns a Guard configured by cfg. An invalid Mode returns an error.
func New(cfg Config) (*Guard, error)

// DialContext is a drop-in replacement for net.Dialer.DialContext.
// It resolves host, validates every IP, then dials the validated IP directly.
func (g *Guard) DialContext(ctx context.Context, network, addr string) (net.Conn, error)
```

## Integration

The guard is wired into every outbound connection path in `cmd/server/main.go`:

- `pkg/httpclient.Client` — REST and GraphQL connectors
- `pgx` pool dial function — Postgres connector
- MySQL driver dial function — MySQL connector
- Cloud SDK transports — AWS, GCP, Azure connectors

## Architecture decisions (ADRs)

| Decision | Rationale |
|---|---|
| Resolve-then-dial (no re-resolve) | Eliminates DNS rebinding: a hostname that resolves to a safe IP at check time cannot later resolve to a dangerous one at dial time |
| Stdlib only | No dependency surface; importable anywhere; auditable in minutes |
| Mode enum (not a boolean) | `allow-private` vs `public-only` vs `allowlist` covers the real deployment spectrum without boolean-flag explosion |
| Wildcard allowlist entries (`*.example.com`) | Practical for wildcard subdomains without requiring every hostname to be enumerated |

## Limitations and todos

- [ ] IPv6 scope IDs (`%eth0`) in addresses are not currently parsed — addresses with scope IDs pass through unvalidated.
- [ ] The always-denied list is hand-curated; new cloud provider metadata addresses may need to be added as cloud providers expand.
- [ ] No CIDR-based allowlist entries; only hostname and `host:port` patterns are supported.
- [ ] No metrics or audit events emitted when a dial is denied — operators currently only see an error log.
