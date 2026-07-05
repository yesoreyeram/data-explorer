# internal/audit

## What this package does

`internal/audit` provides an **append-only audit trail**: a structured log of who did what to which resource, from where, and whether it succeeded. The audit log is a separate signal from application logs — it is queryable through the API, persisted in the `audit_logs` database table, and is never truncated or rotated (it is the evidentiary record, not an operational log stream).

## Anatomy of an audit event

| Field | Type | Description |
|---|---|---|
| `id` | UUID | Unique event identifier |
| `actor_id` | string | User ID, or `"scheduler"` for scheduled runs, or `"system"` for bootstrap events |
| `actor_email` | string | Denormalized email for readability without a join |
| `action` | string | Dot-separated verb: `connection.create`, `workflow.execute`, `user.login`, etc. |
| `resource_type` | string | `connection`, `workflow`, `user`, `audit_log`, etc. |
| `resource_id` | string | ID of the affected resource (empty if no specific resource) |
| `outcome` | string | `success` or `failure` |
| `ip_address` | string | Caller's IP (from `X-Forwarded-For` or `RemoteAddr`) |
| `user_agent` | string | HTTP User-Agent header |
| `request_id` | string | Correlation ID tying this entry to the request's structured log line |
| `metadata` | JSONB | Action-specific context (e.g. connection type, error code, query shape) |
| `created_at` | timestamp | Event time |

## Usage pattern

```go
// In a handler, after the action completes:
h.audit.Record(ctx, audit.Entry{
    Action:       "connection.create",
    ResourceType: "connection",
    ResourceID:   conn.ID,
    Outcome:      audit.OutcomeSuccess,
    Metadata:     map[string]any{"type": conn.Type},
})
```

The handler calls `recordAudit(…)` (defined in `handlers/handlers.go`) before returning, passing both success and failure outcomes.

## Query API

`audit.Service.List(ctx, filter)` supports filtering by:
- `ActorID` — all events by a specific user
- `Action` — all events of a specific action type
- `ResourceType` / `ResourceID` — all events touching a specific resource
- `Outcome` — successes or failures only
- `Since` / `Until` — time range
- `Limit` / `Offset` — pagination

The connection health panel in the UI uses `ResourceID` filtering to scope the "recent checks" history to one connection without a second dedicated table.

## Architecture decisions (ADRs)

| Decision | Rationale |
|---|---|
| Append-only table | Evidentiary integrity; no update/delete endpoint exists for `audit_logs` |
| `actor_id` as plain string, not FK | Allows `"scheduler"` and `"system"` sentinel values without synthetic user rows |
| `request_id` on every entry | Ties the audit entry to the structured log line for the same request, enabling cross-signal investigation |
| Denormalized `actor_email` | Readable without a join; email can change after the fact, so the denormalized value is the historical record |
| `metadata` as JSONB | Per-action structured context without a column per action type |

## Scope and responsibilities

- Write audit log entries via `Service.Record`.
- Query audit log entries with filtering and pagination via `Service.List`.
- Never update or delete entries.

## Limitations and todos

- [ ] No audit log archival/export tooling (beyond the UI's paginated view and CSV export).
- [ ] `metadata` schema is undocumented per action type; adding a registry of known action shapes would improve consistency.
- [ ] No real-time streaming of audit events (webhook, SIEM integration).
- [ ] Large `metadata` blobs are not size-capped; a pathological handler could write oversized JSONB.
