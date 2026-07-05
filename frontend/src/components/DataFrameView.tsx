import { useEffect, useMemo, useState } from "react";

import type { DataFrame } from "../api/types";
import { downloadFrame } from "../lib/exportFrame";
import { useSavedChartsStore, type ChartConfig } from "../state/savedChartsStore";
import { DataTable } from "./DataTable";
import { FrameChart } from "./charts/FrameChart";
import { chartableFields } from "./charts/chartUtils";
import { IconDownload } from "./icons";
import { Modal } from "./Modal";
import { Button, Field, Input, Select } from "./ui";

const TYPE_COLORS: Record<string, string> = {
  string: "var(--accent)",
  int64: "var(--success)",
  float64: "var(--success)",
  bool: "var(--warning)",
  time: "var(--danger)",
  json: "var(--text-tertiary)",
  null: "var(--text-tertiary)",
};
const INITIAL_RENDER_ROWS = 500;
const EXPORT_VISIBLE_ROW_CAP = 10_000;

function configStorageKey(frame: DataFrame) {
  return `de-chart-config:${frame.meta.sourceId ?? frame.meta.name ?? frame.meta.sourceType ?? "frame"}`;
}

function defaultChartConfig(frame: DataFrame): ChartConfig {
  const { numeric, categorical } = chartableFields(frame);
  return {
    title: frame.meta.name || frame.meta.sourceType || "Untitled chart",
    kind: numeric.length > 1 ? "line" : "bar",
    xKey: categorical[0] ?? frame.schema.fields[0]?.name ?? "",
    yKeys: numeric.slice(0, 2),
    viewMode: numeric.length > 0 ? "split" : "table",
  };
}

function loadStoredConfig(frame: DataFrame): ChartConfig {
  try {
    const raw = localStorage.getItem(configStorageKey(frame));
    return raw ? ({ ...defaultChartConfig(frame), ...(JSON.parse(raw) as Partial<ChartConfig>) }) : defaultChartConfig(frame);
  } catch {
    return defaultChartConfig(frame);
  }
}

export function DataFrameView({ frame, onAddFilterNode }: { frame: DataFrame; onAddFilterNode?: () => void }) {
  const columns = frame.schema.fields.map((f) => f.name);
  const saveChart = useSavedChartsStore((s) => s.saveChart);
  const [visibleRows, setVisibleRows] = useState(() => Math.min(frame.rows.length, INITIAL_RENDER_ROWS));
  const [exportWarningOpen, setExportWarningOpen] = useState(false);
  const [chartConfig, setChartConfig] = useState<ChartConfig>(() => loadStoredConfig(frame));
  const { numeric, categorical } = useMemo(() => chartableFields(frame), [frame]);
  const canChart = numeric.length > 0 && categorical.length > 0;

  useEffect(() => {
    const next = loadStoredConfig(frame);
    setChartConfig(next);
  }, [frame]);

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

  useEffect(() => {
    localStorage.setItem(configStorageKey(frame), JSON.stringify(chartConfig));
  }, [chartConfig, frame]);

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

  function patchChartConfig(patch: Partial<ChartConfig>) {
    setChartConfig((prev) => ({ ...prev, ...patch }));
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
        <div style={{ display: "flex", gap: 6, flexShrink: 0, flexWrap: "wrap" }}>
          <Button size="sm" variant={chartConfig.viewMode === "table" ? "primary" : "default"} onClick={() => patchChartConfig({ viewMode: "table" })}>
            Table
          </Button>
          <Button size="sm" variant={chartConfig.viewMode === "chart" ? "primary" : "default"} onClick={() => patchChartConfig({ viewMode: "chart" })} disabled={!canChart}>
            Chart
          </Button>
          <Button size="sm" variant={chartConfig.viewMode === "split" ? "primary" : "default"} onClick={() => patchChartConfig({ viewMode: "split" })} disabled={!canChart}>
            Both
          </Button>
          <Button size="sm" onClick={requestCSVExport}>
            <IconDownload width={12} height={12} /> CSV
          </Button>
          <Button size="sm" onClick={() => downloadFrame(frame, "json")}>
            <IconDownload width={12} height={12} /> JSON
          </Button>
          {canChart && (
            <Button size="sm" onClick={() => saveChart(frame.meta.name || frame.meta.sourceType || "Result", frame, chartConfig)}>
              Save chart
            </Button>
          )}
        </div>
      </div>

      {canChart && chartConfig.viewMode !== "table" && (
        <div className="chart-layout" style={{ marginBottom: chartConfig.viewMode === "split" ? 12 : 0 }}>
          <div className="chart-toolbar">
            <Field htmlFor="chart-title" label="Title" style={{ margin: 0 }}>
              <Input id="chart-title" value={chartConfig.title} onChange={(e) => patchChartConfig({ title: e.target.value })} />
            </Field>
            <Field htmlFor="chart-kind" label="Chart" style={{ margin: 0 }}>
              <Select id="chart-kind" value={chartConfig.kind} onChange={(e) => patchChartConfig({ kind: e.target.value as ChartConfig["kind"] })}>
                <option value="bar">Bar</option>
                <option value="line">Line</option>
                <option value="area">Area</option>
                <option value="pie">Pie</option>
              </Select>
            </Field>
            <Field htmlFor="chart-x" label="Dimension" style={{ margin: 0 }}>
              <Select id="chart-x" value={chartConfig.xKey} onChange={(e) => patchChartConfig({ xKey: e.target.value })}>
                {categorical.map((key) => (
                  <option key={key} value={key}>
                    {key}
                  </option>
                ))}
              </Select>
            </Field>
            <Field htmlFor="chart-y" label={chartConfig.kind === "pie" ? "Value" : "Metrics"} style={{ margin: 0 }}>
              <Select
                id="chart-y"
                multiple={chartConfig.kind !== "pie"}
                value={chartConfig.kind === "pie" ? chartConfig.yKeys.slice(0, 1) : chartConfig.yKeys}
                onChange={(e) => {
                  const values = Array.from(e.target.selectedOptions, (option) => option.value);
                  patchChartConfig({ yKeys: chartConfig.kind === "pie" ? values.slice(0, 1) : values.slice(0, 3) });
                }}
              >
                {numeric.map((key) => (
                  <option key={key} value={key}>
                    {key}
                  </option>
                ))}
              </Select>
            </Field>
          </div>
          <FrameChart frame={frame} config={chartConfig} />
        </div>
      )}

      {chartConfig.viewMode !== "chart" && <DataTable columns={columns} rows={displayedFrame.rows} />}
      {visibleRows < frame.rows.length && chartConfig.viewMode !== "chart" && (
        <div className="field-hint" role="status" style={{ marginTop: 6 }}>
          Rendering rows progressively: showing {visibleRows.toLocaleString()} of {frame.rows.length.toLocaleString()} rows.
        </div>
      )}

      <div className="toolbar" style={{ marginTop: 8, flexWrap: "wrap", color: "var(--text-secondary)", fontSize: 11.5 }}>
        <span>
          {frame.meta.rowCount} row{frame.meta.rowCount === 1 ? "" : "s"} &middot; {frame.meta.columnCount} col{frame.meta.columnCount === 1 ? "" : "s"} &middot; {frame.meta.durationMs}ms
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
              <Button variant="primary" onClick={() => {
                downloadFrame(frame, "csv", EXPORT_VISIBLE_ROW_CAP);
                setExportWarningOpen(false);
              }}>
                Export first 10K
              </Button>
            </>
          }
        >
          <p className="field-hint">
            This result is truncated or exceeds {EXPORT_VISIBLE_ROW_CAP.toLocaleString()} rows. Exporting first 10K keeps the download bounded; refine the query if you need a smaller, complete result.
          </p>
        </Modal>
      )}
    </div>
  );
}
