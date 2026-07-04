import type { DataFrame } from "../api/types";
import { DataTable } from "./DataTable";
import { downloadFrame } from "../lib/exportFrame";
import { IconDownload } from "./icons";
import { Button } from "./ui";

const TYPE_COLORS: Record<string, string> = {
  string: "var(--accent)",
  int64: "var(--success)",
  float64: "var(--success)",
  bool: "var(--warning)",
  time: "var(--danger)",
  json: "var(--text-tertiary)",
  null: "var(--text-tertiary)",
};

/** Renders a dataframe's rows as a table plus its rich metadata (schema
 * types, row/column counts, timing, lineage, truncation, warnings) - the
 * same provenance every node in the pipeline attaches to its output. */
export function DataFrameView({ frame }: { frame: DataFrame }) {
  const columns = frame.schema.fields.map((f) => f.name);

  return (
    <div>
      <div style={{ display: "flex", flexWrap: "wrap", justifyContent: "space-between", gap: 6, marginBottom: 8 }}>
        <div style={{ display: "flex", flexWrap: "wrap", gap: 6 }}>
          {frame.schema.fields.map((f) => (
            <span key={f.name} className="badge" title={f.nullable ? "nullable" : "not null"}>
              {f.name} <span style={{ color: TYPE_COLORS[f.type] ?? "inherit" }}>{f.type}</span>
            </span>
          ))}
        </div>
        <div style={{ display: "flex", gap: 6, flexShrink: 0 }}>
          <Button size="sm" onClick={() => downloadFrame(frame, "csv")}>
            <IconDownload width={12} height={12} /> CSV
          </Button>
          <Button size="sm" onClick={() => downloadFrame(frame, "json")}>
            <IconDownload width={12} height={12} /> JSON
          </Button>
        </div>
      </div>

      <DataTable columns={columns} rows={frame.rows} />

      <div className="toolbar" style={{ marginTop: 8, flexWrap: "wrap", color: "var(--text-secondary)", fontSize: 11.5 }}>
        <span>
          {frame.meta.rowCount} row{frame.meta.rowCount === 1 ? "" : "s"} &middot; {frame.meta.columnCount} col
          {frame.meta.columnCount === 1 ? "" : "s"} &middot; {frame.meta.durationMs}ms
        </span>
        {frame.meta.sourceType && <span className="badge badge-neutral">{frame.meta.sourceType}</span>}
        {frame.meta.truncated && <span className="badge badge-warning">truncated</span>}
        {frame.meta.lineage && frame.meta.lineage.length > 0 && <span>lineage: {frame.meta.lineage.join(" → ")}</span>}
      </div>

      {frame.meta.warnings && frame.meta.warnings.length > 0 && (
        <div className="error-banner" style={{ marginTop: 8, background: "var(--warning-soft)", color: "var(--warning)", borderColor: "var(--warning)" }}>
          {frame.meta.warnings.join(" · ")}
        </div>
      )}
    </div>
  );
}
