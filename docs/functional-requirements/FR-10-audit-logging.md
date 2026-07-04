# FR-10 — Audit Logging & Compliance

## Overview

Data Explorer maintains an **append-only audit log** of every
security-sensitive action taken in the system: authentication events,
role changes, connection lifecycle, workflow lifecycle, query
executions, exports, and administrative operations. The log is designed
to answer three questions with a single query:

1. **Who** did **what**, **when**, from **which IP**, and on **which
   resource**?
2. Did any action **succeed or fail**, and if it failed, **why**?
3. Are there **patterns** — repeated failures, suspicious IPs,
   permission escalations — worth investigating?

The audit log is **not** a metrics store or an application log; it is a
compliance-grade record of principal-scoped events. It is written
inline with the action (never fire-and-forget), retained per policy,
and queryable via the API and a purpose-built UI.

## Product goals

- Produce a **single, authoritative record** of every action a person
  or a scheduler has taken. Nothing security-sensitive should have to
  be reconstructed from web-server logs.
- Make audit queries **fast** and **filterable** so incident response
  isn't waiting on grep.
- Prevent audit-log tampering: entries are **append-only** and never
  edited or deleted from within the product.
- Keep audit payloads **safe** to store: no plaintext secrets, no row
  data, no PII beyond what identifies the acting principal and the
  resource.
- Support both **manual read** by a human admin and **automated export**
  to a SIEM.

## User personas

| Persona                | Description                                                                                             |
| ---------------------- | ------------------------------------------------------------------------------------------------------- |
| **Admin**              | Investigates who changed what.                                                                          |
| **Compliance officer** | Signs off on quarterly reviews; needs to show controls exist and produce evidence when auditors ask.    |
| **Auditor** (external) | Requests point-in-time evidence of user provisioning, access grants, and data exports.                  |
| **Security engineer**  | Correlates suspicious activity in the audit log with the wider security signal stream.                  |
| **Ops engineer**       | Uses the audit log to reconstruct the timeline around an incident.                                      |

## User stories

- **US-10.1** As an admin, I want to see every action taken in the
  product with actor, resource, timestamp, and outcome, so I can
  investigate incidents.
- **US-10.2** As a compliance officer, I want to filter the audit log
  by action type, actor, date range, and resource, so I can produce
  targeted evidence for auditors.
- **US-10.3** As an admin, I want to be *unable* to delete an audit
  entry from within the product, so tampering isn't a click away.
- **US-10.4** As a security engineer, I want assurance that no
  plaintext secret or row of data appears in the audit log, so the
  log itself isn't a data-leak liability.
- **US-10.5** As an auditor, I want the entries to include the client
  IP address (from a trusted forwarded-for header) so I can identify
  the network the action came from.
- **US-10.6** As an admin, I want to page through millions of
  entries at 30 entries per page without timing out, so the UI stays
  usable at scale.
- **US-10.7** As an admin, I want to see failed authentication
  attempts distinguished from succeeded ones, so I can spot brute
  force.
- **US-10.8** As a compliance officer, I want a bounded retention
  policy documented and enforced, so we can prove we retain audit
  records for the required period.

## Functional requirements

### FR-10.1 — Event catalogue

The system SHALL emit an audit event for each of the following
action types. This list is authoritative; changes require an FRD
update.

**Authentication and session**

- `auth.registered` — new user account created (via first-user
  bootstrap or admin-created).
- `auth.login_succeeded` — successful password login.
- `auth.login_failed` — failed password login (bad password,
  unknown user, suspended account).
- `auth.refresh_succeeded` — refresh token exchanged for a new
  access token.
- `auth.refresh_failed` — refresh token invalid, revoked, or
  expired.
- `auth.logout` — refresh token revoked by user action.
- `auth.user_suspended` — admin marked a user as suspended.
- `auth.user_reinstated` — admin re-enabled a suspended user.

**RBAC**

- `rbac.role_created`, `rbac.role_updated`, `rbac.role_deleted`,
  `rbac.role_assigned`, `rbac.role_unassigned`.

**Connections**

- `connections.created`, `connections.updated`, `connections.deleted`,
  `connections.health_checked`, `connections.explore`.

**Workflows**

- `workflows.created`, `workflows.updated`, `workflows.deleted`,
  `workflows.duplicated`, `workflows.run_started`,
  `workflows.run_completed`, `workflows.scheduled_dispatch`,
  `workflows.scheduled_skipped`, `workflows.export`,
  `workflows.output`.

**Audit read**

- `audit.read` — a user opened / paged / exported the audit log
  itself (meta-audit).

### FR-10.2 — Common event fields

Every audit event SHALL include:

- `id` — UUID.
- `occurred_at` — server timestamp (UTC, microsecond precision).
- `action` — one of the event names above.
- `actor_type` — `user`, `scheduler`, or `system`.
- `actor_id` — user id (nullable when `actor_type != user`).
- `actor_email` — snapshotted at emission time, kept even if the
  user is later deleted.
- `resource_type` — e.g. `connection`, `workflow`, `role`, `user`.
- `resource_id` — the id of the resource (nullable for
  system-level events).
- `resource_name` — snapshot of the resource's name at emission
  time.
- `outcome` — `succeeded`, `failed`.
- `error_code` — from the FR-05 error taxonomy, when `outcome ==
  failed`.
- `client_ip` — from the trusted forwarded-for header.
- `user_agent` — from the request header.
- `request_id` — the request-id middleware value tying the entry
  back to structured logs.
- `metadata` — a bounded JSON blob for event-specific fields (e.g.
  `from_role`, `to_role` on a role change).

### FR-10.3 — Append-only guarantee

Audit rows SHALL be inserted only. There SHALL NOT be an UPDATE or
DELETE path exposed by the audit service or the API. Direct
database access is out of scope of the product's controls but is
documented in [`../SECURITY.md`](../SECURITY.md).

### FR-10.4 — Payload constraints

Audit `metadata` SHALL NOT contain:

- Plaintext credentials, tokens, API keys, or refresh tokens.
- Row data returned by any query.
- The raw text of a query (a query hash MAY be recorded).
- Any secret decrypted from the connections vault.

This constraint SHALL be enforced by the audit service layer, not
just by convention — the service SHALL validate that no field with
one of the reserved keys (`password`, `token`, `secret`,
`api_key`, `key`, `credential`, `authorization`) appears in the
metadata payload.

### FR-10.5 — Retention

Audit entries SHALL be retained indefinitely by default. A future
retention-window setting MAY prune entries older than a configured
duration, but the default configuration retains all entries.

### FR-10.6 — Access control

- `audit:read` gates listing and filtering the audit log.
- Only the built-in `admin` role has `audit:read` by default.
- A user's own `audit.read` action against the log SHALL itself
  emit an `audit.read` audit event (meta-audit) to guard against
  silent scraping.

### FR-10.7 — Query API

The API SHALL expose `GET /api/v1/audit-logs` with query parameters:

- `action` (single or multi-valued).
- `actor_id`.
- `resource_type`.
- `resource_id`.
- `from`, `to` (ISO 8601).
- `outcome`.
- `page`, `page_size` (page size ≤ 100, default 30).

Results SHALL be sorted by `occurred_at DESC` and SHALL include a
`total` and `next_page` cursor.

### FR-10.8 — UI

The Audit Log page SHALL:

- Show a paginated table with columns: Time, Actor, Action,
  Resource, Outcome, IP.
- Provide filters matching the API's parameters, applied client-side
  where possible and server-side for time / paging.
- Show a details drawer / modal with all fields including
  `metadata` and `request_id` — see
  [`docs/screenshots/10-audit-log.png`](../screenshots/10-audit-log.png).

### FR-10.9 — Correlated request id

Every audit entry SHALL share a `request_id` with the corresponding
structured log line(s) and Prometheus histogram observation (see
FR-11), so an entry can be fully reconstructed from the log
pipeline.

### FR-10.10 — Time source

`occurred_at` SHALL be set by the server's monotonic time source on
event creation. Clients cannot influence `occurred_at`.

## UI/UX requirements

- The Audit Log page is accessible only to users with `audit:read`
  (route-guarded and menu-hidden otherwise).
- Filters at the top of the table: Action multi-select, Actor
  free-text, Resource type dropdown, Outcome dropdown, Date range
  picker.
- Failed events are visually distinguished by a red status dot;
  successful events by a green dot; system/scheduler events by a
  neutral dot with a "system" chip.
- The Actor column shows the email (snapshotted) with an "unknown
  user" fallback if the actor was later deleted; hovering shows the
  actor id.
- The Details drawer shows JSON `metadata` in a readable pretty-print
  and includes a "Copy JSON" button.
- Empty state ("no matching entries") is neutral; large filter
  results paginate.
- The URL query string mirrors the filter state so a filtered view
  is linkable.

## Acceptance criteria

- [ ] Registering a user emits exactly one `auth.registered` audit
  event with `actor_type = user`, `resource_type = user`,
  `resource_id = <new user id>`, `outcome = succeeded`.
- [ ] Three failed password attempts followed by a successful login
  produce three `auth.login_failed` + one `auth.login_succeeded`
  event.
- [ ] Creating, updating, and deleting a connection produce three
  audit events with distinct action names.
- [ ] Running a scheduled workflow produces
  `workflows.scheduled_dispatch` + `workflows.run_started` +
  `workflows.run_completed` events with `actor_type = scheduler`.
- [ ] Exporting a CSV from Explore produces one `explore.export`
  event with `row_count` in metadata and no row data.
- [ ] A non-admin user attempting `GET /api/v1/audit-logs` receives
  403.
- [ ] An admin user browsing the audit log emits an `audit.read`
  audit event.
- [ ] The `metadata` field never contains a decrypted secret;
  service-layer test cases assert this.
- [ ] The audit table paginates at 30 rows by default; page size can
  be increased to 100.
- [ ] Every audit entry has a non-empty `request_id` matching a
  structured-log entry from the same request.
- [ ] Deleting a user via admin action does not delete their past
  audit entries.

## Edge cases & error handling

- **Actor deleted after event**: The audit entry retains the actor's
  snapshotted email; UI shows the email with a "(deleted user)"
  tag.
- **Resource deleted after event**: The audit entry retains the
  snapshotted `resource_name`; UI shows "(deleted)" next to the
  name.
- **Very large metadata**: Metadata is capped at a service-defined
  size (default 4KB). Oversized payloads truncate a specific
  metadata field to `"<truncated>"` and log a service warning.
- **System principal**: For scheduler-initiated events,
  `actor_type = scheduler`, `actor_id = null`, `actor_email = null`,
  and a `scheduler_id` may be recorded in metadata.
- **High cardinality actions**: When a single request emits many
  audit events (e.g. a bulk role update), each individual change
  emits a distinct event so the log stays granular.
- **Clock skew**: Server timestamps are monotonic per process;
  entries from different processes MAY interleave.
- **Failed emit**: If the audit insert fails, the outer action
  SHALL fail as well — audit is not fire-and-forget. This is a
  hard invariant.
- **Timezone display**: Times are stored in UTC and displayed in
  the user's local timezone in the UI with UTC in the tooltip.

## Non-functional requirements

- **Availability**: Audit inserts SHALL succeed as long as the
  database is reachable. A DB outage takes down the mutating
  actions themselves, not just the audit trail — the two are
  intentionally coupled.
- **Performance**: Audit inserts SHALL add ≤ 10ms to the encompassing
  action at p95.
- **Query performance**: Filtered queries covering ≤ 1 million rows
  SHALL return in ≤ 500ms at p95 using the standard index set on
  `(occurred_at, action)`, `(actor_id, occurred_at)`,
  `(resource_type, resource_id, occurred_at)`.
- **Storage**: Audit table SHALL support tens of millions of rows
  without schema change. A future partitioning strategy is a
  possible enhancement, not a requirement.
- **Security**: Audit rows SHALL NOT be exposed to any user without
  `audit:read`. Bulk export requires the same permission.
- **Integrity**: The `id` UUID SHALL be generated server-side; the
  API never accepts a caller-provided id.

## Market context & differentiation

| Product           | Audit surface                                                | Notes                                                                     |
| ----------------- | ------------------------------------------------------------ | ------------------------------------------------------------------------- |
| **Retool**        | Audit log for actions in the app                             | SaaS-focused; enterprise-tier for deep filtering.                         |
| **Metabase**      | Enterprise-only audit tables                                 | Requires the paid tier.                                                   |
| **Grafana**       | Basic audit via server log; enterprise audit trail add-on    | Requires enterprise for structured audit.                                 |
| **Airflow**       | Task instance history + Flask log                            | Not a first-class audit surface; users glue in Kafka / Splunk.            |
| **n8n**           | Execution history + audit events (Enterprise)                | SaaS/enterprise gate.                                                     |
| **Superset**      | User action log                                              | Limited action set.                                                       |
| **Postman**       | Team activity feed                                           | Team plan; not per-request granular.                                      |
| **Enterprise SSO tools** | Deep audit / SIEM integrations                        | Purpose-built for identity events; not for data-tool actions.             |

Data Explorer's differentiators for audit:

- **Audit is included in the free / self-hosted product from day
  one.** No enterprise tier gate.
- **Structured event catalogue.** Every mutating action has a
  canonical action name; no reverse-engineering from a free-form
  message.
- **Fail-closed inserts.** Audit inserts and the encompassing action
  succeed or fail together — no silent audit gaps.
- **Payload safety enforced at the service layer.** Reserved-key
  scanning prevents credentials or row data from leaking into audit
  metadata.
- **Request-id correlation.** Audit rows, structured logs, and
  Prometheus histograms share a `request_id` so an event can be
  reconstructed end-to-end.
- **Meta-audit of audit reads.** Even reading the audit log is a
  logged event, discouraging silent scraping.

## Future enhancements (out of scope)

- SIEM integration (streaming push to Splunk / Elastic /
  CloudWatch / Sumo).
- Signed-hash chaining for tamper-evident append-only enforcement
  visible to auditors.
- Configurable retention policy with automatic pruning.
- Audit-event webhook subscriptions.
- Field-level redaction for sensitive metadata across all events.
- Compliance report templates (SOC 2 CC7 evidence generation).
- Audit alerting rules ("email me when a role is deleted").
- Time-based audit archival to cold storage.
- Cross-instance federated audit (aggregate multiple deployments).

## Cross-references

- [FR-01 Authentication & Session Management](./FR-01-authentication-and-sessions.md)
- [FR-02 Role-Based Access Control (RBAC)](./FR-02-role-based-access-control.md)
- [FR-03 Data Source Connection Management](./FR-03-connection-management.md)
- [FR-07 Visual Workflow Builder](./FR-07-visual-workflow-builder.md)
- [FR-08 Workflow Scheduling & Automation](./FR-08-workflow-scheduling.md)
- [FR-11 Observability, Guardrails & Reliability](./FR-11-observability-and-guardrails.md)
- [`../SECURITY.md`](../SECURITY.md)
