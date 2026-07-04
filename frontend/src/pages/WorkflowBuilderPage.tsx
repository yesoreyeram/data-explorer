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
import { executeWorkflow, getWorkflow, listWorkflowExecutions, setWorkflowSchedule, updateWorkflow } from "../api/workflows";
import { listFolders } from "../api/folders";
import type { NodeType, DataFrame, WorkflowDefinition, WorkflowEdge, WorkflowNode, WorkflowStatus } from "../api/types";
import { DataFrameView } from "../components/DataFrameView";
import { StatusBadge } from "../components/StatusBadge";
import { PERMISSIONS } from "../lib/permissions";
import { useAuthStore } from "../state/authStore";
import { IconClock, IconPlay } from "../components/icons";
import { Modal } from "../components/Modal";
import { FlowNode, type FlowNodeData } from "./workflow/FlowNode";
import { NodeConfigPanel } from "./workflow/NodeConfigPanel";
import { Badge, Button, Field, Input } from "../components/ui";

const CRON_PRESETS: { label: string; cron: string }[] = [
  { label: "Every 5 minutes", cron: "*/5 * * * *" },
  { label: "Hourly", cron: "0 * * * *" },
  { label: "Daily at midnight", cron: "0 0 * * *" },
  { label: "Weekdays at 9am", cron: "0 9 * * 1-5" },
];

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

function ScheduleSkipSparkline({ executions }: { executions: Awaited<ReturnType<typeof listWorkflowExecutions>> }) {
  const skipped = executions.filter((ex) => ex.status === "skipped");
  if (skipped.length === 0) {
    return <p className="field-hint">No skipped scheduled runs in the recent execution window.</p>;
  }
  const buckets = Array.from({ length: 12 }, (_, i) => {
    const start = Date.now() - (12 - i) * 2 * 60 * 60 * 1000;
    const end = start + 2 * 60 * 60 * 1000;
    return skipped.filter((ex) => {
      const t = new Date(ex.startedAt).getTime();
      return t >= start && t < end;
    }).length;
  });
  const max = Math.max(...buckets, 1);
  return (
    <div className="field-hint" aria-label={`${skipped.length} skipped scheduled runs in recent history`}>
      <div style={{ display: "flex", alignItems: "end", gap: 2, height: 28, marginBottom: 4 }} aria-hidden="true">
        {buckets.map((n, i) => (
          <span
            // eslint-disable-next-line react/no-array-index-key
            key={i}
            style={{ width: 8, height: Math.max(2, (n / max) * 24), borderRadius: 2, background: n > 0 ? "var(--warning)" : "var(--border)" }}
          />
        ))}
      </div>
      {skipped.length} skipped scheduled run{skipped.length === 1 ? "" : "s"} recently; latest reason: {skipped[0]?.error || "overlap"}.
    </div>
  );
}

let nodeCounter = 0;

export function WorkflowBuilderPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const canWriteWorkflows = useAuthStore((s) => s.hasPermission(PERMISSIONS.workflowsWrite));

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
  const { data: folders = [] } = useQuery({ queryKey: ["folders"], queryFn: () => listFolders() });
  const hasScopedPermission = useAuthStore((s) => s.hasScopedPermission);
  const scopeChain = (() => {
    if (!workflow) return [];
    const folder = folders.find((f) => f.id === workflow.folderId);
    return folder ? [folder.id, ...folder.ancestorIds] : [workflow.folderId];
  })();

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
  const [scheduleOpen, setScheduleOpen] = useState(false);
  const [scheduleCron, setScheduleCron] = useState("");
  const [scheduleEnabled, setScheduleEnabled] = useState(false);

  useEffect(() => {
    if (!workflow) return;
    const flow = definitionToFlow(workflow.definition);
    setNodes(flow.nodes);
    setEdges(flow.edges);
    setName(workflow.name);
    setDescription(workflow.description);
    setStatus(workflow.status);
    setScheduleCron(workflow.scheduleCron ?? "");
    setScheduleEnabled(workflow.scheduleEnabled);
    setDirty(false);
  }, [workflow, setNodes, setEdges]);

  const selectedNode = useMemo(() => nodes.find((n) => n.id === selectedNodeId) ?? null, [nodes, selectedNodeId]);

  const saveMutation = useMutation({
    mutationFn: () =>
      updateWorkflow(id!, { name, description, definition: flowToDefinition(nodes, edges), status, folderId: workflow?.folderId ?? "" }),
    onSuccess: (wf) => {
      queryClient.invalidateQueries({ queryKey: ["workflow", id] });
      queryClient.invalidateQueries({ queryKey: ["workflows"] });
      setStatus(wf.status);
      setDirty(false);
    },
  });

  const scheduleMutation = useMutation({
    mutationFn: (input: { cron: string; enabled: boolean }) => setWorkflowSchedule(id!, input.cron, input.enabled),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["workflow", id] });
      queryClient.invalidateQueries({ queryKey: ["workflows"] });
      setScheduleOpen(false);
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
    const deadlineAt = new Date(Date.now() + 60_000).toISOString();
    setNodes((nds) => nds.map((n) => ({ ...n, data: { ...n.data, runActive: true, deadlineAt, error: undefined, rowsOut: undefined } })));
    try {
      const res = await executeWorkflow(id);
      const resultsByNode = new Map((res.execution.nodeResults ?? []).map((r) => [r.nodeId, r]));
      setNodes((nds) =>
        nds.map((n) => {
          const r = resultsByNode.get(n.id);
          const data = n.data as unknown as FlowNodeData;
          return { ...n, data: { ...data, rowsOut: r?.rowsOut, rowCap: r?.rowCap, truncated: r?.truncated, warnings: r?.warnings, error: r?.error, runActive: false, deadlineAt: undefined } as unknown as FlowNodeData };
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
      setNodes((nds) => nds.map((n) => ({ ...n, data: { ...n.data, runActive: false, deadlineAt: undefined } })));
    }
  }

  function addFilterNodeAfterCappedResult() {
    const source = nodes.find((n) => n.data.truncated || (typeof n.data.rowsOut === "number" && n.data.rowsOut >= (n.data.rowCap ?? 100_000) * 0.8)) ?? nodes[nodes.length - 1];
    if (!source) return;
    nodeCounter += 1;
    const id = `filter-${Date.now()}-${nodeCounter}`;
    const newNode: Node<FlowNodeData> = {
      id,
      type: "flowNode",
      position: { x: source.position.x + 220, y: source.position.y },
      data: { label: `filter ${nodeCounter}`, nodeType: "filter", config: { expression: "true" } } as unknown as FlowNodeData,
    };
    setNodes((nds) => [...nds, newNode]);
    setEdges((eds) => [...eds, { id: `e-${source.id}-${id}`, source: source.id, target: id }]);
    setSelectedNodeId(id);
    setDirty(true);
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
          {workflow.scheduleEnabled && (
            <Badge>
              <IconClock width={11} height={11} /> scheduled
            </Badge>
          )}
          <Button onClick={() => navigate("/workflows")}>Back</Button>
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
          {hasScopedPermission(PERMISSIONS.workflowsWrite, scopeChain) && (
            <Button onClick={() => setScheduleOpen(true)}>
              <IconClock width={13} height={13} /> Schedule
            </Button>
          )}
          {hasScopedPermission(PERMISSIONS.workflowsWrite, scopeChain) && (
            <Button variant="primary" onClick={() => saveMutation.mutate()} disabled={saveMutation.isPending}>
              {saveMutation.isPending ? "Saving…" : dirty ? "Save changes" : "Saved"}
            </Button>
          )}
          {hasScopedPermission(PERMISSIONS.workflowsExecute, scopeChain) && (
            <Button variant="primary" onClick={handleRun} disabled={running}>
              <IconPlay width={13} height={13} /> {running ? "Running…" : "Run"}
            </Button>
          )}
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
              {executions.length > 0 && (
                <div style={{ marginBottom: 12 }}>
                  <strong style={{ fontSize: 12 }}>Scheduled skips</strong>
                  <ScheduleSkipSparkline executions={executions} />
                </div>
              )}
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
            <DataFrameView frame={runResult} onAddFilterNode={canWriteWorkflows ? addFilterNodeAfterCappedResult : undefined} />
          </div>
        </div>
      )}

      {scheduleOpen && (
        <Modal
          title="Schedule"
          onClose={() => setScheduleOpen(false)}
          footer={
            <>
              <Button onClick={() => setScheduleOpen(false)}>Cancel</Button>
              <Button
                variant="primary"
                disabled={scheduleMutation.isPending || (scheduleEnabled && scheduleCron.trim() === "")}
                onClick={() => scheduleMutation.mutate({ cron: scheduleCron, enabled: scheduleEnabled })}
              >
                {scheduleMutation.isPending ? "Saving…" : "Save schedule"}
              </Button>
            </>
          }
        >
          {scheduleMutation.isError && <div className="error-banner">{extractErrorMessage(scheduleMutation.error)}</div>}

          <label className="checkbox-row" style={{ marginBottom: 12 }}>
            <input type="checkbox" checked={scheduleEnabled} onChange={(e) => setScheduleEnabled(e.target.checked)} />
            <span>Run this workflow automatically on a schedule</span>
          </label>

          <Field htmlFor="schedule-cron" label="Cron expression" hint="Standard 5-field cron: minute hour day-of-month month day-of-week.">
            <Input
              id="schedule-cron"
              placeholder="*/5 * * * *"
              value={scheduleCron}
              onChange={(e) => setScheduleCron(e.target.value)}
              disabled={!scheduleEnabled}
            />
          </Field>

          <div style={{ display: "flex", flexWrap: "wrap", gap: 6, marginBottom: 12 }}>
            {CRON_PRESETS.map((preset) => (
              <Button key={preset.cron} size="sm" disabled={!scheduleEnabled} onClick={() => setScheduleCron(preset.cron)}>
                {preset.label}
              </Button>
            ))}
          </div>

          {workflow.scheduleEnabled && (
            <p className="field-hint">
              {workflow.scheduleLastRun && <>Last run: {new Date(workflow.scheduleLastRun).toLocaleString()}. </>}
              {workflow.scheduleNextRun && <>Next run: {new Date(workflow.scheduleNextRun).toLocaleString()}.</>}
            </p>
          )}
        </Modal>
      )}
    </div>
  );
}
