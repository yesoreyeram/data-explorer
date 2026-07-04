# FR-16 — Data Frame Optimizations

## Overview

Data Explorer's `Frame` type is the shared, tabular contract every
connector, transform, join, aggregate, and output node uses (see
`pkg/dataframe` and FR-07). Today the Frame is a straightforward
row-oriented representation with per-column schema. That works for the
100K-row cap enforced by the guardrail layer (FR-11, FR-13) but leaves
substantial gains on the table when it comes to memory, latency,
serialization cost, and interchange with other tools (pandas,
DuckDB, Arrow-based ecosystems).

This FRD proposes a **package of Frame optimizations** — some tighten
existing behaviour (memory, hashing, JSONata perf), some add new
capability (Arrow interchange, columnar storage, lazy evaluation,
projection push-down, categorical dictionaries). Every proposed change
is **user-visible** in latency or capability, not merely an internal
refactor.

## Product goals

- Make the platform **feel snappier** across every surface that
  handles frames: Explore, workflow run panel, visualization,
  export.
- Enable **larger effective row counts** without raising the hard
  100K cap: through columnar storage, categorical columns, and
  streaming pipelines, the same cap holds more useful data per byte.
- Provide **interchange with the wider data ecosystem**: Arrow /
  Parquet input & output, so users can hand a frame to a pandas or
  DuckDB consumer without re-serializing.
- Make transforms **push predicates and projections down** to
  sources where possible, so the platform doesn't waste bytes
  fetching data it will immediately drop.
- Preserve **backwards compatibility** — no existing user-facing
  behaviour breaks; changes are performance and capability
  extensions.

## User personas

| Persona                    | Description                                                                                          |
| -------------------------- | ---------------------------------------------------------------------------------------------------- |
| **Analyst**                | Wants Explore results to render faster.                                                              |
| **Editor**                 | Wants workflow runs to complete faster, especially on large source frames.                           |
| **Ops / SRE**              | Wants lower memory and CPU per frame so the platform scales further on the same hardware.            |
| **Data engineer**          | Wants Arrow / Parquet in/out so Data Explorer plugs into pandas / DuckDB workflows.                  |
| **Colleague downstream**   | Consumes exported files; would rather receive Parquet (efficient) than 500MB CSV.                    |
| **Frontend visualizer**    | Renders charts; benefits from smaller frame payloads over the wire.                                  |

## User stories — **existing functionality (10)**

These stories refine or extend Frame-adjacent behaviours that already
exist today.

- **US-16.1** *(refines FR-06 Explore.)* As an analyst, I want the
  Explore result to be **transferred to the browser in a compact
  binary form** instead of a JSON payload, so a 10K-row frame is on
  screen faster and the network transfer is smaller.
- **US-16.2** *(refines FR-07 workflow engine.)* As an editor, I
  want workflow nodes to **share Frame column buffers** where
  possible (copy-on-write), so a 10-node linear pipeline doesn't
  duplicate the same column values 10 times in memory.
- **US-16.3** *(refines FR-09 export.)* As an analyst, I want the
  **CSV / JSON export** to stream directly from the columnar
  frame representation rather than materializing an intermediate
  row list, so a 10K-row export uses bounded memory.
- **US-16.4** *(refines FR-07 filter node.)* As an editor, I want
  a **filter node** whose predicate compares against a single
  column to short-circuit on the column's min/max metadata, so
  a filter with an out-of-range predicate returns empty instantly
  without scanning rows.
- **US-16.5** *(refines FR-07 aggregate node.)* As an editor, I
  want the **aggregate node** to sort by group key in one pass
  when the input is already sorted on that key, so common
  time-bucket aggregations complete faster.
- **US-16.6** *(refines FR-07 join node.)* As an editor, I want
  the **join node** to build a hash on the smaller side of the
  join automatically, so I don't have to think about join order.
- **US-16.7** *(refines FR-07 transform node.)* As an editor, I
  want **JSONata expressions** to be **compiled once** at run
  start and re-used across rows, so I don't pay parse cost per
  row.
- **US-16.8** *(refines FR-06 Explore.)* As an analyst, I want the
  Explore recent-queries entry's stored **query hash** to remain
  stable across whitespace-only edits, so cache hits work as
  expected.
- **US-16.9** *(refines FR-05 error taxonomy.)* As a user, I want
  **out-of-memory during frame materialization** to classify to
  the existing `invalid_config` (with remediation "frame exceeds
  configured limits — narrow your query") instead of surfacing as
  a generic panic.
- **US-16.10** *(refines FR-11 metrics.)* As an SRE, I want new
  metrics `frame_bytes` and `frame_rows` histograms per source
  type and per node type, so I can see who consumes the most
  memory.

## User stories — **new features (10)**

- **US-16.11** As a data engineer, I want workflow **output nodes
  to write Parquet** to blob stores, so downstream pandas/DuckDB
  consumers get an efficient columnar file.
- **US-16.12** As a data engineer, I want workflow **source nodes
  to read Parquet** from blob stores, so I can chain Data
  Explorer with other tools that write Parquet.
- **US-16.13** As an SRE, I want the Frame to have a **columnar
  in-memory representation** (typed arrays per column) so that
  operations that touch one column (filter, aggregate on a
  measure) don't stride over unrelated column data.
- **US-16.14** As an editor, I want a **projection push-down**
  optimizer that discards columns not used by any downstream
  node before fetching, so a `SELECT *` source pruned to two
  columns by a downstream transform doesn't fetch the other
  columns.
- **US-16.15** As an editor, I want a **predicate push-down**
  optimizer that pushes simple filter predicates into SQL
  sources (as `WHERE`) and REST sources (as query parameters
  from a mapping table), so filtering happens where the data
  lives.
- **US-16.16** As an editor, I want **categorical (dictionary-
  encoded) columns** for string columns with few distinct
  values, so a 100K-row × 5-column frame with heavily repeated
  categories uses a fraction of the memory.
- **US-16.17** As an editor, I want **lazy evaluation** for a
  chain of pure transforms (filter → transform → filter) so the
  engine can fuse them into a single pass without allocating
  intermediate frames.
- **US-16.18** As a data engineer, I want the frame to expose an
  **Arrow-compatible interchange** endpoint (Arrow IPC over
  HTTP) so external tools can attach directly.
- **US-16.19** As an editor, I want a **row-group / batching**
  mode for source nodes so a large source can be processed in
  chunks that stay within memory caps.
- **US-16.20** As an analyst, I want **automatic type inference**
  from source data (numeric vs date vs string), with an override
  in the source node config, so I don't have to teach every
  chart or transform what a column is.

## Functional requirements

### FR-16.1 — Columnar in-memory Frame

The `Frame` type SHALL support a columnar representation:

- Each column stores its values in a typed slice / array
  matching its declared type (`Int64`, `Float64`, `Bool`,
  `String`, `Time`, `Bytes`, `Null`).
- Nulls SHALL be tracked via a per-column bitmap.
- Row iteration (still supported for compatibility) SHALL be
  implemented as a projection over the columnar store.

The existing row-oriented API SHALL continue to work for any
consumer that has not yet migrated; internally it wraps the
columnar store.

### FR-16.2 — Categorical (dictionary) columns

String columns whose distinct-value count is ≤ 128 SHALL
automatically use a **dictionary encoding**: a shared dictionary
of unique values plus a per-row `uint8` / `uint16` index. The
encoding SHALL be transparent to consumers (`GetString(row)`
returns the string).

The threshold SHALL be tunable via the guardrail config file
(FR-13.10).

### FR-16.3 — Copy-on-write column buffers

Frame column buffers SHALL be reference-counted. A downstream
node that consumes an unchanged column SHALL share the buffer
with its upstream node until (and unless) the column is modified,
at which point a copy is materialized.

### FR-16.4 — Lazy fusion of pure transforms

The workflow engine SHALL detect linear chains of pure operators
(`filter`, `transform`, `filter`) with no fan-in/fan-out between
them and SHALL fuse them into a single physical pass over the
upstream frame. Users SHALL see the effect as reduced latency
and reduced peak memory; the run panel SHALL continue to show
each logical node's frame preview via a snapshot after the fused
pass.

### FR-16.5 — Projection push-down

Before executing a workflow, the engine SHALL compute the set
of columns each downstream node reads. Source nodes SHALL be
issued a **projection hint** naming the columns actually needed.
Source nodes that can honour projection (SQL, Athena, BigQuery,
Parquet) SHALL rewrite their query / read plan to fetch only
those columns.

### FR-16.6 — Predicate push-down

Filter nodes whose predicates are **conjunctions of simple
comparisons** (`col op literal` where `op` is `=`, `!=`, `<`,
`<=`, `>`, `>=`, `in`) SHALL be candidates for push-down:

- SQL sources: pushed as `WHERE` clauses.
- Athena / BigQuery / MySQL / Postgres: same.
- REST sources with a declared parameter mapping in their
  connection config: pushed as query parameters.
- Others: not pushed; filter runs on the engine side as today.

Push-down SHALL be **safe by construction**: the engine SHALL
still enforce the predicate after fetching, so a source that
ignores a push-down hint does not violate the intended filter.

### FR-16.7 — Parquet output

The `output` node SHALL support **Parquet** as a format option
for blob-store targets (S3 / GCS / Azure Blob). Parquet output
SHALL:

- Preserve column types (including nullability).
- Use Snappy compression by default (Zstd optional).
- Write a single row group unless configured otherwise.
- Emit a `guardrail.frame_bytes` metric with the output byte
  size.

### FR-16.8 — Parquet input

Blob-store `source` nodes SHALL support **Parquet** as a format:

- Read column types from the Parquet schema.
- Honour the projection push-down hint by reading only requested
  columns.
- Respect the 100K-row / 512-column caps at read time.

### FR-16.9 — Arrow IPC interchange endpoint

The API SHALL expose `GET /api/v1/frames/{token}` returning the
frame in **Arrow IPC** format when the client sends
`Accept: application/vnd.apache.arrow.stream`. Falls back to JSON
otherwise for backwards compatibility.

The `{token}` is a short-lived, RBAC-checked, single-use handle
issued when a run completes or an Explore query succeeds.

### FR-16.10 — Compact wire format for the browser

The frontend SHALL be able to fetch frames as **Arrow IPC**
when the browser and server negotiate it, decoding via
`apache-arrow` in the browser. The `DataFrameView` SHALL render
from the Arrow-backed frame with no extra copy.

For older browsers or when Arrow is not available, the API SHALL
fall back to JSON.

### FR-16.11 — Batched (streaming) source nodes

Source nodes that support streaming SHALL emit frames in
**row-group batches** (default 10,000 rows per batch) rather
than one whole frame. Downstream nodes MAY process each batch
independently. The output node MAY concatenate batches into a
single artifact or write one file per batch (config-selected).

### FR-16.12 — Automatic type inference

Frame construction SHALL infer column types from the first N
non-null values (default 1000) and SHALL be overridable via the
source-node config (`columnTypes: {colName: "int" | "float" |
"date" | "datetime" | "bool" | "string" | "bytes"}`). Type
conflicts encountered later in the stream SHALL be reported as
`invalid_config` naming the row and column.

### FR-16.13 — Query hash stability

The Explore recent-queries entry's `query_hash` SHALL be computed
from a **normalized** representation of the query (whitespace
collapsed, trailing semicolons removed) so semantic-equivalent
edits collide. Comments SHALL be preserved verbatim in the
stored query text but stripped for hashing.

### FR-16.14 — Metrics

New Prometheus metrics SHALL be exported:

- `frame_rows{source}` — histogram of frame row counts per
  source-node type.
- `frame_bytes{source}` — histogram of frame byte size per
  source-node type.
- `frame_columns{source}` — histogram of frame column counts.
- `frame_construction_seconds{source}` — histogram of frame
  construction latency.
- `frame_categorical_columns_total` — counter of columns
  auto-promoted to dictionary encoding.
- `frame_pushdown_projection_total` — counter of projection
  push-downs applied.
- `frame_pushdown_predicate_total` — counter of predicate
  push-downs applied.

### FR-16.15 — Backward compatibility guarantee

- All existing Frame APIs SHALL continue to work; migrations
  are internal.
- All existing workflows SHALL run without change; existing
  saved definitions SHALL parse and execute identically (though
  possibly faster).
- All existing exports SHALL continue in their existing formats
  (CSV, JSON, NDJSON); Parquet is an addition, not a
  replacement.
- All existing screenshots and UI copy SHALL continue to apply;
  new UI (Parquet option in the output-node dropdown, type
  override in source-node config) is additive.

## UI/UX requirements

- The output-node config panel adds **Parquet** as a format
  option alongside CSV / JSON / NDJSON.
- The source-node config panel gains a **Column types**
  optional table where users can override inferred types
  (auto-populated with the inferred types after a first run).
- The workflow run panel node card gains a small **"Fused"**
  chip on nodes that were fused by the optimizer, hoverable to
  see which nodes were fused together.
- Push-down actions are surfaced on the source node's run
  result: "Projection pushed: 3 of 10 columns fetched",
  "Predicate pushed: 1 filter applied at source". This is
  informational and non-blocking.
- The Explore recent-queries entry preview trims whitespace
  and shows a `~` indicator when two entries collide on hash
  (offering a "Show variants" affordance).

## Acceptance criteria

- [ ] A 10K-row frame with a string column of 5 distinct values
  uses at most **20%** of the memory of a naive string-slice
  representation.
- [ ] A `filter → transform → filter` chain on the same frame
  runs faster and allocates fewer intermediate frames than the
  same chain without fusion (measured by a benchmark test).
- [ ] A source node returning `SELECT * FROM t LIMIT 100`
  followed by a downstream transform that reads only 2 columns
  results in the source executing `SELECT col1, col2 FROM t
  LIMIT 100` (verified by a query-capture test).
- [ ] A filter with predicate `status = 'open'` on a Postgres
  source is pushed as a `WHERE` clause.
- [ ] Selecting Parquet in the output node writes a valid
  Parquet file readable by pandas and DuckDB.
- [ ] A Parquet source node reads a file written by the same
  platform, preserving all column types.
- [ ] Requesting a frame with `Accept:
  application/vnd.apache.arrow.stream` returns Arrow IPC bytes.
- [ ] The browser negotiates Arrow when supported and renders
  the frame with no observable diff versus the JSON path.
- [ ] Streaming a 500MB CSV source through the platform never
  exceeds the configured peak memory bound.
- [ ] Type inference correctly identifies integer / date /
  string columns on a mixed frame; overriding a column type in
  the source config takes effect on the next run.
- [ ] `frame_rows`, `frame_bytes`, `frame_columns`, and
  `frame_construction_seconds` metrics appear on `/metrics`
  with labels by source type.
- [ ] Push-down counters increment when the corresponding
  push-down is applied.
- [ ] Whitespace-only edits to an Explore query produce the
  same `query_hash`.
- [ ] Attempting to allocate a frame that would exceed the
  configured memory ceiling fails with `invalid_config` and the
  standard remediation string.
- [ ] All existing tests continue to pass without modification.

## Edge cases & error handling

- **Cardinality above the categorical threshold**: The column
  falls back to the raw string encoding; a metric records the
  fallback for visibility.
- **Predicate push-down semantically incorrect** (e.g. the
  source has different NULL semantics): The engine SHALL still
  apply the filter on the returned frame, so correctness is
  preserved regardless of source behaviour.
- **Parquet with unsupported types** (e.g. nested lists, maps,
  structs): The read fails with `invalid_config` naming the
  offending column and type.
- **Arrow IPC negotiation with a browser that doesn't support
  it**: The client falls back to JSON transparently.
- **Copy-on-write invariant violation**: An accidental mutation
  of a shared buffer would be a bug; the frame API surface
  SHALL not permit direct pointer access — all writes go
  through methods that check the reference count.
- **Fusion across a non-pure operator**: The engine SHALL NOT
  fuse across operators with side effects (e.g. an `output`
  node) or across joins/aggregates.
- **Streaming batch that exceeds row cap partway through**:
  The stream aborts at the cap; already-emitted downstream
  effects (e.g. rows written to Parquet) are captured up to
  that point with a warning.
- **Push-down with a REST source lacking a parameter mapping**:
  The push-down is skipped; the filter runs on the engine side
  as today.
- **Automatic type inference on an ambiguous column** (e.g. an
  ISO date string mixed with occasional numeric ids): The
  engine infers `string` (the safest choice) and offers a hint
  in the source-node UI to override.
- **Metric cardinality**: `frame_*` metrics are labelled by
  source *type* (e.g. `postgres`, `rest`), not per connection,
  to bound cardinality.

## Non-functional requirements

- **Memory footprint**: The columnar representation SHALL use
  ≤ 60% of the memory of the row-oriented representation for a
  representative benchmark suite (10K rows × 10 columns mixed
  types).
- **Latency**: Frame construction from a SQL source SHALL be
  ≤ 20% slower than today for small frames (< 1000 rows) and
  ≥ 20% faster for large frames (≥ 10K rows).
- **Interchange fidelity**: Parquet and Arrow output SHALL round-
  trip through pandas without column-type loss for supported
  types.
- **Safety**: All optimizations SHALL be **semantically
  transparent** — the outputs of any workflow SHALL be
  bit-identical before and after enabling the optimizer.
- **Observability**: Every optimization applied SHALL be
  reflected in a metric so operators can see whether the
  optimizer is engaging.
- **Correctness under concurrency**: Copy-on-write buffers SHALL
  be safe under concurrent read + copy operations (the workflow
  engine may execute independent branches in parallel).
- **Backwards compatibility**: JSON API and CSV / NDJSON export
  SHALL continue to work byte-identically to today.

## Market context & differentiation

| Product           | Frame / data representation                                          | Notes                                                                    |
| ----------------- | -------------------------------------------------------------------- | ------------------------------------------------------------------------ |
| **pandas**        | Columnar, ndarray-backed                                             | Standard for local analysis; no visual builder.                          |
| **Polars**        | Columnar, Arrow-backed, lazy                                          | Very fast; Python/Rust; no visual builder.                                |
| **DuckDB**        | Columnar, vectorized, SQL                                             | Excellent for local analytics; embedded.                                  |
| **Apache Arrow**  | Columnar interchange format                                          | Interop standard; the format we adopt for interchange.                    |
| **Airflow / n8n** | Row-oriented, JSON payloads                                          | Fine for orchestration; not optimized for tabular perf.                   |
| **Retool**        | JSON tables inside app runtime                                       | App-runtime performance; no columnar / Arrow story.                      |
| **Metabase / Superset** | SQL-first; frames only in-memory for display                    | Rely on the underlying DB for perf.                                       |
| **Airbyte**       | Row-oriented sync record model                                       | Not tabular / Frame-oriented.                                             |

Data Explorer's differentiators for Frame optimizations:

- **Columnar + dictionary encoding + copy-on-write** applied to
  the tabular contract every node in the product speaks.
- **Projection & predicate push-down** across a heterogeneous
  connector fleet, without a Python DSL.
- **Parquet in & out and Arrow IPC on the wire.** Interchange
  with the wider data ecosystem without ETL.
- **Lazy fusion of pure operators.** Visual workflows keep
  their intuitive per-node structure while running as a fused
  physical plan.
- **Streaming batches** — 500MB CSV through the platform without
  raising the row cap.
- **Automatic type inference with override.** Users don't fight
  the type system; power users can pin exact types.
- **Backwards-compatible.** All changes are performance /
  capability wins; no existing workflow breaks.
- **Every optimization is observable.** Operators see how often
  push-down and fusion engage.
- **Same guardrails apply.** Optimizations do not raise the
  100K-row / 512-column / memory / timeout caps.
- **Query-hash normalization.** Better recent-queries cache
  behaviour without user education.

## Future enhancements (out of scope)

- Query planning across multiple sources (federated joins in the
  source-selection phase).
- Materialized-view caching of Frame results with cache
  invalidation.
- Adaptive execution — re-planning mid-run when statistics
  disagree with estimates.
- DuckDB embedded as an in-process query engine over Frames.
- GPU-backed columnar operations.
- Compression of on-wire Arrow (`Content-Encoding: zstd`).
- Custom UDFs in a sandboxed runtime for transforms beyond
  JSONata.
- Time-series-specific optimizations (delta encoding, run-length
  encoding).
- Zero-copy pass-through when a source natively speaks Arrow
  (e.g. BigQuery Storage API).
- Multi-batch join algorithms that spill to disk.

## Cross-references

- [FR-06 Ad-Hoc Data Exploration](./FR-06-ad-hoc-exploration.md) — Explore uses the same frame.
- [FR-07 Visual Workflow Builder](./FR-07-visual-workflow-builder.md) — nodes exchange frames.
- [FR-09 Query Result Export & Sharing](./FR-09-query-result-export.md) — Parquet added here.
- [FR-11 Observability, Guardrails & Reliability](./FR-11-observability-and-guardrails.md) — new metrics land here.
- [FR-13 Enhanced App Guardrails](./FR-13-enhanced-guardrails.md) — guardrails wrap these optimizations.
- [FR-15 Dataset Visualization](./FR-15-dataset-visualization.md) — charts render Arrow-backed frames.
