# FR-05: Connection Health Monitoring

## Overview

Every connection can be **health-checked** on demand. A health check
dials the underlying data source with the connection's credentials,
records whether the connection succeeded, times how long it took, and —
critically — classifies any failure into a stable error taxonomy with a
plain-language remediation. This transforms a raw driver error like
`pq: password authentication failed for user "svc"` into a
user-facing pattern of **Error code → Message → Next step** that a
non-DBA can actually act on.

## Product goals

- **Answer "is it working?" in one click.** A user seeing an unhealthy
  connection shouldn't need to open a terminal or read backend logs to
  understand why.
- **Replace opaque driver errors with actionable diagnostics.** Every
  failure produces a taxonomy code, a plain-language message, and a
  suggested next step — not a raw stack trace.
- **Keep history.** Users can see recent checks for a connection to
  spot flapping vs. steady failures.
- **Preserve raw error detail underneath for debugging.** The
  taxonomy layers *over* the raw error, it doesn't replace it — the
  original error is still available in structured logs and audit
  metadata for engineers who need it.
- **Cost-bounded and safe.** A health check has the same guardrails as
  any other query (row limits, timeouts, no writes).

## User personas

| Persona          | Description                                                                                                             |
| ---------------- | ----------------------------------------------------------------------------------------------------------------------- |
| Editor           | Runs a health check after creating or editing a connection to confirm it works.                                         |
| Analyst          | Sees a red status on a connection and wants to understand why before deciding whether to work around it.                |
| Ops / SRE        | Diagnoses recurring failures (a rate-limited API, a database that's briefly unreachable) and decides whether to alert. |
| Admin            | Reviews the last-error patterns across all connections during a period of downstream outages.                          |

## User stories

- **US-05.1** As an editor who just saved a new connection, I want a
  single click to test it end-to-end so I know the credentials I
  entered work before I ask anyone to use it.
- **US-05.2** As an analyst, when I see an unhealthy connection I
  want to understand at a glance whether it's a credential problem,
  a rate-limit problem, a network problem, or a permission problem —
  so I know whether to try again, wait, or escalate.
- **US-05.3** As an SRE, I want the health error to include a
  concrete next step ("**Wait ~60 seconds and retry**", "**Check the
  IAM policy attached to the role**") so I don't have to interpret a
  cryptic driver code.
- **US-05.4** As an SRE, I want to see the recent history of health
  checks for one connection so I can distinguish a flapping outage
  from a hard failure.
- **US-05.5** As a user without `connections:test` permission, I
  want the health status displayed to me so I can see the connection
  is unhealthy, even if I can't be the one to re-test it.
- **US-05.6** As a developer, I want the classification to work
  regardless of connector (Postgres / MySQL / REST / GraphQL / AWS /
  GCP / Azure) so a red badge means the same thing everywhere.

## Functional requirements

### FR-05.1 — Health check endpoint

The system SHALL expose `POST /api/v1/connections/{id}/test` that:

- Requires the `connections:test` permission.
- Decrypts the connection's secret in memory only.
- Calls the connector's `Test` method with a bounded timeout.
- Times the call.
- Persists the outcome on the connection row: `status` (`healthy` /
  `unhealthy`), `last_tested_at`, `last_error`, `last_error_code`,
  `last_error_remediation`, `last_check_duration_ms`.
- Emits an `audit_logs` entry with `action = 'connection.test'`,
  `outcome = 'success' | 'failure'`, and metadata carrying the
  duration and (on failure) the error code and remediation.
- Returns a JSON body containing `{ status, lastTestedAt, error?,
  errorCode?, errorRemediation?, durationMs }`.

The endpoint SHALL be safe to call repeatedly and SHALL NOT queue
requests server-side — a second concurrent call from the same user
against the same connection is either allowed (concurrent test) or
rate-limited via `connections.Service`'s per-connection limiter.

### FR-05.2 — Error taxonomy

Every failure of a `Test` (or a `Query`, see [FR-06 &
FR-07](./FR-06-ad-hoc-exploration.md)) SHALL be classified into
exactly one of the following stable **error codes**:

| Code                  | When it applies                                                                                         | Suggested remediation                                                                                         |
| --------------------- | ------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------- |
| `timeout`             | The upstream did not respond within the deadline.                                                       | Increase the connection timeout, check upstream latency, or retry later.                                     |
| `network_unreachable` | DNS failed, TCP connection refused, or the host is unreachable.                                        | Confirm the host / port is correct, and that this server can reach it (firewall, VPN).                       |
| `auth_failed`         | Credentials were rejected by the upstream.                                                              | Confirm the password / token / secret is current, and that the account isn't locked.                          |
| `permission_denied`   | Authenticated, but the account is not authorized to do what the connector attempted.                    | Grant the account the required read permission on the target resource.                                        |
| `not_found`           | The referenced database / table / bucket / API path does not exist.                                    | Confirm the identifier is spelled correctly and exists in the target system.                                  |
| `rate_limited`        | The upstream returned `429 Too Many Requests` or an equivalent SDK error.                              | Wait and retry with a longer interval, or ask the upstream owner to raise the quota.                          |
| `invalid_config`      | The connection configuration itself is invalid (e.g. bad SSL mode, malformed URL, missing region).      | Review the connection form for the flagged field and correct it.                                              |
| `unknown`             | Everything else.                                                                                        | Copy the raw error into an issue report; check the audit log's metadata for driver-specific detail.          |

The classifier SHALL unwrap typed errors from every driver / SDK
already in `go.mod` (`*pgconn.PgError`, `*mysql.MySQLError`,
`smithy.APIError` for AWS, `*azcore.ResponseError`, `*googleapi.Error`,
`net.Error` for generic timeouts / DNS / connection-refused) before
falling back to substring matching on well-known message shapes. The
raw error SHALL remain reachable via `errors.Is` / `errors.As` for
callers who need it.

### FR-05.3 — Error message and remediation

For every classified error, the API SHALL return:

- **`errorCode`**: a stable machine-readable string from FR-05.2.
- **`error`** (a.k.a. `message`): a human-readable, non-driver-specific
  sentence written for a user, not a developer. For example:
  - `auth_failed` for Postgres → **"The database rejected the username
    or password."**
  - `rate_limited` for a REST connection → **"The upstream API returned
    429 (too many requests) for this call."**
  - `not_found` for Athena → **"Athena reported that the referenced
    database or table does not exist."**
- **`errorRemediation`**: the concrete next step, matched to the code
  (see FR-05.2). This SHALL be at most one short sentence.

Neither the message nor the remediation SHALL contain the connection's
credentials or the attempted password / token in any form.

### FR-05.4 — Central classification, single application point

`connections.Classify` SHALL be the **only** place that maps a raw
error to a `HealthError`. Individual connector implementations SHALL
NOT reimplement the classification, so behavior is uniform across
connector types. Classification SHALL be applied in exactly three
call sites (`Service.Test`, `Service.Query`, `Service.QueryAdhoc`) —
the choke points every connector's error already flows through.

### FR-05.5 — Persisted state

The `connections` table SHALL carry:

- `status` (`unverified` for a never-tested connection, `healthy`,
  `unhealthy`).
- `last_tested_at` (nullable).
- `last_error` (string; empty when healthy).
- `last_error_code` (nullable string from FR-05.2).
- `last_error_remediation` (nullable string).
- `last_check_duration_ms` (nullable bigint).

The state SHALL be updated on **every** call to `Test` (regardless of
outcome) and on **every** connection edit (reset to `unverified` — a
saved configuration change invalidates the previous verification).

### FR-05.6 — Recent-checks history

The system SHALL provide a per-connection "recent checks" view derived
from the audit log:

- Query: `GET /api/v1/audit-logs?resourceType=connection&resourceId=<id>
  &action=connection.test&limit=10&order=desc`.
- The connection health modal displays this list.
- No separate `connection_health_history` table is required —
  `audit_logs` is the source of truth (see
  [FR-10](./FR-10-audit-logging.md)).

## UI/UX requirements

### Status column on the Connections list

Reference screenshot:
[`docs/screenshots/28-connections-unhealthy-row.png`](../screenshots/28-connections-unhealthy-row.png).

- Status badge tones:
  - `healthy` → success (green dot)
  - `unhealthy` → danger (red dot)
  - `unverified` → neutral (gray dot)
- The badge SHALL be **clickable** and open the health modal for that
  row.
- Below the badge on an unhealthy row: the short `lastError` in
  subdued text, truncated to a single line.

### Health modal

Reference screenshot:
[`docs/screenshots/29-connection-health-panel.png`](../screenshots/29-connection-health-panel.png) and
[`docs/screenshots/30-connection-health-panel-recheck.png`](../screenshots/30-connection-health-panel-recheck.png).

- Title: **"Connection health — `<name>`"**.
- Top card:
  - Live status badge with the connection's current `status`.
  - **"Last checked"** timestamp (or "never").
  - Duration ("**took 1.23s**") when a recent check exists.
- Primary action button: **"Run health check"** (gated by
  `connections:test`). Label toggles to **"Running…"** while a check
  is in-flight.
- If the current status is `unhealthy`, an error card shows:
  - An **error-code badge** (`Timeout`, `Network unreachable`,
    `Authentication failed`, `Permission denied`, `Not found`, `Rate
    limited`, `Invalid configuration`, `Unknown error`).
  - The **message** as the headline.
  - The **remediation** as a **"Next step"** callout.
- Below the error card: a **"Recent checks"** table with columns
  **Time**, **Outcome** (`healthy` / `unhealthy` badges), and
  **Detail** (short error text or blank).
- Empty state for the history: **"No health checks recorded yet."**
- Modal dismissible with **Escape**, **X**, or clicking outside.

### Behavior after a re-check

- On success, the modal SHALL immediately update the badge to
  `healthy`, the message and remediation SHALL disappear, and the
  history SHALL gain a new row at the top.
- On failure, the modal SHALL update the badge to `unhealthy` and the
  new error's code, message, and remediation SHALL replace the
  previous ones.
- The Connections list SHALL update in the background (same TanStack
  Query cache) so closing the modal shows the fresh status.

## Acceptance criteria

- [ ] Testing a working connection succeeds and updates `status` to
  `healthy` with a non-null `last_tested_at`.
- [ ] Testing a connection whose password is wrong sets `status =
  'unhealthy'`, `last_error_code = 'auth_failed'`, and a remediation
  matching FR-05.2.
- [ ] Testing a connection whose host is unreachable sets
  `last_error_code = 'network_unreachable'` and a corresponding
  remediation.
- [ ] Testing a REST connection where the upstream returns `429`
  classifies as `rate_limited`.
- [ ] Testing an AWS connection with an SDK error whose `smithy.APIError
  .ErrorFault()` indicates permission denied classifies as
  `permission_denied`.
- [ ] Testing a Postgres connection whose SSL mode is misspelled in
  `config` fails at validation as `invalid_config`, without contacting
  the database.
- [ ] The message returned for any authentication failure does not
  contain any part of the attempted password, token, or key.
- [ ] Every health check produces exactly one `audit_logs` row.
- [ ] The recent-checks list scopes correctly to the requested
  connection ID via `audit-logs` filtering.
- [ ] A user without `connections:test` cannot trigger a health check
  from the UI (button hidden) or the API (`403`).
- [ ] Saving an edit to a connection resets its `status` to
  `unverified` and clears `last_error`.
- [ ] Concurrent health checks against the same connection do not
  corrupt the persisted state (the last-completed one wins).

## Edge cases & error handling

- **Very slow response.** The connector's own timeout SHALL end the
  attempt at or before the connection's configured deadline.
  `Classify` maps this to `timeout` with the remediation "**Increase
  the connection timeout, check upstream latency, or retry later.**"
- **Connection deleted while the health modal is open.** The next call
  to `Run health check` SHALL surface a clear "**Connection no longer
  exists.**" error and close the modal.
- **Connection edited by another user while the health modal is
  open.** The modal SHALL rely on the shared cache and reflect the
  latest connection metadata (including the reset `unverified`
  status) on the next request.
- **Kerberos KDC hang.** Documented in [`SECURITY.md`](../SECURITY.md#known-limitations--not-yet-implemented):
  a KDC that hangs during ticket acquisition can hold the health-check
  request beyond the usual bounds because `gokrb5` does not yet accept
  a `context.Context`. Users of Kerberos SHOULD ensure their KDC is
  reliably reachable.
- **Classification miss.** If `Classify` cannot recognize a driver
  error at all, the outcome is `unknown` with a generic remediation.
  New driver error shapes SHALL be added to `healtherror.go` with a
  unit test.
- **Very frequent testing from the same user.** Per-user rate limits
  and the per-IP limiter combine to bound abuse.

## Non-functional requirements

- **Deterministic classification.** For the same underlying error, the
  classifier SHALL always return the same `errorCode`, message, and
  remediation across process restarts.
- **No credential leakage into error output.** Verified by a test that
  runs known-bad credentials against a mock upstream and asserts that
  the resulting `HealthError.Message` contains no substring of the
  credential value.
- **Bounded execution time.** A health check for a slow upstream SHALL
  return within `Connector.TestTimeout` (default 15 s, configurable).
- **Persistence latency.** The updated row SHALL be observable in the
  next `GET /api/v1/connections` within 100 ms of the test finishing.

## Market context & differentiation

| Product     | Connection health experience                                                                                             |
| ----------- | ------------------------------------------------------------------------------------------------------------------------ |
| Grafana     | Test button per data source; on failure, shows the raw driver error. No taxonomy, no remediation, no history.            |
| Metabase    | Tests connections on save; failure shows a short error, without categorical codes or history.                            |
| Retool      | Test button; failure surfaces driver text. Retool Cloud has some pattern-based helpful hints for common errors.           |
| Airbyte     | Full "check connection" step in the connector protocol; result is pass/fail with a message and sometimes a hint.         |
| n8n         | "Test" button per credential; success or a raw error, no history.                                                       |

**Where Data Explorer is intentionally different.** Health monitoring is
elevated to a first-class, uniform surface:

- **Same taxonomy across every connector.** A user learning what
  `permission_denied` means once knows what it means for Postgres, an
  API returning `403`, and an AWS IAM rejection.
- **Every failure ships with a remediation.** The user isn't left to
  interpret an SDK error code; the next step is spelled out in the UI.
- **History without a bespoke table.** Reuses the audit log as the
  source of truth, so retention and permissions are already governed.

## Future enhancements (out of scope for this FR)

- **Scheduled auto-health-checks.** Run a health check every N minutes
  in the background for every connection, so the list always reflects
  live status without user action.
- **Alerting hooks.** Emit a webhook / PagerDuty / Slack alert when a
  connection transitions from `healthy` to `unhealthy`, or vice versa.
- **Health SLO badges.** Show "**Healthy 99.3% of last 7 days**" on
  each connection based on aggregate history.
- **Per-connector deep probes.** For SQL sources, run an `EXPLAIN
  SELECT 1`; for REST, probe a configurable path (e.g. `/health`) to
  distinguish "reachable" from "authenticated" from "fully working".
- **Auto-remediation for known transient failures.** For `rate_limited`
  and `timeout`, retry with exponential backoff before marking the
  connection unhealthy.

## Cross-references

- Implementation: `backend/internal/connections/healtherror.go`,
  `backend/internal/connections/service.go`,
  `backend/internal/api/handlers/connections.go` (`TestConnection`),
  `frontend/src/pages/connections/ConnectionHealthModal.tsx`.
- Related FRs: [FR-03 Connection Management](./FR-03-connection-management.md),
  [FR-10 Audit Logging](./FR-10-audit-logging.md).
- Architecture: [`../ARCHITECTURE.md`](../ARCHITECTURE.md) section
  "Health checks and error classification".
