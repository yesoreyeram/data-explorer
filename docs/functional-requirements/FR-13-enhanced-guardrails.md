# FR-13 — Enhanced App Guardrails & Payload Safety

## Overview

Data Explorer already ships with hard runtime limits (see FR-11): a 25MB
outbound response cap, 5-redirect cap, 20-page pagination cap, 100K-row
per-node cap, 2-minute run timeout, 60-second per-node timeout, and 200
node / 500 edge structural caps. This FRD **extends the guardrail story
end-to-end** — adding user-facing controls, safer parsing, streaming
ingestion, memory pressure signals, and warn-before-load behaviours so
that a legitimate large dataset is handled gracefully instead of merely
being rejected.

The goal of this FRD is twofold:

1. **Harden existing guardrails**: give users clearer feedback,
   consistent surfaces, and configurable overrides where safe, so the
   existing limits feel *helpful* instead of *frustrating*.
2. **Extend guardrails into new dimensions**: parser depth limits,
   column-count caps, cell-size caps, JSON/CSV streaming parse,
   compressed-payload decompression bombs, and per-user quotas that
   don't exist today.

This FRD contains **10 stories on existing functionality** (rehardening
paths already in the product) plus contributes further stories to
FR-14 / FR-15 / FR-16 for genuinely new capability.

## Product goals

- Never let a single request, workflow, or export take down the
  platform — extend "hard limits" from the request layer down into
  the parser, into the frame builder, and up into the UI.
- Turn every limit into a **teachable moment**: whenever a limit
  fires, tell the user *what* fired, *why* it fired, and *what to do
  next*.
- Add **soft warnings before hard rejections** so a user can see
  "this response is 22MB — near the 25MB cap" before it explodes
  into an error.
- Guard against **maliciously-crafted payloads** (deeply nested JSON,
  gzip bombs, oversized cells) — not just accidental oversize.
- Make guardrails **observable**: every trip of a limit records a
  metric and an audit event.
- Where safe, allow admins to **tune limits per deployment** without
  a code change.

## User personas

| Persona                    | Description                                                                                     |
| -------------------------- | ----------------------------------------------------------------------------------------------- |
| **Analyst**                | Runs a query that returns a lot of rows; wants clear guidance when a cap fires.                 |
| **Editor**                 | Wires a workflow whose source may occasionally spike; wants a preview of size before running.   |
| **SRE / Operator**         | Runs the platform; needs to tune limits to their traffic profile without forking the code.      |
| **Security engineer**      | Wants to know that a malicious payload cannot exhaust memory, CPU, or file descriptors.         |
| **Admin**                  | Wants per-user or per-role quotas so one heavy user doesn't degrade the platform for others.    |
| **Curious first-time user**| Wants the product to explain what's happening rather than fail silently.                        |

## User stories — **existing functionality (10)**

These stories refine or extend guardrails that already exist in the
product. Each story references the FRD/section the guardrail lives in
today so the enhancement path is unambiguous.

- **US-13.1** *(refines FR-11.5, 25MB body cap.)* As an editor, I want a
  **warning banner at 80% of the response-body cap** during Explore /
  workflow run so I can narrow my query before I hit the hard 25MB
  ceiling.
- **US-13.2** *(refines FR-11.6, 100K row cap.)* As an analyst, I want
  the "row cap reached" banner to include a **one-click "Add filter
  node"** action that seeds the workflow builder with a filter step so
  I can trim rows without a manual re-authoring loop.
- **US-13.3** *(refines FR-05 error taxonomy.)* As a user, I want every
  guardrail trip to classify to one of the existing 8 error codes
  (`invalid_config`, `timeout`, `rate_limited`, …) with a **remediation
  string that names the exact limit**, not a generic "request too
  large" message.
- **US-13.4** *(refines FR-07.5, per-node timeout.)* As an editor, I
  want the per-node timeout indicator to render on the canvas
  **counting down** while the node runs, so I can tell whether a node
  is "slow" or "stuck" before the timeout fires.
- **US-13.5** *(refines FR-08.6, overlap policy.)* As an ops engineer,
  I want the scheduler's skipped-run reasons visible on the workflow
  detail page as **a sparkline of skips over time**, so I can see
  chronic overlap without opening the audit log.
- **US-13.6** *(refines FR-11.7, rate limits.)* As an editor, I want
  the 429 rate-limit response to include the **exact quota, current
  count, and window** in the response body (JSON) so my client code
  can back off intelligently.
- **US-13.7** *(refines FR-09.1, export formats.)* As an analyst, I
  want to be told **before** clicking Export CSV that my visible frame
  will exceed 10K rows so I can choose "Export first 10K" vs "Refine
  query" instead of downloading a truncated file with a small banner.
- **US-13.8** *(refines FR-06.3, Explore run.)* As an analyst, I want
  the Explore result grid to **stream in the first 500 rows within
  200ms of the server responding**, and continue streaming, so I can
  start reading immediately on large results.
- **US-13.9** *(refines FR-07.5, node row cap.)* As an editor, I want
  a **row-count preview badge** on each node in the workflow canvas
  after a run, so I can see which node is close to the 100K cap
  without opening the run panel.
- **US-13.10** *(refines FR-11.9, graceful shutdown.)* As an SRE, I
  want SIGTERM handling to expose a **`/status/shutdown` endpoint**
  during the drain window that returns 503 with a JSON body describing
  in-flight runs and their remaining time, so my load balancer can
  make informed routing decisions.

## User stories — **new features (10)**

These stories introduce genuinely new guardrail dimensions or new
user-visible surfaces around guardrails.

- **US-13.11** As a security engineer, I want a **JSON parse depth
  cap** (default 64) so a deeply nested response can't cause stack
  exhaustion during unmarshalling.
- **US-13.12** As a security engineer, I want a **JSON parse element
  cap** (default 5,000,000 tokens) so a payload with millions of
  small elements can't wedge the parser even under the 25MB body
  cap.
- **US-13.13** As a security engineer, I want a **compressed-payload
  decompression cap** (default: reject if the compression ratio
  exceeds 100:1) so a "zip bomb" served over `Content-Encoding: gzip`
  cannot inflate to gigabytes.
- **US-13.14** As an editor, I want a **frame column cap** (default
  512 columns) so a REST response with a runaway `additionalProperties`
  can't produce a frame with 100k columns.
- **US-13.15** As an editor, I want a **per-cell size cap** (default
  1 MB per string cell, 5 MB per bytes cell) so a single "attachment"
  column cannot dominate memory for a whole frame.
- **US-13.16** As an admin, I want a **per-user quota** for Explore
  runs per hour and workflow executions per hour, configurable per
  role, so a single power user doesn't monopolize the platform.
- **US-13.17** As an admin, I want to **override the platform-wide
  limits** (body cap, row cap, redirect cap, page cap, run timeout,
  per-node timeout) via a JSON config file loaded at boot, so I can
  match the platform to my infrastructure without recompiling.
- **US-13.18** As an SRE, I want a **memory-pressure kill switch**
  that stops accepting new workflow runs when process RSS exceeds a
  configured high-water mark, and resumes when it drops below the
  low-water mark, so the platform sheds load before OOM instead of
  after.
- **US-13.19** As an editor, I want a **CSV / NDJSON streaming
  parser** for blob-store source nodes so a 500MB object can be
  ingested one row at a time up to the 100K row cap, instead of being
  buffered in memory.
- **US-13.20** As an admin, I want a **guardrails dashboard tile** on
  the admin dashboard showing the 24-hour count of trips per limit
  type, so I can see whether any limit is chronically firing and worth
  raising.

## Functional requirements

### FR-13.1 — Soft-warning threshold

Every hard limit that is expressible in a numeric quantity SHALL
have an accompanying **soft threshold at 80%** of the hard limit.
Crossing the soft threshold SHALL:

- emit a `guardrail.soft_warning` metric with the limit name and
  ratio;
- append a warning to the run/result surface visible to the user;
- NOT abort the operation.

### FR-13.2 — Uniform remediation strings

The remediation string on every guardrail-triggered error SHALL
include (a) the human-readable name of the limit, (b) the numeric
threshold, (c) the actual measured value, and (d) a next-step
suggestion (`"Add a filter node"`, `"Reduce page size"`, etc.).

### FR-13.3 — Rate-limit body

When the platform returns 429, the response body SHALL be JSON
with fields `code = "rate_limited"`, `quota`, `used`, `window_ms`,
`retry_after_ms`, and `remediation`. The `Retry-After` header
SHALL still be set for HTTP-client compatibility.

### FR-13.4 — JSON parser hardening

The shared JSON parser used by REST/GraphQL connectors SHALL enforce:

- **Depth ≤ 64** — nesting deeper than 64 aborts with
  `invalid_config` and remediation citing the depth cap.
- **Token count ≤ 5,000,000** — parsing more than 5M tokens aborts.
- **String cell size ≤ 1 MB** — any single string value beyond 1MB
  aborts.

Values are configurable at boot via the guardrail config file
(FR-13.10).

### FR-13.5 — Decompression bomb protection

When an outbound response uses `Content-Encoding: gzip`, `deflate`,
or `br`, the client SHALL:

- refuse to inflate if the reported `Content-Length` × 100 would
  exceed the 25MB body cap, without reading the body;
- stop inflating and abort with `invalid_config` if the inflated
  size exceeds 100× the compressed bytes read so far;
- record a `guardrail.decompression_ratio_exceeded` metric.

### FR-13.6 — Frame column and cell caps

Frame construction SHALL enforce:

- **Column cap** — default 512 columns per frame; abort with
  `invalid_config` beyond this.
- **String cell cap** — default 1MB per string cell; the parser
  truncates strings above the cap and emits a warning tag on the
  node result.
- **Bytes cell cap** — default 5MB per binary cell; hard reject
  above this cap.

### FR-13.7 — Streaming ingestion

The following source paths SHALL use streaming ingestion — rows
enter the frame builder as they are parsed, and the pipeline aborts
as soon as any cap is exceeded:

- CSV and NDJSON blob-store objects (S3, GCS, Azure Blob).
- REST responses whose `Content-Type` is `application/json`
  containing a top-level array whose element count is unknown in
  advance.
- SQL cursor-based results from Postgres and MySQL.

Streaming ingestion SHALL bound peak memory to the row-cap × the
average row size, plus a small constant buffer, not to the full
payload size.

### FR-13.8 — Per-user quotas

Each role SHALL have an optional per-hour quota for:

- Explore runs (`explore_runs_per_hour`, default unlimited).
- Workflow executions (`workflow_runs_per_hour`, default
  unlimited).
- Bytes exported (`export_bytes_per_hour`, default unlimited).

When a user exceeds the quota, the platform SHALL return 429 with a
`rate_limited` payload naming the quota. Admins SHALL see quota
utilization in the user detail view.

### FR-13.9 — Memory-pressure kill switch

The server SHALL sample its own RSS periodically (default every 10
seconds). When RSS exceeds a configured high-water mark:

- new workflow runs SHALL be rejected with `rate_limited` and a
  remediation "Platform under memory pressure — retry in a moment";
- in-flight runs continue.

When RSS drops below the configured low-water mark, new runs are
accepted again. A `guardrail.memory_pressure_active` gauge SHALL
be exported.

### FR-13.10 — Configurable limits

A boot-time configuration file (`GUARDRAILS_CONFIG_PATH` env, JSON
schema documented in `docs/DEVELOPER_GUIDE.md`) SHALL allow admins
to override defaults for:

- `http_response_body_bytes_max` (default 26_214_400)
- `http_redirects_max` (default 5)
- `http_pages_max` (default 20)
- `json_depth_max` (default 64)
- `json_tokens_max` (default 5_000_000)
- `frame_rows_max` (default 100_000)
- `frame_columns_max` (default 512)
- `cell_string_bytes_max` (default 1_048_576)
- `cell_bytes_bytes_max` (default 5_242_880)
- `run_wallclock_seconds_max` (default 120)
- `node_wallclock_seconds_max` (default 60)
- `decompression_ratio_max` (default 100)
- `memory_high_watermark_bytes`
- `memory_low_watermark_bytes`
- Per-role quotas.

Any value SHALL be validated at boot; invalid values SHALL prevent
startup with a clear error.

### FR-13.11 — Guardrails dashboard

The admin Dashboard SHALL expose a **Guardrails** tile with a
24-hour rolling count of trips per limit type. Clicking the tile
opens a detail view with a sparkline per limit and a
sample-of-latest-trips list.

### FR-13.12 — Audit trail

Every guardrail hard-trip SHALL emit an audit event
`system.guardrail_tripped` with fields `limit_name`,
`limit_value`, `observed_value`, `resource_type`, `resource_id`,
and `actor_id`.

### FR-13.13 — Countdown UI on running nodes

Workflow-run node cards SHALL render a per-node countdown from the
node's timeout budget, updating at 1 Hz, so users can see how much
of the budget is left.

### FR-13.14 — Row-count preview on canvas

After a workflow run completes, each node SHALL display a badge
with the row count of its output frame. A row-count near the cap
(≥ 80%) SHALL be colored amber; at the cap, red.

### FR-13.15 — Add-filter action from row-cap banner

When the row-cap warning appears on a run result, the banner SHALL
offer a **"Add filter node"** button which, when clicked, opens the
workflow builder with a pre-populated filter node between the
capped node and its downstream target(s).

## UI/UX requirements

- Soft-warning banners use the `--status-warning` token (yellow
  dot + amber background) with a clear "80% of limit" wording.
- Hard-limit errors use the `--status-danger` token (red dot) and
  never look like a soft warning.
- Countdown timers on node cards use a monospace font to avoid
  digit-width jitter.
- The guardrails dashboard tile follows the `StatTile` primitive
  conventions from FR-12.
- Rate-limit toasts include a live countdown to the `retry_after`
  time.
- Export "would-be-truncated" prompt uses the standard `Modal`
  primitive with two buttons: **Export first 10K** (primary) and
  **Refine query** (secondary).

## Acceptance criteria

- [ ] A REST response reaching 21MB during ingestion (84% of the
  25MB cap) surfaces a soft-warning banner without aborting.
- [ ] A REST response exceeding 25MB aborts with error code
  `invalid_config` and a remediation string containing the exact
  cap value and observed size.
- [ ] A GET whose upstream sends 30MB after `Content-Length:
  200000` and `Content-Encoding: gzip` aborts with
  `invalid_config` naming the decompression-ratio cap.
- [ ] A workflow with a source node whose output would exceed 512
  columns aborts with `invalid_config` naming the column cap.
- [ ] A workflow node running longer than 45s (75% of the 60s cap)
  shows an on-canvas countdown with amber styling; at 60s the
  node fails with `timeout` and the run stops.
- [ ] A user whose role limits Explore runs to 60/hour receives 429
  on the 61st Explore run in that window, with a JSON body naming
  the quota.
- [ ] After a run, every node card shows its output row count as a
  badge; nodes at ≥ 80K rows are amber; at 100K rows, red.
- [ ] The row-cap warning banner offers an "Add filter node"
  button that opens the builder pre-populated correctly.
- [ ] The admin dashboard shows a Guardrails tile with a 24-hour
  count of guardrail trips per limit.
- [ ] Every hard-trip records a `system.guardrail_tripped` audit
  event with the required fields.
- [ ] Setting `frame_rows_max` = 50000 in the config file limits
  frame builds to 50K rows without a code change.
- [ ] Invalid config values prevent startup with a clear message
  naming the offending field.

## Edge cases & error handling

- **Streaming abort mid-response**: When a hard cap trips during
  streaming, the partial frame SHALL be discarded — no partial
  results reach the run panel.
- **Compressed payload with unknown Content-Length**: The client
  applies the decompression-ratio check every 1 MB read; excess
  aborts without waiting for the stream to end.
- **Config override lower than a hardcoded floor**: The platform
  refuses to start if any override is below a documented safety
  floor (e.g. can't set `run_wallclock_seconds_max` to 1 second).
- **Race between quota reset and request**: Quotas are enforced
  atomically per user + role via a rolling window; a burst around
  the reset boundary is deterministic (never counts a request
  twice, never misses one).
- **Row-count badge on failed run**: Nodes that never executed
  show a neutral "—" badge, not a zero.
- **Memory-pressure flapping**: The high- and low-water marks must
  differ by at least 10% to prevent flapping; the config
  validator enforces this.
- **User-specific quota disabled**: A role with no quota
  configuration inherits unlimited; a role with `0` explicitly
  configured blocks all runs (documented, intentional).
- **Streaming source that reports total rows in-header**: If the
  source announces a row count exceeding the row cap, the run
  fails fast before any parse.
- **JSON parser hitting depth cap on a valid file**: The user is
  given a remediation to raise the depth cap in the config file
  (with a warning about the trade-off).

## Non-functional requirements

- **Overhead**: Guardrail checks (per-token, per-cell) SHALL add
  ≤ 5% CPU overhead to parsing/ingestion measured on a benchmark
  suite.
- **Observability**: Every limit SHALL have a counter
  (`guardrail_trip_total{limit}`) and a histogram
  (`guardrail_observed_ratio{limit}`).
- **Safety floors**: The platform SHALL refuse to boot with any
  guardrail set below a documented safety floor.
- **Auditability**: Every hard trip is one audit event, one metric
  observation, and one log line with the same request-id.
- **Backward compatibility**: Adding a new guardrail SHALL default
  to a non-restrictive value on upgrade so existing users are not
  surprised.
- **Deterministic classification**: The same overflow SHALL always
  classify to the same error code with the same remediation string
  across the API surface.

## Market context & differentiation

| Product           | Guardrail story                                                          | Notes                                                             |
| ----------------- | ------------------------------------------------------------------------ | ----------------------------------------------------------------- |
| **Retool**        | App-level limits; runtime limits per query                                | Focused on request-time; less on frame/parse depth.               |
| **n8n**           | Execution timeout + node config                                          | User-configurable; less taxonomy uniformity.                      |
| **Airflow**       | Task timeouts, DAG concurrency; XCom size limits                          | Powerful; entirely operator-configured.                           |
| **Airbyte**       | Row / byte / column caps per sync                                        | Sync-oriented; less about ad-hoc.                                 |
| **Metabase**      | Query timeouts + max rows per query                                       | Coarse-grained.                                                   |
| **Superset**      | `SQL_MAX_ROW`, query timeout                                              | Config-file driven; limited surfaces.                             |
| **Postman**       | Response-size warnings                                                    | UI only; not scriptable.                                          |
| **jq / DuckDB**   | Memory / recursion limits                                                 | CLI tools; user must know.                                        |

Data Explorer's differentiators for enhanced guardrails:

- **Two-tier limits.** Soft warnings at 80%, hard limits at 100%,
  applied uniformly.
- **Parser-depth and token limits.** Not just body size — the
  JSON parser itself is bounded.
- **Decompression bomb defence.** Compression-ratio checks on
  every gzipped response.
- **Per-cell caps.** A single string cell can't dominate a frame.
- **Streaming ingestion.** Bound memory to the row-cap, not the
  payload size.
- **Per-user, per-role quotas.** Fair sharing without ops
  intervention.
- **Memory-pressure kill switch.** Shed load before OOM.
- **Config-file overrides with safety floors.** Ops can tune
  without recompiling — but cannot foot-gun.
- **Guardrails dashboard.** Trip counts and sparklines built in.
- **Uniform audit event for every trip.** Every cap fire is a
  first-class event.

## Future enhancements (out of scope)

- Adaptive limits that self-adjust based on rolling p95 usage.
- Per-connection cost-aware caps (e.g. "1 Athena scan-cost budget
  per user per day").
- Automatic query-cost estimation with pre-run "this looks
  expensive" prompts.
- Server-side result caching to reduce repeat-query cost.
- Per-workflow parallelism ceilings.
- Row-sampling mode where a source returns a representative sample
  when it would exceed the row cap.

## Cross-references

- [FR-05 Connection Health Monitoring](./FR-05-connection-health-monitoring.md) — shared error taxonomy.
- [FR-06 Ad-Hoc Data Exploration](./FR-06-ad-hoc-exploration.md) — streaming ingestion, row-cap prompts.
- [FR-07 Visual Workflow Builder](./FR-07-visual-workflow-builder.md) — countdown UI, row-count badges.
- [FR-08 Workflow Scheduling & Automation](./FR-08-workflow-scheduling.md) — skips sparkline.
- [FR-09 Query Result Export & Sharing](./FR-09-query-result-export.md) — export-truncation prompt.
- [FR-10 Audit Logging & Compliance](./FR-10-audit-logging.md) — `system.guardrail_tripped` event.
- [FR-11 Observability, Guardrails & Reliability](./FR-11-observability-and-guardrails.md) — the underlying guardrail platform this FRD extends.
- [FR-16 Data Frame Optimizations](./FR-16-data-frame-optimizations.md) — streaming and columnar work that this FRD's guardrails guard.
