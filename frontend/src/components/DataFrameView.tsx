import { useEffect, useMemo, useState } from "react";
import type { DataFrame } from "../api/types";
import { DataTable } from "./DataTable";
import { downloadFrame } from "../lib/exportFrame";
import { IconDownload } from "./icons";
import { Button } from "./ui";
import { Modal } from "./Modal";

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
const INITIAL_RENDER_ROWS = 500;
const EXPORT_VISIBLE_ROW_CAP = 10_000;

export function DataFrameView({ frame, onAddFilterNode }: { frame: DataFrame; onAddFilterNode?: () => void }) {
  const columns = frame.schema.fields.map((f) => f.name);
  const [visibleRows, setVisibleRows] = useState(() => Math.min(frame.rows.length, INITIAL_RENDER_ROWS));
  const [exportWarningOpen, setExportWarningOpen] = useState(false);

  useEffect(() => {
    setVisibleRows(Math.min(frame.rows.length, INITIAL_RENDER_ROWS));
    if (frame.rows.length <= INITIAL_RENDER_ROWS) return;
    let cancelled = false;
    let next = INITIAL_RENDER_ROWS;
    const tick = () => {
      if (cancelled) return;
      next = Math.min(frame.rows.length, next + 1000);
      setVisibleRows(next);
      if (next < frame.rows.length) window.setTimeout(tick, 16);
    };
    window.setTimeout(tick, 16);
    return () => {
      cancelled = true;
    };
  }, [frame]);

  const displayedFrame = useMemo(() => ({ ...frame, rows: frame.rows.slice(0, visibleRows) }), [frame, visibleRows]);
  const hasRowCapWarning = frame.meta.truncated || (frame.meta.warnings ?? []).some((w) => /row cap|100000|100,000/i.test(w));
  const shouldWarnCSV = frame.meta.truncated || frame.meta.rowCount > EXPORT_VISIBLE_ROW_CAP || frame.rows.length > EXPORT_VISIBLE_ROW_CAP;

  function requestCSVExport() {
    if (shouldWarnCSV) {
      setExportWarningOpen(true);
      return;
    }
    downloadFrame(frame, "csv");
  }

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
          <Button size="sm" onClick={requestCSVExport}>
            <IconDownload width={12} height={12} /> CSV
          </Button>
          <Button size="sm" onClick={() => downloadFrame(frame, "json")}>
            <IconDownload width={12} height={12} /> JSON
          </Button>
        </div>
      </div>

      <DataTable columns={columns} rows={displayedFrame.rows} />
      {visibleRows < frame.rows.length && (
        <div className="field-hint" role="status" style={{ marginTop: 6 }}>
          Rendering rows progressively: showing {visibleRows.toLocaleString()} of {frame.rows.length.toLocaleString()} rows.
        </div>
      )}

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
          <div>{frame.meta.warnings.join(" · ")}</div>
          {hasRowCapWarning && onAddFilterNode && (
            <Button size="sm" onClick={onAddFilterNode} style={{ marginTop: 8 }}>
              Add filter node
            </Button>
          )}
        </div>
      )}

      {exportWarningOpen && (
        <Modal
          title="Export visible rows?"
          onClose={() => setExportWarningOpen(false)}
          footer={
            <>
              <Button onClick={() => setExportWarningOpen(false)}>Refine query</Button>
              <Button
                variant="primary"
                onClick={() => {
                  downloadFrame(frame, "csv", EXPORT_VISIBLE_ROW_CAP);
                  setExportWarningOpen(false);
                }}
              >
                Export first 10K
              </Button>
            </>
          }
        >
          <p className="field-hint">
            This result is truncated or exceeds {EXPORT_VISIBLE_ROW_CAP.toLocaleString()} rows. Exporting first 10K keeps the
            download bounded; refine the query if you need a smaller, complete result.
          </p>
        </Modal>
      )}
    </div>
  );
}
