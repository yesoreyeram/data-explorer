import type { ReactNode } from "react";

export type StatTileDeltaDirection = "up" | "down" | "flat";

export interface StatTileDelta {
  /** Human-readable delta value shown before the description, e.g. "+12%". */
  value: string;
  /** Direction drives the arrow glyph and semantic color (up = success,
   * down = danger, flat = tertiary text). Callers own the polarity, since
   * "down is good" for e.g. latency but bad for revenue. */
  direction: StatTileDeltaDirection;
  /** Free-form context shown after the value, e.g. "from last week". */
  description?: ReactNode;
}

export interface StatTileProps {
  label: string;
  value: string | number;
  /** Optional 14–16px glyph rendered inside a monochrome square badge next
   * to the label (see the FlowAI dashboard reference). */
  icon?: ReactNode;
  /** Optional trend footer. Omit to render the tile without a divider or
   * footer row (backwards-compatible minimal variant). */
  delta?: StatTileDelta;
}

const ARROW: Record<StatTileDeltaDirection, string> = {
  up: "↑",
  down: "↓",
  flat: "→",
};

/** Compact KPI tile. Backward compatible: without `icon` or `delta` it
 * renders the same minimal shape it always did. With them, it matches the
 * dashboard-tile pattern from the design reference. */
export function StatTile({ label, value, icon, delta }: StatTileProps) {
  return (
    <div className="stat-tile">
      <div className="stat-tile-head">
        {icon && (
          <span className="stat-tile-icon" aria-hidden="true">
            {icon}
          </span>
        )}
        <div className="stat-label">{label}</div>
      </div>
      <div className="stat-value">{value}</div>
      {delta && (
        <div className="stat-tile-foot">
          <span className={`stat-tile-delta stat-tile-delta-${delta.direction}`}>
            <span aria-hidden="true">{ARROW[delta.direction]}</span>
            <span>{delta.value}</span>
            <span className="sr-only">
              {delta.direction === "up" ? "trending up" : delta.direction === "down" ? "trending down" : "no change"}
            </span>
          </span>
          {delta.description && <span>{delta.description}</span>}
        </div>
      )}
    </div>
  );
}
