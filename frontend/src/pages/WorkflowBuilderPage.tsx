import { useCallback, useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router-dom";
import ReactFlow, {
  addEdge,
  Background,
  Controls,
  MiniMap,
  ReactFlowProvider,
  useEdgesState,
  useNodesState,
  type Connection as FlowConnection,
  type Edge,
  type Node,
  type NodeTypes,
} from "reactflow";
import "reactflow/dist/style.css";

import { extractErrorMessage } from "../api/client";
import { executeWorkflow, getWorkflow, listWorkflowExecutions, updateWorkflow } from "../api/workflows";
import type { NodeType, DataFrame, WorkflowDefinition, WorkflowEdge, WorkflowNode, WorkflowStatus } from "../api/types";
import { DataFrameView } from "../components/DataFrameView";
import { StatusBadge } from "../components/StatusBadge";
import { PermissionGate } from "../components/PermissionGate";
import { PERMISSIONS } from "../lib/permissions";
import { IconPlay } from "../components/icons";
import { FlowNode, type FlowNodeData } from "./workflow/FlowNode";
import { NodeConfigPanel } from "./workflow/NodeConfigPanel";

const nodeTypes: NodeTypes = { flowNode: FlowNode };

const PALETTE: { type: NodeType; label: string }[] = [
  { type: "source", label: "Source" },
  { type: "transform", label: "Transform (JSONata)" },
  { type: "filter", label: "Filter (JSONata)" },
  { type: "join", label: "Join" },
  { type: "aggregate", label: "Aggregate" },
  { type: "output", label: "Output" },
];

const DEFAULT_CONFIG: Record<NodeType, Record<string, unknown>> = {
  source: { connectionId: "", query: { sql: "SELECT 1", rowLimit: 1000 } },
  transform: { expression: "$" },
  filter: { expression: "true" },
  join: { leftKey: "", rightKey: "", type: "inner", rightPrefix: "right_" },
  aggregate: { groupBy: [], aggregations: [] },
  output: {},
};

function definitionToFlow(def: WorkflowDefinition): { nodes: Node<FlowNodeData>[]; edges: Edge[] } {
  const nodes: Node<FlowNodeData>[] = (def.nodes ?? []).map((n, i) => ({
    id: n.id,
    type: "flowNode",
    position: n.position ?? { x: 80 + (i % 4) * 220, y: 80 + Math.floor(i / 4) * 130 },
    data: { label: n.name, nodeType: n.type, config: n.config } as unknown as FlowNodeData,
  }));
  const edges: Edge[] = (def.edges ?? []).map((e) => ({
    id: e.id,
    source: e.source,
    target: e.target,
    targetHandle: e.targetHandle || undefined,
  }));
  return { nodes, edges };
}

function flowToDefinition(nodes: Node<FlowNodeData>[], edges: Edge[]): WorkflowDefinition {
  const wfNodes: WorkflowNode[] = nodes.map((n) => ({
    id: n.id,
    type: n.data.nodeType,
    name: n.data.label,
    config: (n.data as unknown as { config: Record<string, unknown> }).config ?? {},
    position: { x: n.position.x, y: n.position.y },
  }));
  const wfEdges: WorkflowEdge[] = edges.map((e) => ({
    id: e.id,
    source: e.source,
    target: e.target,
    targetHandle: e.targetHandle ?? undefined,
  }));
  return { nodes: wfNodes, edges: wfEdges };
}

let nodeCounter = 0;

export function WorkflowBuilderPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const { data: workflow, isLoading } = useQuery({
    queryKey: ["workflow", id],
    queryFn: () => getWorkflow(id!),
    enabled: Boolean(id),
  });
  const { data: executions = [] } = useQuery({
    queryKey: ["workflow-executions", id],
    queryFn: () => listWorkflowExecutions(id!, 20),
    enabled: Boolean(id),
  });

  const [nodes, setNodes, onNodesChange] = useNodesState<FlowNodeData>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState([]);
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [status, setStatus] = useState<WorkflowStatus>("draft");
  const [runResult, setRunResult] = useState<DataFrame | null>(null);
  const [runError, setRunError] = useState<string | null>(null);
  const [running, setRunning] = useState(false);
  const [dirty, setDirty] = useState(false);

  useEffect(() => {
    if (!workflow) return;
    const flow = definitionToFlow(workflow.definition);
    setNodes(flow.nodes);
    setEdges(flow.edges);
    setName(workflow.name);
    setDescription(workflow.description);
    setStatus(workflow.status);
    setDirty(false);
  }, [workflow, setNodes, setEdges]);

  const selectedNode = useMemo(() => nodes.find((n) => n.id === selectedNodeId) ?? null, [nodes, selectedNodeId]);

  const saveMutation = useMutation({
    mutationFn: () =>
      updateWorkflow(id!, { name, description, definition: flowToDefinition(nodes, edges), status }),
    onSuccess: (wf) => {
      queryClient.invalidateQueries({ queryKey: ["workflow", id] });
      queryClient.invalidateQueries({ queryKey: ["workflows"] });
      setStatus(wf.status);
      setDirty(false);
    },
  });

  const onConnect = useCallback(
    (connection: FlowConnection) => {
      setEdges((eds) => addEdge({ ...connection, id: `e-${connection.source}-${connection.target}-${connection.targetHandle ?? "default"}` }, eds));
      setDirty(true);
    },
    [setEdges],
  );

  function handleDrop(e: React.DragEvent<HTMLDivElement>) {
    e.preventDefault();
    const type = e.dataTransfer.getData("application/de-node-type") as NodeType;
    if (!type) return;
    const bounds = e.currentTarget.getBoundingClientRect();
    const position = { x: e.clientX - bounds.left, y: e.clientY - bounds.top };
    nodeCounter += 1;
    const id = `${type}-${Date.now()}-${nodeCounter}`;
    const newNode: Node<FlowNodeData> = {
      id,
      type: "flowNode",
      position,
      data: { label: `${type} ${nodeCounter}`, nodeType: type, config: DEFAULT_CONFIG[type] } as unknown as FlowNodeData,
    };
    setNodes((nds) => [...nds, newNode]);
    setSelectedNodeId(id);
    setDirty(true);
  }

  function handleNodeChange(nodeId: string, patch: Partial<WorkflowNode>) {
    setNodes((nds) =>
      nds.map((n) => {
        if (n.id !== nodeId) return n;
        const data = n.data as unknown as { label: string; nodeType: NodeType; config: Record<string, unknown> };
        return {
          ...n,
          data: {
            ...data,
            label: patch.name ?? data.label,
            config: patch.config ?? data.config,
          } as unknown as FlowNodeData,
        };
      }),
    );
    setDirty(true);
  }

  function handleDeleteNode(nodeId: string) {
    setNodes((nds) => nds.filter((n) => n.id !== nodeId));
    setEdges((eds) => eds.filter((e) => e.source !== nodeId && e.target !== nodeId));
    setSelectedNodeId(null);
    setDirty(true);
  }

  async function handleRun() {
    if (!id) return;
    setRunning(true);
    setRunError(null);
    setRunResult(null);
    try {
      const res = await executeWorkflow(id);
      const resultsByNode = new Map((res.execution.nodeResults ?? []).map((r) => [r.nodeId, r]));
      setNodes((nds) =>
        nds.map((n) => {
          const r = resultsByNode.get(n.id);
          const data = n.data as unknown as FlowNodeData;
          return { ...n, data: { ...data, rowsOut: r?.rowsOut, error: r?.error } as unknown as FlowNodeData };
        }),
      );
      if (res.error) {
        setRunError(res.error);
      } else if (res.output) {
        setRunResult(res.output);
      }
      queryClient.invalidateQueries({ queryKey: ["workflow-executions", id] });
    } catch (err) {
      setRunError(extractErrorMessage(err));
    } finally {
      setRunning(false);
    }
  }

  if (isLoading || !workflow) {
    return <p>Loading…</p>;
  }

  return (
    <div>
      <div className="page-header">
        <div style={{ flex: 1 }}>
          <input
            className="input"
            style={{ fontSize: 18, fontWeight: 650, border: "none", padding: 0, background: "transparent" }}
            value={name}
            onChange={(e) => {
              setName(e.target.value);
              setDirty(true);
            }}
          />
          <p className="panel-subtitle" style={{ marginTop: 4 }}>
            <input
              className="input"
              style={{ border: "none", padding: 0, background: "transparent", fontSize: 12.5 }}
              placeholder="Add a description…"
              value={description}
              onChange={(e) => {
                setDescription(e.target.value);
                setDirty(true);
              }}
            />
          </p>
        </div>
        <div className="toolbar">
          <button className="btn" type="button" onClick={() => navigate("/workflows")}>
            Back
          </button>
          <select
            className="select"
            style={{ width: 110 }}
            value={status}
            onChange={(e) => {
              setStatus(e.target.value as WorkflowStatus);
              setDirty(true);
            }}
          >
            <option value="draft">Draft</option>
            <option value="published">Published</option>
          </select>
          <PermissionGate permission={PERMISSIONS.workflowsWrite}>
            <button className="btn btn-primary" type="button" onClick={() => saveMutation.mutate()} disabled={saveMutation.isPending}>
              {saveMutation.isPending ? "Saving…" : dirty ? "Save changes" : "Saved"}
            </button>
          </PermissionGate>
          <PermissionGate permission={PERMISSIONS.workflowsExecute}>
            <button className="btn btn-primary" type="button" onClick={handleRun} disabled={running}>
              <IconPlay width={13} height={13} /> {running ? "Running…" : "Run"}
            </button>
          </PermissionGate>
        </div>
      </div>

      {saveMutation.isError && <div className="error-banner">{extractErrorMessage(saveMutation.error)}</div>}
      {runError && <div className="error-banner">{runError}</div>}

      <div className="workflow-builder">
        <div className="workflow-palette">
          <div className="nav-section" style={{ padding: "0 0 8px" }}>
            Nodes
          </div>
          {PALETTE.map((item) => (
            <div
              key={item.type}
              className="palette-item"
              draggable
              onDragStart={(e) => e.dataTransfer.setData("application/de-node-type", item.type)}
            >
              {item.label}
            </div>
          ))}
        </div>

        <div className="workflow-canvas" onDrop={handleDrop} onDragOver={(e) => e.preventDefault()}>
          <ReactFlowProvider>
            <ReactFlow
              nodes={nodes}
              edges={edges}
              onNodesChange={(changes) => {
                onNodesChange(changes);
                setDirty(true);
              }}
              onEdgesChange={(changes) => {
                onEdgesChange(changes);
                setDirty(true);
              }}
              onConnect={onConnect}
              nodeTypes={nodeTypes}
              onNodeClick={(_, node) => setSelectedNodeId(node.id)}
              onPaneClick={() => setSelectedNodeId(null)}
              fitView
            >
              <Background gap={16} />
              <Controls />
              <MiniMap pannable zoomable style={{ background: "var(--bg-surface)" }} />
            </ReactFlow>
          </ReactFlowProvider>
        </div>

        {selectedNode ? (
          <NodeConfigPanel
            node={{
              id: selectedNode.id,
              type: selectedNode.data.nodeType,
              name: selectedNode.data.label,
              config: (selectedNode.data as unknown as { config: Record<string, unknown> }).config,
            }}
            onChange={handleNodeChange}
            onClose={() => setSelectedNodeId(null)}
            onDelete={handleDeleteNode}
          />
        ) : (
          <div className="workflow-config-panel">
            <div className="card-header">
              <h3>Execution history</h3>
            </div>
            <div className="card-body" style={{ flex: 1 }}>
              {executions.length === 0 && <p className="field-hint">No runs yet. Click Run to execute this workflow.</p>}
              {executions.map((ex) => (
                <div className="execution-row" key={ex.id}>
                  <div>
                    <StatusBadge status={ex.status} />
                    <div style={{ color: "var(--text-tertiary)", fontSize: 10.5, marginTop: 2 }}>
                      {new Date(ex.startedAt).toLocaleString()}
                    </div>
                  </div>
                  <span>{ex.durationMs}ms</span>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>

      {runResult && (
        <div className="card" style={{ marginTop: 16 }}>
          <div className="card-header">
            <h3>Run output</h3>
          </div>
          <div className="card-body">
            <DataFrameView frame={runResult} />
          </div>
        </div>
      )}
    </div>
  );
}
