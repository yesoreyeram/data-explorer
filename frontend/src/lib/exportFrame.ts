import type { DataFrame } from "../api/types";

function csvCell(value: unknown): string {
  if (value === null || value === undefined) return "";
  const s = typeof value === "object" ? JSON.stringify(value) : String(value);
  // Quote whenever the value could otherwise be misread: a comma, a quote,
  // or a newline embedded in the cell (common in free-text/JSON columns).
  if (/[",\n\r]/.test(s)) {
    return `"${s.replace(/"/g, '""')}"`;
  }
  return s;
}

export function frameToCSV(frame: DataFrame): string {
  const columns = frame.schema.fields.map((f) => f.name);
  const lines = [columns.map(csvCell).join(",")];
  for (const row of frame.rows) {
    lines.push(columns.map((col) => csvCell(row[col])).join(","));
  }
  // CRLF is the CSV RFC's line ending and what spreadsheet apps expect.
  return lines.join("\r\n");
}

export function frameToJSON(frame: DataFrame): string {
  return JSON.stringify(frame.rows, null, 2);
}

function download(content: string, mimeType: string, filename: string) {
  const blob = new Blob([content], { type: mimeType });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}

export function downloadFrame(frame: DataFrame, format: "csv" | "json") {
  const stamp = new Date().toISOString().replace(/[:.]/g, "-");
  const base = frame.meta.name ? frame.meta.name.replace(/[^a-z0-9_-]+/gi, "_") : "query-result";
  if (format === "csv") {
    download(frameToCSV(frame), "text/csv;charset=utf-8", `${base}-${stamp}.csv`);
  } else {
    download(frameToJSON(frame), "application/json;charset=utf-8", `${base}-${stamp}.json`);
  }
}
