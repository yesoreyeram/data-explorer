# FR-06 — Ad-Hoc Data Exploration

## Overview

The **Explore** surface is a lightweight, query-and-preview workspace where a
user can point Data Explorer at a data source, issue a single query (SQL,
REST/GraphQL request, or a cloud-provider request), and see the response
rendered as a tabular dataframe within seconds. It is the fastest path from
"I want to look at data" to "I'm looking at data" in the product and is
intentionally decoupled from the long-lived pipeline (workflow) surface: no
scheduling, no persistence of results, and no need to first create a saved
connection.

Unlike a full BI dashboard, Explore does not save the answer — it saves the
question. The user leaves with a URL / recent-queries entry they can re-run,
an exported CSV/JSON, or a workflow node seeded from what they just tried.
The goal is to make discovery feel like a REPL, not a report.

## Product goals

- Give any authenticated user a **frictionless first success**: from login
  to first row of data in under a minute, without touching a config file or
  restarting a service.
- Allow both **saved-connection** (reuse credentials that have already been
  vetted by an editor/admin) and **temporary-connection** (paste
  credentials, run once, throw away) exploration in the same UI.
- Render responses in a consistent **dataframe grid** regardless of whether
  the source is SQL, REST, GraphQL, or a cloud service, so users don't need
  to context-switch between viewers.
- Export the current result to CSV or JSON with a single click for handoff
  to spreadsheets, notebooks, or teammates.
- Feed Explore output straight into the pipeline builder — the "I ran it
  once, now I want it every hour" journey.

## User personas

| Persona                        | Description                                                                                              |
| ------------------------------ | -------------------------------------------------------------------------------------------------------- |
| **Analyst**                    | Runs ad-hoc SQL against a warehouse or a REST/GraphQL query against an API. Rarely creates workflows.    |
| **Editor** (application dev)   | Uses Explore to validate that a saved connection actually returns the expected data before wiring it in. |
| **Viewer**                     | Read-only user allowed to browse existing saved connections and re-run queries but not create new ones.  |
| **Ops engineer**               | Explores CloudWatch Logs / Log Analytics with a KQL / Insights query while investigating an incident.    |
| **Onboarding user**            | First-time user pasting credentials from a scratchpad to see whether the tool works before committing.   |

## User stories

- **US-06.1** As an analyst, I want to select one of my organization's saved
  Postgres connections and type SQL against it, so I can inspect data
  without asking a DBA for credentials.
- **US-06.2** As an editor, I want to paste a REST endpoint URL, a bearer
  token, and a JSON path into an Explore form, so I can validate an API
  response shape before creating a persistent connection.
- **US-06.3** As an analyst, I want the result of my query rendered as an
  interactive table with column headers and types, so I can eyeball the
  data structure without leaving the app.
- **US-06.4** As an analyst, I want to click "Export CSV" and "Export JSON"
  on the result, so I can share the answer with a teammate who is not a
  Data Explorer user.
- **US-06.5** As an analyst, I want my most recent queries listed on the
  Explore page, so I can re-run a query I just wrote without re-typing it.
- **US-06.6** As an operations engineer, I want to run a CloudWatch Logs
  Insights query in the same UI I use for SQL, so I don't have to jump to
  the AWS console during an incident.
- **US-06.7** As an editor, I want to turn a working Explore query into a
  workflow source node with a single click, so I can move from "it worked
  once" to "it runs every morning" in seconds.
- **US-06.8** As a security-conscious user, I want assurance that temporary
  credentials I paste into an Explore session are never written to disk or
  the audit log, so I can safely try the product with production tokens.

## Functional requirements

### FR-06.1 — Two exploration modes

The Explore page SHALL support two mutually exclusive modes selected via a
toggle at the top of the page:

- **Saved connection**: choose a connection from the same list managed on
  the Connections page. The user's `connections:read` permission is
  required to see the list.
- **Temporary connection**: fill in a connection form inline (type,
  auth, credentials, base URL / DSN, region, etc.) that is used only for
  the duration of the request and never persisted.

The mode toggle SHALL be reflected in the URL query string so a specific
mode can be linked to.

### FR-06.2 — Query editor

The Explore page SHALL provide a query editor whose shape depends on the
connection type:

- **SQL sources** (Postgres, MySQL, Athena, BigQuery): a multi-line SQL
  textarea with monospace font.
- **REST**: method dropdown (GET / POST / PUT / PATCH / DELETE), path
  input, optional headers editor, optional body editor, and an optional
  JSON-path/JSONata expression for extracting the tabular slice.
- **GraphQL**: query textarea, variables JSON, and an optional
  JSON-path/JSONata expression.
- **CloudWatch Logs**: log-group selector plus an Insights query editor.
- **Log Analytics**: workspace selector plus a KQL editor.
- **DynamoDB**: table selector, key-condition builder, projection.
- **S3 / GCS / Blob Storage**: object-key input with an optional format
  hint (csv/tsv/ndjson/parquet).

The editor SHALL preserve its content when the user switches between
saved and temporary modes, subject to type compatibility.

### FR-06.3 — Run and preview

Clicking **Run** SHALL:

1. Validate the current form (target, credentials for temporary mode,
   query non-empty).
2. Submit the query to the backend. For saved connections the backend
   loads the connection and decrypts the secret internally; for
   temporary connections the credentials are passed in the request body
   over TLS and are not stored.
3. Enforce the same guardrails as workflow source nodes: 25MB response
   body cap for HTTP, 10K row cap for tabular results, ≤5 redirects,
   ≤20 pages, per-connection rate limit.
4. Render the response as a `Frame` in the `DataFrameView` grid, showing
   column names, inferred column types, and row count.
5. Show a duration badge (elapsed ms) next to the result.

### FR-06.4 — Structured errors

If the query fails, the response SHALL include the same
[health-error taxonomy](./FR-05-connection-health-monitoring.md) used by
Connections (code + message + remediation), rendered inline above the
grid with a colored status dot and a "Copy details" affordance. The
result grid area SHALL be replaced with the error panel — partial rows
SHALL NOT be shown when the query errored.

### FR-06.5 — Recent queries

The Explore page SHALL display a **Recent queries** list showing up to
20 most recent Explore runs by the current user, ordered
most-recent-first. Each entry SHALL show connection name (or "Temporary"
for temporary-mode runs), a truncated query preview, timestamp, and
run duration. Clicking an entry SHALL restore the query into the editor
without automatically running it.

For temporary-mode runs the query and target SHALL be recorded but
the credentials SHALL NOT — re-running a "Temporary" entry requires
the user to re-enter credentials.

### FR-06.6 — Export result

When a result is on screen, the Explore page SHALL expose:

- **Export CSV** — downloads the currently-displayed frame as
  RFC 4180 CSV with a UTF-8 BOM and column headers.
- **Export JSON** — downloads the frame as an array of objects keyed
  by column name.

Export SHALL use the frame as it currently appears in the browser (no
extra server round-trip); the user gets exactly what they see.

### FR-06.7 — Feed into a workflow

Explore SHALL provide a **Create workflow from this query** action that,
on click, opens the Workflow Builder with a pre-populated source node
matching the current mode:

- Saved connection: the source node references the same `connection_id`
  and query.
- Temporary connection: the credentials MUST be omitted from the seeded
  workflow — the user is prompted to create a saved connection first.

### FR-06.8 — Permissioning

Access to Explore SHALL follow RBAC:

- `connections:read` is required to select from saved connections.
- `connections:test` is required to run a temporary connection (the
  temporary path issues an outbound request to a user-controlled target
  and therefore has the same permission as a health probe).
- No permission gates the Recent-queries list beyond the user seeing
  only their own history.

### FR-06.9 — No result persistence

Query results SHALL NOT be persisted server-side. Reloading the page
after a run SHALL clear the on-screen result. Recent queries persist
metadata (target, query text, duration, timestamp) but not row data.

### FR-06.10 — Audit and logging

Every Explore run SHALL emit a `connections.explore` audit event with:
`user_id`, `connection_id` (nullable for temporary mode),
`connection_type`, `query_hash`, `row_count`, `duration_ms`,
`success`, and, on failure, the error code. Neither the raw query text
nor any credential SHALL be included in the audit payload.

## UI/UX requirements

- The Explore page follows the near-monochrome design language: neutral
  surfaces, single accent color for primary action, status dots
  (green/yellow/red) reserved for run outcome.
- Empty state (never run before): centered illustration + copy "Try a
  query against one of your connections, or paste credentials to try
  something new." — see [`docs/screenshots/18-explore-empty.png`](../screenshots/18-explore-empty.png).
- Saved-connection mode: dropdown lists connections grouped by type,
  with an inline health dot next to each — see
  [`docs/screenshots/19-explore-saved-connection.png`](../screenshots/19-explore-saved-connection.png).
- Result grid: sticky headers, monospace numeric columns,
  right-aligned numbers, left-aligned text, null values rendered as a
  dimmed en-dash — see
  [`docs/screenshots/20-explore-saved-result.png`](../screenshots/20-explore-saved-result.png).
- Temporary-connection mode: the credential fields have a `password`
  input type and are visually grouped inside a "This connection is not
  saved" callout — see
  [`docs/screenshots/21-explore-temporary-connection.png`](../screenshots/21-explore-temporary-connection.png).
- Temporary-mode result: identical layout to saved-mode result but with
  a "Temporary" badge on the run duration chip — see
  [`docs/screenshots/22-explore-temporary-result.png`](../screenshots/22-explore-temporary-result.png).
- Export buttons appear above the grid and are disabled until a result
  is available — see
  [`docs/screenshots/23-explore-export-buttons.png`](../screenshots/23-explore-export-buttons.png).
- Recent-queries list sits below the editor, collapsible, with a
  "Clear history" affordance — see
  [`docs/screenshots/24-explore-recent-queries.png`](../screenshots/24-explore-recent-queries.png).
- Keyboard: **Ctrl/Cmd+Enter** runs the current query. Focus returns
  to the editor after run so the user can iterate.

## Acceptance criteria

- [ ] Selecting a saved Postgres connection, typing `SELECT 1`, and
  pressing **Run** renders a one-row, one-column grid with the value
  `1` within 2 seconds.
- [ ] Selecting a REST temporary connection, entering a GET URL with a
  bearer token, and pressing **Run** renders the response body extracted
  by the JSON-path expression as a table.
- [ ] Running a syntactically invalid SQL query surfaces the classified
  error code (e.g. `invalid_config`) with a remediation string and does
  not render a partial grid.
- [ ] Clicking **Export CSV** downloads a CSV whose header row matches
  the displayed columns and whose row count matches the displayed row
  count.
- [ ] Clicking **Export JSON** downloads an array of objects whose keys
  match the displayed columns.
- [ ] Reloading the Explore page clears the result grid but keeps the
  Recent queries list.
- [ ] Recent queries entries are limited to 20 and are stored per-user;
  no other user can see them.
- [ ] Temporary-mode credentials never appear in the browser history,
  the recent-queries list, the audit log, or the backend logs.
- [ ] A user without `connections:test` sees the temporary-mode toggle
  disabled with an inline tooltip explaining why.
- [ ] `connections.explore` audit events are emitted with the required
  fields and no secret payload.

## Edge cases & error handling

- **Empty result**: A 0-row result renders the grid with column
  headers only and shows the copy "No rows returned." The export
  buttons remain enabled and produce a header-only CSV / an empty
  JSON array.
- **10K-row cap**: When the source returns more rows than the row cap,
  the grid renders the first 10K rows and a warning banner explains
  the truncation with a link to the workflow builder (where a
  filter/aggregate can reduce the row count).
- **25MB response cap** (HTTP): The request is aborted; the error panel
  shows code `invalid_config` with remediation "Response exceeded 25MB
  cap — narrow the query or add pagination."
- **Rate-limit exceeded**: The error panel shows code `rate_limited`
  with remediation "You have run too many queries against this
  connection recently — wait a moment and retry."
- **Slow query**: A spinner appears after 500ms; a cancel button
  appears after 5 seconds; the request is server-side cancelled if
  the user clicks cancel.
- **Concurrent runs**: Clicking Run while a previous run is in flight
  cancels the previous run rather than queueing.
- **Temporary connection with invalid credentials**: The error panel
  shows code `auth_failed` and remediation "The credentials provided
  did not authenticate — re-check the token/password and try again."
- **Recent-query replay against a deleted connection**: Clicking the
  entry restores the query but disables the Run button and shows
  "The connection this query used has been deleted."
- **Multi-tab isolation**: Recent queries entries are made server-side,
  so opening a second tab immediately sees the newest run.

## Non-functional requirements

- **Latency**: The API round-trip overhead (excluding the source's own
  query time) SHALL be under 200ms at p95.
- **Isolation**: A single user's runs SHALL NOT be able to starve
  another user's runs; per-connection rate limits are enforced at the
  service layer.
- **Security**: Temporary credentials SHALL be zeroed from memory
  after the request completes. No `X-Cache` header or CDN cache is
  applied to the run endpoint.
- **Auditability**: Every run — successful or not — SHALL leave a
  single audit record. No run SHALL be silently dropped.
- **Accessibility**: The result grid SHALL be navigable by keyboard
  (arrow keys move focus between cells; Home/End jump to row edges);
  column headers SHALL be announced by screen readers with column
  types.

## Market context & differentiation

| Product              | Ad-hoc mode                                                            | Notes                                                                            |
| -------------------- | ---------------------------------------------------------------------- | -------------------------------------------------------------------------------- |
| **Metabase**         | "Ask a question" / SQL editor                                          | Focus on saving as a card / dashboard; less strong for REST/GraphQL/CloudWatch.  |
| **Superset**         | SQL Lab                                                                | Excellent SQL editor; no REST or cloud-service exploration.                      |
| **Postman**          | Request builder                                                        | Excellent for REST/GraphQL; no tabular grid, no SQL, no cloud-service parity.    |
| **Retool**           | Query editor inside an app                                             | Requires wrapping the query in an app; not a "just look at data" surface.        |
| **AWS console**      | Query editor per service (Athena, CloudWatch Logs Insights, DynamoDB)  | Requires jumping between services and is AWS-only.                               |
| **DBeaver / TablePlus** | SQL editor                                                          | Great for SQL only; not for REST, cloud logs, or blob objects.                   |
| **Hex / Deepnote**   | Notebook cells                                                         | Powerful but heavyweight; multi-cell state is overkill for one-shot exploration. |

Data Explorer's differentiators for Explore are:

- **One editor for every source type.** SQL, REST, GraphQL, CloudWatch,
  Log Analytics, DynamoDB, and blob-store objects are all queried and
  previewed in the same tabular grid.
- **Temporary connection mode.** Try a new API or a personal token
  without touching the shared connection list — nothing is persisted
  server-side beyond metadata.
- **Zero-friction path to a scheduled workflow.** A working Explore
  query can be promoted to a workflow source node in one click.
- **Consistent error taxonomy.** The same 8 error codes appear whether
  you're diagnosing a bad SQL, an expired OAuth token, or a
  network-unreachable Athena workgroup.
- **Uniform export.** CSV/JSON export works identically for every
  source type — the user never learns per-source export UI.

## Future enhancements (out of scope)

- Multi-cell scratchpad (like a lightweight notebook) that binds
  results of one query into the input of the next.
- Explore-side charting on top of the result frame.
- Result diff between two Explore runs.
- Sharing an Explore result via a signed URL that expires.
- Result snapshot pinning for a fixed window (e.g. "keep this result
  for 24 hours").
- Autocomplete for SQL against the connection's schema (currently
  users type SQL blind).
- KQL / Insights-query linting inside the editor.

## Cross-references

- [FR-03 Data Source Connection Management](./FR-03-connection-management.md) —
  the connection resource Explore reads from.
- [FR-05 Connection Health Monitoring](./FR-05-connection-health-monitoring.md) —
  the error taxonomy Explore reuses for run failures.
- [FR-07 Visual Workflow Builder](./FR-07-visual-workflow-builder.md) —
  the surface an Explore query is promoted into.
- [FR-09 Query Result Export & Sharing](./FR-09-query-result-export.md) —
  the export surface used by Explore and the workflow output node.
- [FR-10 Audit Logging & Compliance](./FR-10-audit-logging.md) —
  the audit stream Explore writes to.
