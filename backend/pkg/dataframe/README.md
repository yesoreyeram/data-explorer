# pkg/dataframe

## What this package does

`pkg/dataframe` is a **standalone, pandas-style tabular data library**. It is the single contract every connector and every workflow node speaks: a `*dataframe.Frame` in, a `*dataframe.Frame` out. This uniformity is what lets filter, transform, join, and aggregate nodes compose freely regardless of whether the data came from a Postgres table, a paginated REST endpoint, or another node's output.

**Zero imports from this module's `internal/*` packages.** It does not know HTTP, SQL, auth, or workflows exist. It can be imported and used in any Go project.

## Core types

| Type | Description |
|---|---|
| `Frame` | Columnar storage (`map[string]any` per column, index-aligned) + schema + metadata |
| `Schema` | Ordered slice of `Field{Name, Type, Nullable}` |
| `FieldType` | Enum: `string`, `int64`, `float64`, `bool`, `time`, `json` |
| `Metadata` | Provenance: `SourceType`, `SourceID`, `Lineage`, `GeneratedAt`, `DurationMs`, `RowCount`, `ColumnCount`, `Truncated`, `Warnings` |

## Built-in operations

| Operation | Description |
|---|---|
| `New(schema)` | Create an empty frame from a schema |
| `Append(row)` | Add a row; infers/widens column types automatically |
| `Select(cols…)` | Project a subset of columns |
| `Rename(old, new)` | Rename a column |
| `Filter(predicate)` | Row-level predicate filter |
| `Concat(frames…)` | Union-compatible schema concat (widens types as needed) |
| `Join(left, right, key, kind)` | Inner or left join; auto-prefixes collision columns |
| `GroupBy(cols…).Agg(…)` | Group-by with sum/avg/count/min/max aggregators |
| `Describe()` | Per-column statistics: count, null-count, min, max, mean / string-length bounds |
| `InferType(value)` | Infer `FieldType` from a Go value |
| `unifyType(a, b)` | Widen two types: `int64`+`float64`→`float64`; incompatible→`json` |

## Guardrails (pkg/dataframe/guardrails.go)

| Guardrail | Default | Description |
|---|---|---|
| `LimitRows(n)` | Connector-configured | Caps row count; sets `Meta.Truncated = true` |
| `LimitColumns(n)` | Connector-configured | Bounds schema width |
| `TruncateCellsByType(limits)` | Per-type map | Clips oversized `string`/`bytes` cells without dropping the row |

These live in the standalone package (not in callers) because they are about data *integrity*, not any one caller's business policy.

## JSON wire format

```json
{
  "schema": [{"name": "id", "type": "int64", "nullable": false}, …],
  "rows":   [[1, "alice", …], …],
  "meta":   {"sourceType": "postgres", "sourceId": "conn-123", "rowCount": 42, "truncated": false, …}
}
```

This is the format returned by `POST /api/v1/explore/query` and every workflow execution node output.

## Additional features

- **Dictionary metadata** — lightweight column-level dictionary for high-cardinality string reuse.
- **Columnar snapshot format** (`columnar.go`) — additive representation for tighter memory layouts without changing the JSON wire contract; used by newer execution paths.

## Architecture decisions (ADRs)

| Decision | Rationale |
|---|---|
| Standalone package, no `internal/*` imports | Keeps the tabular data contract portable and testable in isolation; avoids circular imports |
| Type widening instead of error on mismatch | Real-world data sources produce heterogeneous types; silently widening `int64`+`float64` is correct; forcing everything to `json` only for incompatibles is honest |
| Guardrails live here, not in callers | Every execution path gets them for free; a new connector cannot accidentally bypass them |
| Columnar storage (`map[string]any`) | O(1) column access for filter/project/join without row-major copies |

## Limitations and todos

- [ ] No streaming / iterator interface; the entire result set is in memory before the caller sees it. Large queries rely on the row/size guardrails to keep this safe.
- [ ] `Join` is an in-memory nested-loop or hash join; there is no sort-merge path for very large inputs.
- [ ] `GroupBy` allocates intermediate maps; very high cardinality groups may be memory-intensive.
- [ ] No native Parquet or Arrow serialization; CSV/JSON are the only export formats today.
- [ ] The columnar snapshot format is not yet used by all connectors.
