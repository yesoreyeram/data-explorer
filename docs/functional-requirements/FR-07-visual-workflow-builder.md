# FR-07 — Visual Workflow Builder (Pipelines)

## Overview

The **Workflow Builder** is a canvas-based, drag-and-drop editor that lets
users assemble a directed acyclic graph (DAG) of typed nodes — `source`,
`filter`, `transform`, `join`, `aggregate`, `output` — connect them with
edges, configure each node in a side panel, and save the resulting
pipeline. When executed, the pipeline pulls data from one or more
connections, reshapes it in tabular form, and delivers it to a chosen
output (a downstream file/blob, another connection, or an inspectable
run result).

The builder is intentionally opinionated: every node consumes and
produces the same shape — a `Frame` (pandas-style rows-and-columns with
schema) — so users can freely rewire nodes without worrying about data
formats. Filters and transforms use JSONata expressions so the same
expression language works whether the source is SQL, REST, GraphQL, or
a cloud log query.

## Product goals

- Let a user assemble a repeatable data pipeline in the browser without
  writing Airflow DAGs, Argo manifests, or Kubernetes jobs.
- Make node behaviour *predictable*: every node type has one clear
  purpose and the same tabular contract.
- Enforce hard **safety guardrails** so a mis-configured or malicious
  pipeline cannot exhaust server memory, run forever, or cause
  cascading failure of unrelated pipelines.
- Give users a workflow-authoring loop that mirrors the "code → run →
  inspect → tweak" cadence they already know from notebooks and
  REPLs.
- Keep the on-disk representation of a workflow to a single JSON
  document so it can be exported, diffed, and re-imported.

## User personas

| Persona                      | Description                                                                                                    |
| ---------------------------- | -------------------------------------------------------------------------------------------------------------- |
| **Analyst**                  | Wants to move a working Explore query into a repeatable pipeline that runs on a schedule.                       |
| **Editor** (application dev) | Assembles multi-source pipelines: e.g. "pull orders from Stripe, join with users from Postgres, output to S3." |
| **Ops engineer**             | Builds monitoring pipelines that combine logs from multiple regions and aggregate them into daily rollups.     |
| **Viewer**                   | Reads existing workflows and inspects historical runs; cannot edit.                                            |
| **Admin**                    | Reviews and, if needed, quarantines pipelines that misbehave.                                                  |

## User stories

- **US-07.1** As an analyst, I want to drag source, filter, and output
  nodes onto a canvas and connect them, so I can build a pipeline
  without learning YAML.
- **US-07.2** As an editor, I want to configure each node in a
  side-panel form rather than editing a JSON blob, so I don't have to
  remember which fields belong to which node type.
- **US-07.3** As an analyst, I want to click **Run now** and see the
  output of every node in the pipeline, so I can debug where my data
  went wrong.
- **US-07.4** As an editor, I want the builder to reject cyclic
  pipelines (A → B → A) at save time, so I don't discover the mistake
  at run time.
- **US-07.5** As an ops engineer, I want to join two source frames on a
  key column with left/right/inner/outer joins, so I can enrich one
  data set with another.
- **US-07.6** As an analyst, I want to aggregate a source frame with a
  group-by and one or more aggregations (sum/avg/min/max/count), so I
  can produce daily/weekly rollups.
- **US-07.7** As an editor, I want to duplicate a saved workflow with
  one click so I can use it as a template for a similar pipeline.
- **US-07.8** As an admin, I want to see who last edited a workflow and
  when, so I can trace unexpected pipeline changes.
- **US-07.9** As an analyst, I want confidence that a broken pipeline
  will fail in bounded time, so my one-off mistake doesn't consume the
  server for hours.

## Functional requirements

### FR-07.1 — Workflow resource

A workflow SHALL be a stored resource with these persistent fields:
`id`, `name`, `description`, `definition` (JSON), `schedule` (nullable
cron string), `created_by`, `created_at`, `updated_by`, `updated_at`.

### FR-07.2 — Definition shape

The `definition` JSON SHALL contain two arrays:

- `nodes[]`: each entry has `id`, `type` (one of `source`, `filter`,
  `transform`, `join`, `aggregate`, `output`), `name`, `config`
  (per-type JSON), and an optional `position` (`x`, `y`) used to
  round-trip the canvas layout.
- `edges[]`: each entry has `id`, `source` (node id),
  `target` (node id), and an optional `targetHandle` (used by
  multi-input nodes like `join` — values `left` or `right`).

### FR-07.3 — Node types and contracts

Every node executor SHALL accept `map[string]*Frame` (keyed by target
handle, and by producing-node id for convenience) and produce a
single `*Frame`.

- **source** — pulls a frame from a connection using the same query
  shapes as Explore (SQL text, REST/GraphQL request, cloud request).
  Config includes `connectionId` and query-shape fields.
- **filter** — evaluates a JSONata predicate against every input row
  and passes rows where the predicate is truthy.
- **transform** — evaluates a JSONata expression that returns a new
  row shape (add columns, drop columns, rename columns, derive
  columns).
- **join** — joins the frame arriving on the `left` handle with the
  frame arriving on the `right` handle using `leftKey`, `rightKey`,
  and `joinType` (`inner`, `left`, `right`, `outer`).
- **aggregate** — group-by aggregation: `groupBy` (array of column
  names) and `aggregations` (array of `{column, op, alias}` where
  `op` is `sum`, `avg`, `min`, `max`, `count`).
- **output** — terminal node: writes the frame to an output target
  (blob storage object, a connection, or a run-result artifact
  visible in the Run panel). Config includes `target` and per-target
  fields.

### FR-07.4 — Structural validation

Save SHALL reject the workflow with a descriptive error when any of:

- A node id is missing or duplicated.
- An edge references a source or target node that does not exist.
- A node has an unknown `type`.
- The graph contains a cycle.
- Node count exceeds **200**.
- Edge count exceeds **500**.

### FR-07.5 — Execution guardrails

Execution SHALL be bounded by:

- **Per-node row cap**: any node's output frame SHALL be clamped at
  **100,000** rows. When clamped, the run marks the node with a
  warning tag but continues.
- **Per-run timeout**: the entire workflow run SHALL abort if it
  exceeds **2 minutes** wall-clock.
- **Per-node timeout**: any single node SHALL abort if it exceeds
  **60 seconds**.
- **Per-connection rate limit**: source nodes respect the same rate
  limit as the Connections page and the Explore surface.
- **Guardrails are enforced server-side**; the client cannot opt out.

### FR-07.6 — Run and inspect

The builder SHALL expose a **Run now** action which:

1. Persists any unsaved changes (with the user's confirmation).
2. Enqueues the run.
3. Streams intermediate node results to the UI as they complete, so
   the user can inspect the frame that each node produced.
4. Renders the final output frame in a **DataFrameView** grid.
5. Records the run as a `workflow_executions` row with `id`,
   `workflow_id`, `status` (`succeeded`, `failed`, `cancelled`,
   `timeout`), `started_at`, `finished_at`, `duration_ms`,
   `error_code`, `error_message`, `row_count`, `initiated_by`.

### FR-07.7 — Per-node result inspection

The builder SHALL let the user click any node to see the frame it
produced during the most recent run. If the node was never run or
produced no result (upstream error, skipped), the panel SHALL show a
tri-state indicator (pending, running, failed) and the classified
error taxonomy from FR-05.

### FR-07.8 — Save, duplicate, delete

- **Save** SHALL be gated by `workflows:write`.
- **Duplicate** SHALL create a new workflow with `" (copy)"` appended
  to the name, an empty schedule, and the same definition as the
  source workflow.
- **Delete** SHALL be gated by `workflows:write`, SHALL require an
  in-place confirmation, and SHALL cascade-delete the workflow's
  execution history rows.

### FR-07.9 — Immutability of run history

Once a `workflow_executions` row is written, its fields SHALL NOT be
mutated. If the underlying workflow is edited, past executions
continue to reference the workflow id but preserve their own
`definition_snapshot` (the definition as it was at run time).

### FR-07.10 — Permissioning

- `workflows:read` gates viewing workflows and their run history.
- `workflows:write` gates create/update/delete/duplicate.
- `workflows:execute` gates the **Run now** action.

## UI/UX requirements

- The Workflow Builder occupies the full main-content area with:
  - A left rail of node types (draggable "chips").
  - A center canvas backed by React Flow with zoom/pan and a
    minimap.
  - A right rail: node configuration panel when a node is
    selected, otherwise workflow-level info (name, description,
    schedule).
  - A top toolbar: **Run**, **Save**, **Duplicate**, workflow
    name, last-saved indicator, unsaved-changes indicator.
- Nodes are rendered with a neutral card style, a small icon per
  type, and a status dot on the top-right during and after run —
  see [`docs/screenshots/08-workflow-builder-dark.png`](../screenshots/08-workflow-builder-dark.png).
- Edges are drawn with a smooth Bezier curve; hovering an edge
  reveals a delete affordance.
- Attempted cycle: the offending edge briefly flashes red and a
  toast explains "This connection would create a loop."
- Attempted-oversized definition: save is blocked with a modal
  listing the failing invariant(s).
- Run panel opens as a bottom drawer showing per-node status and
  the final frame — see
  [`docs/screenshots/09-workflow-run-output.png`](../screenshots/09-workflow-run-output.png).
- Keyboard: **Delete** removes the selected node/edge; **Ctrl/Cmd+S**
  saves; **Ctrl/Cmd+Enter** runs.
- The workflows list is a table with columns: name, schedule
  status, last run status, last run time — see
  [`docs/screenshots/07-workflows.png`](../screenshots/07-workflows.png).

## Acceptance criteria

- [ ] Dragging a `source` node from the left rail onto the canvas
  creates a node with a unique id and opens the configuration
  panel.
- [ ] Connecting `source` → `filter` → `output` and clicking Save
  persists the workflow and returns 200.
- [ ] Attempting to save a workflow with a cycle returns 400 with a
  message containing "cycle."
- [ ] Attempting to save a workflow with 201 nodes returns 400.
- [ ] Clicking **Run now** on a valid workflow streams per-node
  results and ends with a final `workflow_executions` row whose
  `status` is `succeeded`.
- [ ] A source node returning 200K rows produces a downstream frame
  of exactly 100,000 rows, and the run is marked with a
  row-cap warning tag.
- [ ] A workflow whose overall wall-clock time exceeds 2 minutes
  ends with `status = timeout` and `error_code = timeout`.
- [ ] A user with `workflows:read` but not `workflows:execute`
  sees the **Run now** button disabled with a tooltip.
- [ ] Deleting a workflow deletes its execution history rows.
- [ ] Duplicating a workflow creates a new workflow with a distinct
  id, an appended " (copy)" name, and no schedule.

## Edge cases & error handling

- **Empty workflow**: Saving a workflow with zero nodes SHALL be
  allowed (draft state), but **Run now** SHALL be disabled.
- **Multiple sources**: A workflow may have multiple `source` nodes;
  they run in parallel.
- **Multiple outputs**: A workflow may have multiple `output` nodes;
  each produces a distinct artifact in the run result.
- **Dangling nodes**: Nodes with no downstream path to an `output`
  node SHALL be allowed (they run and their frames are visible in
  the run panel but not exported).
- **JSONata compile error**: A `filter`/`transform` node whose
  expression fails to compile SHALL fail the run with the classified
  error `invalid_config` and a JSONata-annotated message.
- **Empty frame**: A frame with zero rows SHALL flow through
  downstream nodes normally; join/aggregate on empty inputs
  produces empty outputs.
- **Type mismatches on join key**: When left and right keys have
  different types, the run SHALL fail with `invalid_config` and a
  message pointing at the offending columns.
- **Source connection deleted mid-edit**: The node keeps its
  `connectionId` value; the source panel shows an inline error and
  Run is blocked until the reference is fixed.
- **Concurrent edits**: Server SHALL use `updated_at` as an optimistic
  lock; a second save with a stale `updated_at` returns 409 and the
  UI offers a "reload workflow" affordance.
- **Aggregate on missing column**: The run SHALL fail with
  `invalid_config` naming the missing column.

## Non-functional requirements

- **Latency**: Small workflows (<10 nodes, <1000 rows total) SHALL
  complete in under 5 seconds at p95 excluding external source
  time.
- **Isolation**: A single failing workflow SHALL NOT prevent other
  workflows from running; the execution engine SHALL run workflows
  concurrently up to a configurable worker pool size.
- **Determinism**: Given identical inputs and identical
  definitions, node executors SHALL produce identical outputs
  (barring nondeterminism in the source itself, such as clock
  time).
- **Memory**: Executor SHALL stream row batches where possible and
  SHALL NOT hold the full frame in memory beyond the 100K-row cap.
- **Observability**: Every run SHALL emit Prometheus metrics
  (`workflow_run_started_total`, `workflow_run_completed_total{status}`,
  `workflow_run_duration_seconds` histogram, per-node duration).

## Market context & differentiation

| Product              | Visual DAG builder                       | Notes                                                                                       |
| -------------------- | ---------------------------------------- | ------------------------------------------------------------------------------------------- |
| **n8n**              | Node-based visual builder                | Great UX; very code-y with JS expressions; self-hosted OK; less structured DAG semantics.   |
| **Zapier**           | Linear step editor                       | Excellent onboarding; branching is limited; hosted-only.                                    |
| **Airbyte**          | Connector-centric UI                     | Focused on ELT source-destination pairs, not arbitrary transformations.                     |
| **Airflow**          | DAG defined in Python                    | Powerful but code-only; visual UI is read-only.                                             |
| **Prefect / Dagster**| Python-first with UI                     | Code-only authoring; visual view is for observability.                                      |
| **AWS Step Functions**| Amazon States Language JSON             | Cloud-native, AWS-only; no in-browser drag/drop authoring outside the AWS console.          |
| **Retool Workflows** | Visual builder                           | Ties data pipelines to Retool apps; less standalone.                                        |
| **Metabase Actions** | Not a workflow tool                      | Metabase has models but no visual DAG.                                                      |

Data Explorer's differentiators for the Workflow Builder are:

- **One tabular contract across every node.** Every executor is a
  `Frame` → `Frame` function; there's no per-source data-shape
  mismatch.
- **JSONata everywhere.** `filter` and `transform` share one
  expression language regardless of the source type — no per-source
  DSL to relearn.
- **Guardrails first.** Hard limits on nodes (200), edges (500),
  per-node rows (100K), per-node time (60s), and whole-run time
  (2 min) are enforced by the executor, not just documented.
- **Every intermediate frame is inspectable.** Users see what each
  node produced, not just the final answer.
- **In-repo cron scheduler.** No external orchestrator dependency;
  the same server that authored the workflow runs it.
- **Self-hosted from day one.** No SaaS-only tier.

## Future enhancements (out of scope)

- Node types for HTTP output (post the result to a webhook).
- Branching / conditional edges.
- Loop / iterator nodes.
- Sub-workflow references (call another workflow as a node).
- Real-time streaming sources.
- Multiple runs comparison / diff.
- Version control for workflows (git-style history with rollback).
- Per-node retry policy configuration.
- Local secret-only inputs (fill a workflow's parameter at run time
  without persisting).
- Parallelism ceiling per workflow.
- Frame-level dry-run without side effects on output targets.

## Cross-references

- [FR-03 Data Source Connection Management](./FR-03-connection-management.md)
- [FR-06 Ad-Hoc Data Exploration](./FR-06-ad-hoc-exploration.md)
- [FR-08 Workflow Scheduling & Automation](./FR-08-workflow-scheduling.md)
- [FR-09 Query Result Export & Sharing](./FR-09-query-result-export.md)
- [FR-10 Audit Logging & Compliance](./FR-10-audit-logging.md)
- [FR-11 Observability, Guardrails & Reliability](./FR-11-observability-and-guardrails.md)
