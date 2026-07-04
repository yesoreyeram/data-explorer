import {
  Area,
  AreaChart,
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  Legend,
  Line,
  LineChart,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

import type { DataFrame } from "../../api/types";
import type { ChartConfig } from "../../state/savedChartsStore";

const COLORS = ["#111827", "#374151", "#6b7280", "#9ca3af", "#d1d5db"];

function numericValue(value: unknown): number | null {
  if (typeof value === "number") return Number.isFinite(value) ? value : null;
  if (typeof value === "string" && value.trim() !== "") {
    const parsed = Number(value);
    return Number.isFinite(parsed) ? parsed : null;
  }
  return null;
}

export function FrameChart({ frame, config, height = 280 }: { frame: DataFrame; config: ChartConfig; height?: number }) {
  const rows = frame.rows.slice(0, 50).map((row) => {
    const next: Record<string, unknown> = { ...row };
    config.yKeys.forEach((key) => {
      next[key] = numericValue(row[key]);
    });
    return next;
  });

  if (!config.xKey || config.yKeys.length === 0) {
    return <div className="empty-state">Choose chart axes to preview data.</div>;
  }

  return (
    <div className="chart-surface" style={{ height }}>
      <ResponsiveContainer width="100%" height="100%">
        {config.kind === "bar" ? (
          <BarChart data={rows}>
            <CartesianGrid strokeDasharray="3 3" />
            <XAxis dataKey={config.xKey} />
            <YAxis />
            <Tooltip />
            <Legend />
            {config.yKeys.map((key, index) => (
              <Bar key={key} dataKey={key} fill={COLORS[index % COLORS.length]} />
            ))}
          </BarChart>
        ) : config.kind === "line" ? (
          <LineChart data={rows}>
            <CartesianGrid strokeDasharray="3 3" />
            <XAxis dataKey={config.xKey} />
            <YAxis />
            <Tooltip />
            <Legend />
            {config.yKeys.map((key, index) => (
              <Line key={key} type="monotone" dataKey={key} stroke={COLORS[index % COLORS.length]} strokeWidth={2} dot={false} />
            ))}
          </LineChart>
        ) : config.kind === "area" ? (
          <AreaChart data={rows}>
            <CartesianGrid strokeDasharray="3 3" />
            <XAxis dataKey={config.xKey} />
            <YAxis />
            <Tooltip />
            <Legend />
            {config.yKeys.map((key, index) => (
              <Area key={key} type="monotone" dataKey={key} stroke={COLORS[index % COLORS.length]} fill={COLORS[index % COLORS.length]} fillOpacity={0.18} />
            ))}
          </AreaChart>
        ) : (
          <PieChart>
            <Tooltip />
            <Legend />
            <Pie data={rows} nameKey={config.xKey} dataKey={config.yKeys[0]} outerRadius={90}>
              {rows.map((row, index) => (
                <Cell key={`${String(row[config.xKey])}-${index}`} fill={COLORS[index % COLORS.length]} />
              ))}
            </Pie>
          </PieChart>
        )}
      </ResponsiveContainer>
    </div>
  );
}
