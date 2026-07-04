# FR-09 — Query Result Export & Sharing

## Overview

Data Explorer treats **the tabular result** as a first-class artifact. A
query result — whether from Ad-Hoc Explore, a workflow output node, or a
finished scheduled run — can be exported in a machine-readable format
that a non-Data-Explorer user can open in Excel, Google Sheets, jq,
pandas, or their own tooling. Export is deliberately client-side where
possible (no server round-trip when the data is already in the browser)
and produces canonical formats (RFC 4180 CSV, JSON arrays of objects,
NDJSON where appropriate) so results interoperate everywhere.

Beyond raw export, this feature also covers the copy-paste story
(clipboard-friendly TSV that pastes cleanly into a spreadsheet) and,
for workflow output nodes, delivering results to storage targets like
S3 / GCS / Azure Blob so downstream systems can pick them up on a
schedule.

## Product goals

- Give users an **immediate path to reuse** the data they see —
  spreadsheet, notebook, colleague — without SQL or an API integration.
- Produce **canonical, tool-agnostic** output formats: CSV that opens
  in Excel without an import wizard, JSON that parses in every
  language.
- Never silently truncate or reformat the data — the export SHALL match
  the visible frame row for row and column for column, with any caps
  clearly signalled.
- Support **delivery targets** for workflow outputs: an object dropped
  into a blob store on a schedule is often the actual product.
- Keep export cost negligible: don't re-run the query, don't
  re-fetch, don't require a background job for typical result sizes.

## User personas

| Persona                       | Description                                                                                              |
| ----------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Analyst**                   | Wants a CSV they can drop into a spreadsheet for further slicing.                                        |
| **Engineer (downstream)**     | Consumes JSON/CSV artifacts dropped into a bucket by a scheduled workflow.                               |
| **Ops engineer**              | Runs a one-off query, copies the result to the clipboard, pastes into an incident chat.                  |
| **Non-technical stakeholder** | Receives an email or a link to a spreadsheet and doesn't need to know Data Explorer exists.              |
| **Auditor**                   | Verifies which exports happened, from which query, by whom, at what time.                                |

## User stories

- **US-09.1** As an analyst, I want to click **Export CSV** on my
  Explore result, so I can open the numbers in Excel/Google Sheets.
- **US-09.2** As an analyst, I want to click **Export JSON**, so I can
  paste the data into a notebook.
- **US-09.3** As an ops engineer, I want to select a range of rows and
  copy them to the clipboard as TSV, so I can paste them into an
  incident chat as a table.
- **US-09.4** As an engineer, I want a workflow output node to write a
  gzipped NDJSON file to S3 with a timestamped key, so my downstream
  systems can `s3 ls | sort` and pick up the newest file.
- **US-09.5** As an auditor, I want every export to leave a record with
  who exported what, when, and how many rows — but not the row data
  itself.
- **US-09.6** As an analyst, I want the CSV to open cleanly in Excel
  including special characters (é, £, 中文), so I don't have to do a
  UTF-8 import dance.
- **US-09.7** As a security-conscious user, I want assurance that the
  export path enforces the same row cap as the on-screen grid, so an
  Export CSV button cannot accidentally leak millions of rows.
- **US-09.8** As an admin, I want to know how large a workflow's most
  recent output is, so I can estimate storage cost.

## Functional requirements

### FR-09.1 — Export formats supported

Data Explorer SHALL support the following export formats from Ad-Hoc
Explore and the workflow run panel:

- **CSV** (RFC 4180): comma separator, `"` quoting, `\r\n` line
  endings, UTF-8 encoding with BOM (for Excel compatibility), first
  row is column headers.
- **JSON**: array of objects, one object per row, keys equal to
  column names, values encoded per column type (numbers as JSON
  numbers, strings as JSON strings, nulls as JSON `null`, booleans
  as JSON booleans, dates as ISO 8601 strings).
- **TSV** (clipboard-only): tab separator, no quoting, `\n` line
  endings — chosen for clean spreadsheet paste behaviour.

### FR-09.2 — Client-side export path (Explore, run panel)

When the frame is already resident in the browser (Explore result,
run panel), the export action SHALL:

- Convert the on-screen frame to the requested format entirely
  client-side.
- Produce a downloadable file whose row count matches the visible
  row count.
- Not make a server round-trip.
- Use a `Content-Disposition: attachment; filename="<name>.<ext>"`
  header for the download, where `<name>` is the connection or
  workflow name plus an ISO 8601 timestamp.

### FR-09.3 — Copy to clipboard

The result grid SHALL support:

- **Copy selection** — copies the currently selected rows to the
  clipboard as TSV.
- **Copy all** — copies the entire visible frame as TSV.

Both actions SHALL work with standard Ctrl/Cmd+C when a cell is
focused, and via a dedicated menu item for discoverability.

### FR-09.4 — Workflow output node — file delivery

The `output` node SHALL support writing the frame to a delivery
target. Supported targets in the first release:

- **S3 / GCS / Azure Blob Storage** — object key + bucket / container,
  with format options `csv`, `json`, `ndjson`, `parquet` (parquet is
  optional — falls back to ndjson if not compiled in).
- **Connection re-write** — write into a SQL table via a Postgres /
  MySQL connection using `INSERT` / `UPSERT`.
- **Run-result artifact** (default) — stores the frame on the run
  itself for retrieval by API and by the UI, subject to a size cap.

### FR-09.5 — Row cap enforcement

All export paths SHALL respect the same row caps as the underlying
frame:

- 10K rows for Explore results.
- 100K rows per node for workflow frames.

Export SHALL NOT exceed these caps. Where the frame has been
truncated, the export SHALL include an accompanying banner (in the
UI) noting the truncation and the format-specific footer (for
formats that support it — none do inline; the UI callout is the
authoritative signal).

### FR-09.6 — Character encoding and cell formatting

Exports SHALL:

- Encode as UTF-8 with BOM for CSV (Excel compatibility).
- Escape embedded quotes by doubling them in CSV (RFC 4180).
- Escape embedded newlines by wrapping the cell in quotes.
- Emit ISO 8601 timestamps for date-typed columns (`2024-01-15T10:30:00Z`).
- Emit `null` (not the string "null") for null values in JSON.
- Emit an empty field (not "null") for null values in CSV.
- Preserve numeric precision by writing numbers as their canonical
  decimal representation.

### FR-09.7 — Audit trail for exports

Every export action SHALL emit an audit event:

- `explore.export` for Explore-mode exports.
- `workflow.export` for run-panel exports.
- `workflow.output` for output-node file deliveries.

Each event SHALL record `user_id`, `connection_id` or
`workflow_id`, `format`, `row_count`, `byte_count` (or a bounded
estimate), `target` (for output nodes), and `success`. Row payload
SHALL NOT be logged.

### FR-09.8 — Output-node object key conventions

For blob-store outputs, the object key SHALL support template
placeholders: `{{workflow_id}}`, `{{workflow_name}}`,
`{{run_id}}`, `{{yyyy}}`, `{{mm}}`, `{{dd}}`, `{{hh}}`, `{{ts}}`
(ISO 8601 timestamp). Placeholders that reference unknown values
SHALL cause a save-time validation failure with a clear message.

### FR-09.9 — Delivery credentials

Output-node delivery to blob stores SHALL use the credentials from
a saved connection (referenced by `connectionId` in the node's
config). Inline credentials in the node config SHALL NOT be
permitted.

### FR-09.10 — Failure surface

An export failure (blob PUT rejected, network unreachable, permission
denied) SHALL classify the error via the standard health taxonomy
(see FR-05) and record it on the run's node result. The run SHALL
be marked `failed` and downstream nodes SHALL NOT execute.

## UI/UX requirements

- **Explore result** exposes **Export CSV** and **Export JSON**
  buttons above the grid — see [`docs/screenshots/23-explore-export-buttons.png`](../screenshots/23-explore-export-buttons.png).
- The grid supports keyboard row selection (Shift+ArrowUp/Down) and
  **Ctrl/Cmd+C** copies the selection as TSV.
- A file name is proposed automatically:
  `<connection-or-workflow-name>-<yyyymmdd-HHMMSS>.csv`.
- The **output** node's config panel shows:
  - Target dropdown (S3, GCS, Azure Blob, SQL connection, Run-result).
  - Connection selector (populated by matching connection type).
  - Format dropdown.
  - Object-key or table-name input with placeholder helper.
  - Preview: "This will write to `s3://…/2024-01-15/orders.csv`".
- Run-result artifacts appear on the workflow run detail as a
  downloadable link with row count and byte size.

## Acceptance criteria

- [ ] Clicking **Export CSV** on a 10-row Explore result downloads a
  file whose row count is exactly 10 plus 1 header row.
- [ ] The exported CSV opens in Excel with correct column names and
  no import dialog.
- [ ] The exported CSV preserves special characters (`é`, `中文`,
  quotes, commas, newlines) losslessly.
- [ ] Clicking **Export JSON** produces a file that parses as a valid
  JSON array via `JSON.parse`.
- [ ] Row selection followed by Ctrl/Cmd+C places TSV on the
  clipboard with tab-separated fields.
- [ ] A workflow output node with target=S3 and format=NDJSON writes a
  single object to the bucket whose contents match the frame.
- [ ] An output node targeting a bucket the connection is not
  authorized to write to produces a `permission_denied` error code on
  the run.
- [ ] An audit event is emitted for every export with `row_count` and
  `byte_count` set and no row payload.
- [ ] An object-key template referencing an unknown placeholder
  (`{{unknown_field}}`) is rejected at workflow save time.
- [ ] Exporting a truncated result (10K row cap hit) is allowed and
  the UI shows a banner reminding the user of truncation.

## Edge cases & error handling

- **Empty frame**: CSV export produces a header-only file. JSON export
  produces `[]`. TSV clipboard copy places just column headers.
- **Very wide frame**: A frame with 500 columns is exported without
  reordering columns; the file may be large.
- **Numeric precision**: A `float64` column emits at most 17
  significant digits (round-trippable).
- **Column with mixed types (JSON REST source)**: Types are coerced to
  string in CSV; JSON preserves original type.
- **Null vs missing**: JSON preserves `null` explicitly; CSV emits
  empty string.
- **Column named `""` or with special characters**: Column headers
  are quoted per RFC 4180.
- **Blob-store slow write**: Output node respects the per-node 60s
  timeout and fails with `timeout` error code.
- **Blob-store payload > 25MB**: Not blocked by the HTTP guardrail
  since output uses provider SDKs, but any single object is bounded
  by the 100K-row frame cap.
- **Filename with reserved characters**: The proposed filename is
  sanitized to replace `/`, `\`, `:`, `*`, `?`, `"`, `<`, `>`, `|`
  with `_`.
- **Non-idempotent output**: Re-running a workflow whose output-node
  key template resolves to the same key SHALL overwrite the object
  (blob-store default semantics); users who need immutable outputs
  include a `{{run_id}}` in the key.
- **SQL output — schema drift**: An INSERT target whose table schema
  changed since the workflow was authored will fail with
  `invalid_config`; the run is marked failed.

## Non-functional requirements

- **Latency**: Client-side CSV/JSON export of ≤10K rows SHALL complete
  in under 1 second on a mid-range laptop.
- **Memory**: Client-side export SHALL stream into the download
  Blob so a 10K-row × 50-column export doesn't allocate the whole
  file twice in memory.
- **Correctness**: CSV output SHALL round-trip losslessly through
  `csv.Reader` (Go), `csv` (Python), and Excel/Google Sheets import.
- **Privacy**: Row data SHALL NOT be persisted server-side by the
  export path itself — only export metadata is written to the audit
  log.
- **Security**: Output-node writes SHALL use credentials from the
  referenced connection only; the node config SHALL never contain
  raw secrets.
- **Observability**: Export size distributions SHALL be observable
  via Prometheus histograms (`export_rows`, `export_bytes` labelled
  by format and source-page).

## Market context & differentiation

| Product           | Result export options                                            | Notes                                                                       |
| ----------------- | ---------------------------------------------------------------- | --------------------------------------------------------------------------- |
| **Metabase**      | CSV / JSON / XLSX / PNG                                          | Rich options but centered on saved questions and dashboards.                |
| **Superset**      | CSV / Excel                                                      | Standard SQL Lab export; per-chart export in dashboards.                    |
| **Looker Studio** | CSV / Excel / Google Sheets                                      | Deep GSheets integration; SaaS-locked.                                      |
| **Retool**        | CSV / clipboard / typed handlers                                 | Export depends on the app's designer.                                       |
| **Postman**       | Export request/response as file                                  | Not tabular — file per response.                                            |
| **DBeaver**       | CSV / TSV / SQL insert / XML / HTML                              | Rich format list, desktop-only.                                             |
| **Airbyte**       | Delivery-target focus (blob, warehouse)                          | No lightweight one-off export — everything is a sync.                       |
| **n8n**           | Per-node conversion + file operations                            | Users manually wire an export step; no first-class button.                  |

Data Explorer's differentiators for export & sharing:

- **Uniform export button on every result.** Same CSV / JSON contract
  whether the source is SQL, REST, GraphQL, DynamoDB, or a log query.
- **Excel-friendly CSV by default.** UTF-8 BOM, RFC 4180 quoting, no
  import wizard.
- **Delivery targets are a first-class output node.** No manual "then
  make a PUT request to S3" step.
- **Client-side export path.** No re-fetch, no server memory
  pressure, no audit-payload leakage.
- **Every export is audited but no row payload is logged.** The
  compliance answer is "yes we know it happened; no we didn't
  duplicate the data."
- **Object-key templating.** Users get partitioned, timestamped
  outputs (`s3://bucket/{{yyyy}}/{{mm}}/{{dd}}/{{workflow_name}}.ndjson`)
  without scripting.

## Future enhancements (out of scope)

- Excel `.xlsx` export with column types.
- Google Sheets / Airtable direct-write output targets.
- Presigned-URL result sharing that expires.
- Email / Slack digest of a run result.
- Result diffing across runs.
- Delta-only output (only rows that changed since the last run).
- Compression choices (gzip, zstd) with per-node opt-in.

## Cross-references

- [FR-06 Ad-Hoc Data Exploration](./FR-06-ad-hoc-exploration.md) —
  the surface most exports originate from.
- [FR-07 Visual Workflow Builder](./FR-07-visual-workflow-builder.md) —
  the workflow whose `output` node writes to a delivery target.
- [FR-10 Audit Logging & Compliance](./FR-10-audit-logging.md) —
  the audit trail that records every export.
- [FR-11 Observability, Guardrails & Reliability](./FR-11-observability-and-guardrails.md) —
  the guardrails that cap row and byte counts on export.
