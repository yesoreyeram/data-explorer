# internal/quota

## What this package does

`internal/quota` provides a **per-user, per-role, sliding-window hourly quota service** for two operation kinds: ad-hoc explore runs (`KindExplore`) and workflow runs (`KindWorkflow`). It prevents a single user from monopolizing server resources within a rolling one-hour window.

## How it works

```go
result := quotaService.Check(userID, roles, quota.KindExplore, time.Now())
if !result.Allowed {
    // 429 response with Retry-After = result.RetryAfter
}
```

- **Sliding window**: keeps a timestamp list per `(kind, userID)` key; evicts entries older than one hour on every `Check` call.
- **Quota from config**: resolved from `GuardrailsConfig.RoleQuotas` by taking the **maximum** across all the user's roles (best-privilege wins).
- **No quota configured** (`quota <= 0`) or **no user ID** (anonymous) → always allowed.
- **State**: in-process `sync.Map` — fast, no external dependency, but not shared across replicas.

## Result struct

```go
type Result struct {
    Allowed    bool
    Quota      int           // configured limit
    Used       int           // current count in the window
    Window     time.Duration // always 1 hour
    RetryAfter time.Duration // how long until a slot opens (when !Allowed)
}
```

## Architecture decisions (ADRs)

| Decision | Rationale |
|---|---|
| In-process `sync.Map` | Zero external dependency; sufficient for single-replica deployments |
| Best-privilege role resolution | A user with multiple roles gets the highest quota of any of their roles |
| Sliding window (not fixed bucket) | Fairer: a burst at the top of the hour doesn't block all use for the rest of the hour |
| Separate from the per-connection rate limiter | Connection rate limits protect upstreams; quota protects server-side resources per user |

## Scope and responsibilities

- Check and record per-user operation counts.
- Resolve quotas from role configuration.
- Return a `Result` indicating whether the operation is allowed and when to retry if not.

## Limitations and todos

- [ ] In-process state is lost on restart and is not shared across replicas.
- [ ] No quota for custom roles without explicit `RoleQuotas` configuration — they default to "no quota" (unlimited).
- [ ] No real-time quota visibility in the UI (users cannot see how many runs they have left today).
- [ ] `RetryAfter` is an approximation; the actual window is a sliding list, so the earliest available slot is `oldest_hit_in_window + 1 hour`.
