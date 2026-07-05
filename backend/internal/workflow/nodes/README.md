# internal/workflow/nodes

## What this package does

`internal/workflow/nodes` contains the **executor for each workflow node type**. Each executor implements a small interface: given input `*dataframe.Frame`(s) and the node's config, return an output `*dataframe.Frame`. The workflow engine calls them in topological order.

## Node executors

### source (`source.go`)

- Reads `config.ConnectionID` and `config.QuerySpec` from the node.
- Calls `connections.Service.Query(ctx, connectionID, spec)`.
- Returns the resulting `*dataframe.Frame` — metadata includes `sourceType`, `sourceID`, and lineage.
- **This is the only node type that touches an external system.**

### filter (`filter.go`)

- Evaluates a JSONata boolean expression against each row.
- Rows where the expression evaluates to `false` are dropped.
- Expression is compiled once and reused across all rows.
- Input and output schemas are identical.

### transform (`transform.go`)

- Applies a JSONata expression to each row to reshape it into a new object.
- Output schema is inferred from the transformed rows.
- Can add, remove, rename, or compute new fields.

### join (`join.go`)

- Thin adapter over `dataframe.Join(left, right, key, kind)`.
- Receives two input frames (disambiguated by `targetHandle: "left" | "right"`).
- Supports inner and left joins.
- Automatic column-collision prefixing (e.g., `left_id`, `right_id`) when both frames have a column of the same name.

### aggregate (`aggregate.go`)

- Thin adapter over `frame.GroupBy(cols…).Agg(…)`.
- Config: group-by column list + aggregation specs (`{column, function}` pairs where function is `sum`/`avg`/`count`/`min`/`max`).
- The actual algorithm lives in `pkg/dataframe`, not here.

### output (`output.go`)

- Terminal node; simply passes through its input frame.
- The engine uses the output node's frame as the execution result.
- Validates that a DAG has exactly one output node.

### streaming (`streaming.go`)

- Utility helpers for streaming partial results to clients (used by newer execution paths).
- Not a node type itself.

### types (`types.go`)

- Shared type definitions: `NodeConfig`, `NodeResult`, `ExecutionInput`.
- Defines the executor interface:

```go
type Executor interface {
    Execute(ctx context.Context, inputs map[string]*dataframe.Frame, config NodeConfig) (*dataframe.Frame, error)
}
```

## Architecture decisions (ADRs)

| Decision | Rationale |
|---|---|
| `join` and `aggregate` are thin wrappers | The algorithm lives in `pkg/dataframe` — a standalone, testable package — not duplicated here |
| `source` is the only node with external I/O | Every other node is a pure in-memory transformation; this makes the engine testable without mocking a connector |
| JSONata for filter/transform | Expressive, standard expression language; no need to invent a DSL |
| Single output node validation | Ambiguous output (which frame is the result?) is a definition error, not a runtime choice |

## Scope and responsibilities

- Implement the execution logic for each node type.
- Delegate all data algorithms to `pkg/dataframe`.
- Delegate all external I/O to `connections.Service`.
- Return a `*dataframe.Frame` for every successful execution.

## Limitations and todos

- [ ] JSONata expressions are not sandboxed — a complex or recursive expression can be CPU-intensive.
- [ ] No timeout per-node (only a total execution timeout on the full DAG).
- [ ] `source` node does not support streaming results from the connector.
- [ ] Only one `output` node is supported; fan-out to multiple sinks is not possible today.
- [ ] No node-level retry configuration.
