# internal/workflow

## What this package does

`internal/workflow` implements the **visual pipeline builder's backend**: DAG definition, validation, topological execution, schedule management, and execution history persistence. Every saved pipeline in the UI is a `Workflow` with a `Definition` (a JSON DAG) stored as JSONB. Running a workflow means executing that DAG through the `Engine`.

## DAG definition

```
{ nodes: [...], edges: [...] }
```

Authored on the React Flow canvas and stored as-is (JSONB) — the server round-trips node positions without transformation. The frontend and backend share the same JSON shape, defined in `definition.go`.

### Node types

| Type | Description |
|---|---|
| `source` | Queries a `Connection` via `connections.Service.Query`; produces a `*dataframe.Frame` |
| `filter` | Evaluates a JSONata boolean expression per row; drops non-matching rows |
| `transform` | Applies a JSONata reshape expression; restructures columns/values |
| `join` | Inner or left join between two upstream frames on a shared key column |
| `aggregate` | Group-by one or more columns + one or more aggregation functions (sum/avg/count/min/max) |
| `output` | Terminal node; materializes the final frame as the execution result |

### Size guardrails

| Guardrail | Value | Description |
|---|---|---|
| `MaxNodes` | 200 | Rejected at validation time |
| `MaxEdges` | 500 | Rejected at validation time |
| `MaxRowsPerNode` | 100,000 | Applied after every node's execution |
| `MaxExecutionDuration` | 2 minutes | Context timeout on the full `Engine.Run` call |

## Engine (`engine.go`)

`Engine.Run` executes a validated DAG:

1. **Topological sort** (Kahn's algorithm) — a cycle is a validation error rejected before execution starts.
2. **Execute each node in order** — gather declared inputs from upstream outputs; most nodes have one input; `join` has two (disambiguated by `targetHandle: "left" | "right"`).
3. **Row-cap after each node** — `dataframe.LimitRows(MaxRowsPerNode)` is applied after every node; defense in depth for `join`, whose output is not bounded by any connector's row limit.
4. **Stop at first failing node** — everything executed up to that point is still reported (timings, row counts) so a partially-broken pipeline is debuggable.

Every execution is persisted as a `workflow_executions` row regardless of success or failure, with per-node `timings`, `rowCounts`, and `errors`.

## Schedule management

`Service.SetSchedule(ctx, workflowID, cronExpr, enabled)`:
- Validates the cron expression via `robfig/cron/v3`.
- Computes `schedule_next_run` up front.
- Stores `schedule_cron`, `schedule_enabled`, and `schedule_next_run` on the `workflows` row.

The scheduler's due-check is a cheap `WHERE schedule_enabled AND schedule_next_run <= now()` against a partial index — no live cron evaluation on every tick.

`TriggeredBy` is a plain `string` column (not a FK to `users`) because scheduled runs have no acting user; the sentinel value `"scheduler"` is used instead.

## Repository

All SQL access for `workflows`, `workflow_executions`, and schedule fields. Never called directly by handlers — only by `Service`.

## Architecture decisions (ADRs)

| Decision | Rationale |
|---|---|
| DAG stored as JSONB | React Flow canvas data round-trips without transformation; frontend is the authoritative shape |
| Topological sort at execution time, not at save time | The definition can be saved in an incomplete/invalid state (in-progress canvas); validation only happens on "Run" |
| Row cap after every node | `join` output is unbounded relative to input sizes; defense in depth prevents OOM on low-selectivity joins |
| Execution record for failed runs | Partial execution results are essential for debugging; "it failed" with no context is not useful |
| `TriggeredBy` as plain string | No synthetic system user; scheduled runs are a legitimate non-human trigger |

## Scope and responsibilities

- CRUD for `Workflow` definitions.
- Validate DAG structure (cycle detection, size limits).
- Execute DAGs with topological ordering and guardrails.
- Manage cron schedules and next-run timestamps.
- Persist and query execution history.

## Limitations and todos

- [ ] Execution is synchronous within the HTTP request; a 2-minute timeout is the only bound.
- [ ] No parallel node execution; a wide fan-in DAG executes serially.
- [ ] No retry logic for individual failing nodes.
- [ ] No partial result streaming; the client waits for the entire run.
- [ ] `join` guardrail applies after the full join is computed; a very large join will consume memory before the cap is applied.
- [ ] JSONata is the only transform/filter expression language; no SQL-over-dataframe option.
