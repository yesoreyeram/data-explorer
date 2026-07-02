import { memo } from "react";
import { Handle, Position, type NodeProps } from "reactflow";

import type { NodeType } from "../../api/types";
import { IconDatabase, IconFilter, IconGitMerge, IconLayers, IconOutput, IconWand } from "./workflowIcons";

export interface FlowNodeData {
  label: string;
  nodeType: NodeType;
  summary?: string;
  error?: string;
  rowsOut?: number;
}

const META: Record<NodeType, { icon: typeof IconDatabase; color: string; hasInput: boolean; hasOutput: boolean }> = {
  source: { icon: IconDatabase, color: "hsl(217 91% 60%)", hasInput: false, hasOutput: true },
  transform: { icon: IconWand, color: "hsl(268 80% 62%)", hasInput: true, hasOutput: true },
  filter: { icon: IconFilter, color: "hsl(38 92% 50%)", hasInput: true, hasOutput: true },
  join: { icon: IconGitMerge, color: "hsl(2 74% 58%)", hasInput: true, hasOutput: true },
  aggregate: { icon: IconLayers, color: "hsl(152 60% 40%)", hasInput: true, hasOutput: true },
  output: { icon: IconOutput, color: "hsl(222 15% 45%)", hasInput: true, hasOutput: false },
};

export const FlowNode = memo(function FlowNode({ data, selected }: NodeProps<FlowNodeData>) {
  const meta = META[data.nodeType];
  const Icon = meta.icon;

  return (
    <div
      className="flow-node"
      style={{ borderColor: selected ? meta.color : undefined, boxShadow: data.error ? "0 0 0 2px var(--danger)" : undefined }}
    >
      {data.nodeType === "join" ? (
        <>
          <Handle type="target" position={Position.Left} id="left" style={{ top: "35%", background: meta.color }} />
          <Handle type="target" position={Position.Left} id="right" style={{ top: "65%", background: meta.color }} />
        </>
      ) : (
        meta.hasInput && <Handle type="target" position={Position.Left} style={{ background: meta.color }} />
      )}

      <div className="flow-node-icon" style={{ background: meta.color }}>
        <Icon width={13} height={13} />
      </div>
      <div className="flow-node-body">
        <div className="flow-node-type">{data.nodeType}</div>
        <div className="flow-node-label">{data.label}</div>
        {data.summary && <div className="flow-node-summary">{data.summary}</div>}
        {typeof data.rowsOut === "number" && !data.error && <div className="flow-node-rows">{data.rowsOut} rows</div>}
        {data.error && <div className="flow-node-error">{data.error}</div>}
      </div>

      {meta.hasOutput && <Handle type="source" position={Position.Right} style={{ background: meta.color }} />}
    </div>
  );
});
