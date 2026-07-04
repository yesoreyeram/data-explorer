# FR-03: Data Source Connection Management

## Overview

A **connection** is Data Explorer's centrally managed, encrypted-at-rest
handle to an external data source. Editors create and maintain
connections through a single form that adapts to the source type; every
other feature in the product (Explore, workflows, catalog, health
checks) is built on top of these connections. This FR defines the
create/read/update/delete lifecycle, the per-type configuration surface,
and the credential-handling discipline that makes it safe.

## Product goals

- **One place, many sources.** Support relational databases, HTTP APIs
  (REST/GraphQL), and the three major clouds (AWS/GCP/Azure) with the
  *same* CRUD flow and the *same* permission gate.
- **Bring your own auth.** Every real-world API uses a different auth
  scheme. Connections SHALL support the full matrix of common schemes
  without asking the user to build an integration.
- **Credentials never leak.** Secrets are AES-256-GCM encrypted at
  rest, decrypted only in-memory immediately before a connector dials
  out, and are never present in any API response or log line.
- **Editable without re-entering credentials.** Editing a connection
  SHALL never require the user to re-type a secret they've already
  stored. Blank secret fields mean "keep the current value."
- **Discoverable through the catalog.** For ~20 well-known
  integrations, creating a connection is a one-click prefill (see
  [FR-04](./FR-04-integration-catalog.md)).

## User personas

| Persona          | Description                                                                                                              |
| ---------------- | ------------------------------------------------------------------------------------------------------------------------ |
| Editor           | Creates, edits, tests, and deletes connections; owns the day-to-day maintenance of connection configuration.             |
| Admin            | Sets guardrails (encryption key rotation, connection deletion policy) and reviews connections during onboarding/audits. |
| Analyst          | Consumes connections via Explore or workflows; may not have `connections:write` at all.                                  |
| Ops / SRE        | Runs health checks on unhealthy connections and reads audit logs to diagnose downstream outages.                        |

## User stories

- **US-03.1** As an editor, I want to add a new data source in under a
  minute so that I can start querying it right away.
- **US-03.2** As an editor, I want to edit a connection without
  re-typing my password / API key / OAuth client secret so that
  routine changes (renaming, adjusting a scope) don't force me to
  fetch a credential from a password manager.
- **US-03.3** As an editor, I want to delete a stale connection with a
  visible confirmation prompt so that I don't accidentally remove one
  that a workflow still depends on.
- **US-03.4** As an editor working on a REST API, I want to pick from a
  drop-down of the ten auth schemes real APIs actually use — Basic,
  Bearer, API key, Digest, OAuth2, JWT, workload identity, Kerberos —
  so that I never have to hand-craft an auth header.
- **US-03.5** As an editor working with cloud services, I want to be
  able to omit static credentials and rely on the server's ambient
  identity (IAM role / ADC / DefaultAzureCredential) so that no
  long-lived cloud secret needs to be stored at all.
- **US-03.6** As an admin, I want a full audit trail of who created,
  edited, or deleted every connection so that access changes are
  traceable.
- **US-03.7** As an editor, I want to see the connection's current
  health status directly in the list (last tested, last error) so that
  I don't need to open each one to know if it's working.

## Functional requirements

### FR-03.1 — Supported connection types

The system SHALL support the following connection **types**, each
identified by a stable string in the API:

| Type       | Underlying source                                                                        |
| ---------- | ---------------------------------------------------------------------------------------- |
| `postgres` | PostgreSQL (pgx driver)                                                                  |
| `mysql`    | MySQL (`database/sql` + `go-sql-driver/mysql`)                                           |
| `rest`     | Any HTTP+JSON API (`pkg/httpclient`)                                                     |
| `graphql`  | Any GraphQL API over HTTP (`pkg/httpclient`)                                             |
| `aws`      | AWS services (Athena / CloudWatch Logs Insights / DynamoDB / S3, via `aws-sdk-go-v2`)   |
| `gcp`      | Google Cloud (BigQuery / GCS, via `cloud.google.com/go/*`)                               |
| `azure`    | Azure (Log Analytics / Blob Storage, via `azure-sdk-for-go`)                             |

Adding a new type requires implementing the `Connector` interface
(`Test` + `Execute`) and registering it in `cmd/server/main.go`.

### FR-03.2 — Connection resource model

A connection SHALL be persisted as a row containing:

- `name` (unique, non-empty, user-supplied)
- `type` (from the FR-03.1 vocabulary)
- `description` (optional free text)
- `config` (JSONB, non-sensitive fields: host, port, database, base URL,
  auth type + non-secret auth parameters, cloud region/service, ...)
- `secret_encrypted` (AES-256-GCM ciphertext, hex-encoded; empty for
  no-secret connections)
- `status` (`unverified` / `healthy` / `unhealthy`)
- `last_tested_at` (nullable timestamp)
- `last_error` (string; empty when healthy)
- `created_by` (user ID)
- `created_at`, `updated_at` (timestamps, managed automatically)

The **encryption key** is loaded from the `CONNECTION_ENCRYPTION_KEY`
environment variable and the app SHALL refuse to start in
`APP_ENV=production` without one supplied.

### FR-03.3 — Connection list

`GET /api/v1/connections` SHALL return every connection visible to the
caller. There is no per-connection ACL today (see
[FR-02](./FR-02-role-based-access-control.md)); the caller either has
`connections:read` or they don't. The response SHALL include for each
connection: `id`, `name`, `type`, `description`, `status`,
`lastTestedAt`, `lastError`, `lastErrorCode`, `lastErrorRemediation`,
`createdAt`, `updatedAt`, and a **safe** view of `config` with no
secret fields. `secret_encrypted` SHALL NEVER appear in any response.

### FR-03.4 — Create connection

`POST /api/v1/connections` SHALL create a new connection with:

- Non-empty `name`, unique across all connections.
- Valid `type`.
- Type-specific `config` validated by the connector's own validator.
- `secret` map validated against the auth type's required keys (e.g.
  a `basic` REST connection requires `username` and `password`; an
  `apiKey` connection requires `apiKey`).

The endpoint SHALL:

1. Validate all fields.
2. Encrypt the secret map with AES-256-GCM using a fresh random nonce.
3. Persist the row with `status = 'unverified'`.
4. Write an `audit_logs` entry (`connection.create`).
5. Return the safe view of the new connection (no secret).

### FR-03.5 — Update connection

`PUT /api/v1/connections/{id}` SHALL update an existing connection.

- The `type` SHALL NOT be changed. Changing type would leak semantics —
  the correct workflow is to create a new connection and delete the old
  one.
- Any secret key with a **non-empty** value SHALL replace the stored
  value for that key.
- Any secret key with an **empty** value (or omitted entirely) SHALL
  keep the previously stored value. This is the "leave blank to keep"
  UX.
- The updated row's `updated_at` timestamp SHALL be refreshed.
- On successful save, `status` SHALL be reset to `unverified` and
  `last_error` cleared — the caller is expected to run the health
  check separately.
- An `audit_logs` entry (`connection.update`) SHALL be written.

### FR-03.6 — Delete connection

`DELETE /api/v1/connections/{id}` SHALL delete a connection.

- The API SHALL check that no workflow references the connection in its
  DAG. If any do, the delete SHALL fail with a `409 conflict` and a
  message listing the referencing workflows' names.
- An `audit_logs` entry (`connection.delete`) SHALL be written before
  the row is removed, so a deleted connection's history is still
  intact.

### FR-03.7 — Per-type configuration surface

The **connection form** SHALL adapt to the selected `type`:

**Postgres / MySQL:**
- Host, Port, Database, User, Password (with hint "leave blank to keep"
  when editing).
- Postgres also has an SSL mode selector: `disable`, `prefer`,
  `require`, `verify-full`. Default is `require`.
- A guidance line reads: **"Use a read-only database role for this
  connection."**

**REST / GraphQL:**
- Base URL (REST) or GraphQL endpoint (GraphQL). Must be `http` or
  `https`.
- An **Authentication** drop-down that switches the auth-specific
  sub-form (see FR-03.8).

**AWS:**
- Region + Service selector: Athena / CloudWatch Logs Insights /
  DynamoDB / S3.
- Service-specific fields (e.g. Athena: Database, Workgroup, Query
  result output location).
- Static credentials (Access Key ID / Secret Access Key / Session
  Token): all optional, with the hint **"Leave credentials blank to use
  the server's ambient AWS identity."**
- STS assume-role section: Role ARN, External ID, Role session name —
  optional, layered on top of whichever identity was resolved above.

**GCP:**
- Project ID + Service selector: BigQuery / GCS.
- Service account key JSON (multi-line, optional; if empty the SDK's
  Application Default Credentials are used).
- **Impersonate service account** field with the hint that the calling
  identity needs `roles/iam.serviceAccountTokenCreator` on the target.

**Azure:**
- Service selector: Log Analytics / Blob Storage.
- Workspace ID (Log Analytics) or Storage account name (Blob Storage).
- Tenant ID, Client ID, Client Secret — all optional (hint:
  `DefaultAzureCredential`).
- Alternative **Client certificate** (PEM/PKCS12) with optional
  password.

Screenshots:
[AWS](../screenshots/12-connection-form-aws-athena.png),
[GCP](../screenshots/13-connection-form-gcp-bigquery.png),
[Azure](../screenshots/14-connection-form-azure-loganalytics.png),
[AWS assume role](../screenshots/31-aws-assume-role-fields.png),
[GCP impersonation](../screenshots/32-gcp-impersonation-field.png),
[Azure client certificate](../screenshots/33-azure-client-certificate-field.png).

### FR-03.8 — Auth-type matrix for REST / GraphQL

The **Authentication** drop-down offers, at minimum, the following
schemes, and the form renders the fields each requires. Every secret
field respects the "blank means keep" edit UX.

| Auth type              | Fields                                                                                                             |
| ---------------------- | ------------------------------------------------------------------------------------------------------------------ |
| `none`                 | (no fields)                                                                                                        |
| `basic`                | Username, Password                                                                                                 |
| `bearer`               | Token                                                                                                              |
| `apiKey`               | Header or Query param toggle, Name, Value                                                                          |
| `digest`               | Username, Password                                                                                                 |
| `oauth2ClientCreds`    | Token URL, Scopes, Client ID, Client Secret                                                                        |
| `oauth2RefreshToken`   | Token URL, Scopes, Client ID, Client Secret, Refresh Token                                                         |
| `jwt`                  | Algorithm (HS256/RS256), TTL seconds, Signing key, JSON claims                                                     |
| `workloadIdentity`     | Token endpoint, Audience, Scope, Subject token file path or static token                                           |
| `kerberos`             | Realm, Username, SPN, `krb5.conf` path, keytab path or password                                                    |

The form SHALL show a docs link when the connection was prefilled from a
catalog entry, pointing at the vendor's auth documentation (see
[FR-04](./FR-04-integration-catalog.md)).

Reference screenshot:
[`docs/screenshots/05-connection-form-graphql-oauth2.png`](../screenshots/05-connection-form-graphql-oauth2.png).

### FR-03.9 — Guardrails on stored credentials

- The `secret_encrypted` column SHALL NEVER appear in any API response,
  log line, error message, or metric label.
- The plaintext secret map SHALL exist in the server process's memory
  only during the encryption step (on save) or immediately before
  handing it to a connector's `Test`/`Execute` call.
- A wrong password SHALL surface to the user as "**the database
  rejected the username or password**" (or the analogous cloud message)
  and NEVER echo the attempted secret back.
- The encryption key SHALL be rotated by re-encrypting all
  `secret_encrypted` values with the new key in a maintenance script;
  see [`SECURITY.md`](../SECURITY.md#known-limitations--not-yet-implemented).

## UI/UX requirements

### Connections list page (`/connections`)

Reference screenshot: [`docs/screenshots/04-connections.png`](../screenshots/04-connections.png).

- Table columns: **Name**, **Type**, **Status** (clickable, opens
  health modal), **Last tested**, **Actions**.
- Header buttons (gated by `connections:write`):
  - **Browse catalog** — opens the [Integration
    Catalog](./FR-04-integration-catalog.md) modal.
  - **New connection** — opens a blank `ConnectionFormModal`.
- Per-row icon buttons (order left-to-right):
  1. **Test connection** — icon button; disabled while the row's test
     is in flight; requires `connections:test`.
  2. **View health** — opens the health modal; requires
     `connections:test`.
  3. **Run query** — opens the ad-hoc query modal (see
     [FR-06](./FR-06-ad-hoc-exploration.md)); requires
     `connections:read`.
  4. **Edit** — opens the connection form pre-populated; requires
     `connections:write`.
  5. **Delete** — native `confirm()` prompt "**Delete connection
     `<name>`?**", then requires `connections:write`.
- Empty state (spans full table width): **"No connections yet."** with
  a helpful "Get started" callout linking to Browse catalog.
- Loading state: a **"Loading…"** cell.
- If a connection is `unhealthy`, its status badge SHALL display the
  short error under the badge; clicking the badge opens the health
  modal for details.

Reference screenshot:
[`docs/screenshots/28-connections-unhealthy-row.png`](../screenshots/28-connections-unhealthy-row.png).

### Connection form modal

- Title toggles between **New connection** and **Edit connection**.
- Fields, top to bottom: Type (drop-down; disabled in edit mode), Name
  (required), Description (optional), then the type-specific fields.
- Primary button label: **Save connection** / **Saving…**.
- Errors SHALL surface as an inline red banner at the top of the modal,
  never as a native browser alert.
- All secret inputs SHALL be `type="password"` and SHALL show the "leave
  blank to keep current value" hint when editing.

## Acceptance criteria

- [ ] A user without `connections:write` sees the Connections page but
  no Create/Edit/Delete controls.
- [ ] Creating a connection with a duplicate name fails with a
  `409 conflict` and a clear inline message.
- [ ] Creating a connection with an invalid `type` fails with `400`.
- [ ] Creating a Postgres connection persists `sslmode` in `config`
  and does not leak the password into `config`.
- [ ] After saving, the API response contains **no** field named
  `secret`, `secret_encrypted`, `password`, `token`, or `apiKey`.
- [ ] Editing a connection and submitting an empty password field
  preserves the previous password (verified by a subsequent successful
  test).
- [ ] Editing a connection and submitting a non-empty password field
  overwrites it (verified by a subsequent test using the new
  credential succeeding and using the old credential failing).
- [ ] Deleting a connection referenced by a workflow returns
  `409 conflict` with the referencing workflow name in the message.
- [ ] All CRUD operations produce corresponding `audit_logs` entries
  (`connection.create` / `.update` / `.delete`) with the acting
  user, the connection ID, and the outcome.
- [ ] Every server log line related to a connection redacts the
  encrypted secret and the decrypted secret. (Verified by a
  content-based grep in tests.)
- [ ] The connection form correctly renders every supported auth type
  and every cloud service without console errors.
- [ ] Saving a REST connection whose `baseUrl` is neither `http` nor
  `https` fails with `400 invalid config`.

## Edge cases & error handling

- **A connection deleted while another user has an open Explore tab
  against it.** The next query against the deleted connection SHALL
  fail with `404 not_found` and a clear message
  "**Connection no longer exists.**" The SPA SHALL refresh its
  connection list.
- **A connection updated while a workflow is executing against it.**
  The in-flight execution completes against the pre-update credentials
  (the secret has already been decrypted and handed to the connector);
  subsequent executions use the new credentials.
- **Missing `CONNECTION_ENCRYPTION_KEY` in dev.** Dev mode SHALL use a
  well-known, non-secret key with a very loud warning at startup.
  Production SHALL refuse to start entirely.
- **Restore from a backup without the encryption key.** Every
  `secret_encrypted` row SHALL be unreadable. The system SHALL surface
  connections as `unverified` with an error code
  `encryption_key_mismatch` when a test or query is attempted, so the
  operator understands what happened.
- **A very large `config` blob.** The API SHALL reject request bodies
  larger than 1MB (`httpx.MaxRequestBodyBytes`) with `413`.

## Non-functional requirements

- **Security.** See [`SECURITY.md`](../SECURITY.md) sections
  "Secrets at rest" and "Data source access (SQL connectors)".
- **Isolation between connections.** A misbehaving connection SHALL
  NOT affect any other. Per-connection rate limiting (in-memory,
  per-instance) is applied at `connections.Service`.
- **Latency.** Connection list and detail endpoints SHALL return in
  < 200 ms at the 95th percentile against a moderate dataset (100
  connections).
- **Consistency.** Every listed connection SHALL have its
  `status`/`lastTestedAt` field updated within a bounded window after a
  test.

## Market context & differentiation

| Product     | Connection model                                                                                                                                        |
| ----------- | ------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Grafana     | Data sources are per-org, per-type plugins; auth options vary widely by plugin.                                                                          |
| Metabase    | "Databases" only (SQL sources) as first-class citizens; APIs need a custom Metabase driver.                                                              |
| Retool      | Resources (connections) with per-team ACLs, per-user API tokens, and a huge integration catalog.                                                        |
| Postman     | "Environments" hold auth per-request; connections aren't a separate resource. Suited to individual developers, not team workflows.                       |
| n8n         | "Credentials" as a separate resource; a workflow references credentials by ID. Similar shape to Data Explorer.                                          |
| Airbyte     | "Sources" and "destinations" with elaborate connector-specific config forms.                                                                             |

**Where Data Explorer is intentionally different.** The unified
"connection = config + encrypted secret + auth type" model spans SQL,
HTTP, and cloud in one CRUD flow, one permission (`connections:write`),
and one audit surface. Cloud identities can be *ambient* — no
long-lived key needs to be typed at all — while still supporting
scoped-down-from-there mechanisms (STS AssumeRole, GCP impersonation,
Azure client certificate). Together this is meant to feel like Grafana's
data-source management but with n8n's clarity of "one credentials
resource, referenced from everywhere."

## Future enhancements (out of scope for this FR)

- **Per-connection ACLs.** Restrict `connections:read` per-connection
  to a specific team or set of users.
- **Connection folders / tags.** Group connections into logical
  buckets and filter the list by them.
- **Bulk import / export.** Import a list of connection templates from
  a YAML file (without secrets).
- **Automatic key rotation.** Move `CONNECTION_ENCRYPTION_KEY` to a
  primary/secondary pair with a background re-encryption task.
- **More sources.** MongoDB, Snowflake, ClickHouse, Kafka, DuckDB.
- **Connection change history.** Beyond audit logs, a versioned diff of
  what changed on each `.update`.
- **Test on save.** Optionally run the health check as part of the save
  request when the caller opts in.

## Cross-references

- Implementation: `backend/internal/connections/`,
  `backend/internal/connections/connectors/`,
  `backend/internal/api/handlers/connections.go`,
  `frontend/src/pages/ConnectionsPage.tsx`,
  `frontend/src/pages/connections/*`.
- Related FRs: [FR-04 Integration Catalog](./FR-04-integration-catalog.md),
  [FR-05 Health Monitoring](./FR-05-connection-health-monitoring.md),
  [FR-06 Ad-Hoc Exploration](./FR-06-ad-hoc-exploration.md),
  [FR-07 Workflow Builder](./FR-07-visual-workflow-builder.md).
- Architecture: [`../ARCHITECTURE.md`](../ARCHITECTURE.md) sections
  "Connections and secrets" and "Cloud provider connectors".
