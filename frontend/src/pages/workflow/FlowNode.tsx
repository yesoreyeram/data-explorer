import { memo, useEffect, useMemo, useState } from "react";
import { Handle, Position, type NodeProps } from "reactflow";

import type { NodeType } from "../../api/types";
import { IconDatabase, IconFilter, IconGitMerge, IconLayers, IconOutput, IconWand } from "./workflowIcons";

export interface FlowNodeData {
  label: string;
  nodeType: NodeType;
  summary?: string;
  error?: string;
  rowsOut?: number;
  rowCap?: number;
  truncated?: boolean;
  warnings?: string[];
  deadlineAt?: string;
  runActive?: boolean;
}

const META: Record<NodeType, { icon: typeof IconDatabase; hasInput: boolean; hasOutput: boolean }> = {
  source: { icon: IconDatabase, hasInput: false, hasOutput: true },
  transform: { icon: IconWand, hasInput: true, hasOutput: true },
  filter: { icon: IconFilter, hasInput: true, hasOutput: true },
  join: { icon: IconGitMerge, hasInput: true, hasOutput: true },
  aggregate: { icon: IconLayers, hasInput: true, hasOutput: true },
  output: { icon: IconOutput, hasInput: true, hasOutput: false },
};

// Node types are told apart by icon + label, not color - see the palette's
// minimal-color design in styles/app.css. Handles/borders use the theme's
// monochrome accent; only the error state keeps a (semantic) red.
export const FlowNode = memo(function FlowNode({ data, selected }: NodeProps<FlowNodeData>) {
  const meta = META[data.nodeType];
  const Icon = meta.icon;
  const [now, setNow] = useState(Date.now());
  const remainingMs = useMemo(() => (data.deadlineAt ? Math.max(0, new Date(data.deadlineAt).getTime() - now) : 0), [data.deadlineAt, now]);

  useEffect(() => {
    if (!data.runActive || !data.deadlineAt) return;
    const id = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(id);
  }, [data.deadlineAt, data.runActive]);

  const rowCap = data.rowCap ?? 100_000;
  const rowTone =
    data.truncated || (typeof data.rowsOut === "number" && data.rowsOut >= rowCap)
      ? "danger"
      : typeof data.rowsOut === "number" && data.rowsOut >= rowCap * 0.8
        ? "warning"
        : "neutral";

  return (
    <div
      className="flow-node"
      style={{
        borderColor: selected ? "var(--accent)" : undefined,
        boxShadow: data.error ? "0 0 0 2px var(--danger)" : undefined,
      }}
    >
      {data.nodeType === "join" ? (
        <>
          <Handle type="target" position={Position.Left} id="left" style={{ top: "35%", background: "var(--border-strong)" }} />
          <Handle type="target" position={Position.Left} id="right" style={{ top: "65%", background: "var(--border-strong)" }} />
        </>
      ) : (
        meta.hasInput && <Handle type="target" position={Position.Left} style={{ background: "var(--border-strong)" }} />
      )}

      <div className="flow-node-icon">
        <Icon width={13} height={13} />
      </div>
      <div className="flow-node-body">
        <div className="flow-node-type">{data.nodeType}</div>
        <div className="flow-node-label">{data.label}</div>
        {data.summary && <div className="flow-node-summary">{data.summary}</div>}
        {data.runActive && data.deadlineAt && !data.error && (
          <div className="flow-node-rows" aria-label={`Node timeout countdown ${Math.ceil(remainingMs / 1000)} seconds remaining`}>
            timeout {Math.ceil(remainingMs / 1000)}s
          </div>
        )}
        {typeof data.rowsOut === "number" && !data.error && (
          <div className={`flow-node-rows flow-node-rows-${rowTone}`}>
            {data.rowsOut.toLocaleString()} rows{rowTone === "warning" ? " · near cap" : rowTone === "danger" ? " · at cap" : ""}
          </div>
        )}
        {data.error && <div className="flow-node-error">{data.error}</div>}
      </div>

      {meta.hasOutput && <Handle type="source" position={Position.Right} style={{ background: "var(--border-strong)" }} />}
    </div>
  );
});
