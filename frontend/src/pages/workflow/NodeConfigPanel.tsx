import { useQuery } from "@tanstack/react-query";

import { listConnections } from "../../api/connections";
import type { WorkflowNode } from "../../api/types";
import { IconTrash, IconX } from "../../components/icons";

interface NodeConfigPanelProps {
  node: WorkflowNode;
  onChange: (id: string, patch: Partial<WorkflowNode>) => void;
  onClose: () => void;
  onDelete: (id: string) => void;
}

function updateConfig(node: WorkflowNode, patch: Record<string, unknown>) {
  return { ...node.config, ...patch };
}

export function NodeConfigPanel({ node, onChange, onClose, onDelete }: NodeConfigPanelProps) {
  const { data: connections = [] } = useQuery({
    queryKey: ["connections"],
    queryFn: listConnections,
    enabled: node.type === "source",
  });

  return (
    <div className="workflow-config-panel">
      <div className="card-header">
        <h3>Configure node</h3>
        <div style={{ display: "flex", gap: 4 }}>
          <button className="icon-btn" title="Delete node" onClick={() => onDelete(node.id)}>
            <IconTrash width={14} height={14} />
          </button>
          <button className="icon-btn" title="Close" onClick={onClose}>
            <IconX width={14} height={14} />
          </button>
        </div>
      </div>
      <div className="card-body" style={{ flex: 1 }}>
        <div className="field">
          <label htmlFor="node-name">Name</label>
          <input
            id="node-name"
            className="input"
            value={node.name}
            onChange={(e) => onChange(node.id, { name: e.target.value })}
          />
        </div>

        {node.type === "source" && (
          <SourceForm node={node} onChange={onChange} connections={connections} />
        )}
        {(node.type === "transform" || node.type === "filter") && <ExpressionForm node={node} onChange={onChange} />}
        {node.type === "join" && <JoinForm node={node} onChange={onChange} />}
        {node.type === "aggregate" && <AggregateForm node={node} onChange={onChange} />}
        {node.type === "output" && <p className="field-hint">Marks the final result of this branch. No configuration needed.</p>}
      </div>
    </div>
  );
}

function SourceForm({
  node,
  onChange,
  connections,
}: {
  node: WorkflowNode;
  onChange: NodeConfigPanelProps["onChange"];
  connections: { id: string; name: string; type: string }[];
}) {
  const cfg = node.config as { connectionId?: string; query?: { sql?: string; method?: string; path?: string; rowLimit?: number } };
  const query = cfg.query ?? {};
  const selected = connections.find((c) => c.id === cfg.connectionId);
  const isSQL = selected ? selected.type === "postgres" || selected.type === "mysql" : true;

  function setQuery(patch: Record<string, unknown>) {
    onChange(node.id, { config: updateConfig(node, { query: { ...query, ...patch } }) });
  }

  return (
    <>
      <div className="field">
        <label htmlFor="src-conn">Connection</label>
        <select
          id="src-conn"
          className="select"
          value={cfg.connectionId ?? ""}
          onChange={(e) => onChange(node.id, { config: updateConfig(node, { connectionId: e.target.value }) })}
        >
          <option value="" disabled>
            Select a connection…
          </option>
          {connections.map((c) => (
            <option key={c.id} value={c.id}>
              {c.name} ({c.type})
            </option>
          ))}
        </select>
      </div>

      {isSQL ? (
        <div className="field">
          <label htmlFor="src-sql">SQL (SELECT only)</label>
          <textarea
            id="src-sql"
            className="textarea"
            rows={5}
            value={query.sql ?? ""}
            onChange={(e) => setQuery({ sql: e.target.value })}
          />
        </div>
      ) : (
        <div style={{ display: "grid", gridTemplateColumns: "90px 1fr", gap: 8 }}>
          <div className="field">
            <label htmlFor="src-method">Method</label>
            <select id="src-method" className="select" value={query.method ?? "GET"} onChange={(e) => setQuery({ method: e.target.value })}>
              <option>GET</option>
              <option>POST</option>
            </select>
          </div>
          <div className="field">
            <label htmlFor="src-path">Path</label>
            <input id="src-path" className="input" value={query.path ?? ""} onChange={(e) => setQuery({ path: e.target.value })} />
          </div>
        </div>
      )}

      <div className="field">
        <label htmlFor="src-limit">Row limit</label>
        <input
          id="src-limit"
          className="input"
          type="number"
          min={1}
          max={10000}
          value={query.rowLimit ?? 1000}
          onChange={(e) => setQuery({ rowLimit: Number(e.target.value) })}
        />
      </div>
    </>
  );
}

function ExpressionForm({ node, onChange }: { node: WorkflowNode; onChange: NodeConfigPanelProps["onChange"] }) {
  const cfg = node.config as { expression?: string };
  const isFilter = node.type === "filter";
  return (
    <div className="field">
      <label htmlFor="expr">JSONata expression</label>
      <textarea
        id="expr"
        className="textarea"
        rows={6}
        placeholder={isFilter ? "amount > 100 and status = \"active\"" : '{"fullName": name & " " & surname}'}
        value={cfg.expression ?? ""}
        onChange={(e) => onChange(node.id, { config: updateConfig(node, { expression: e.target.value }) })}
      />
      <span className="field-hint">
        {isFilter
          ? "Evaluated once per row against that row; rows where it's truthy are kept."
          : "Evaluated once per row against that row; the result becomes the new row shape."}
      </span>
    </div>
  );
}

function JoinForm({ node, onChange }: { node: WorkflowNode; onChange: NodeConfigPanelProps["onChange"] }) {
  const cfg = node.config as { leftKey?: string; rightKey?: string; type?: string; rightPrefix?: string };
  return (
    <>
      <p className="field-hint" style={{ marginBottom: 10 }}>
        Wire the two upstream branches into this node&rsquo;s left and right handles on the canvas.
      </p>
      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 8 }}>
        <div className="field">
          <label htmlFor="join-left">Left key</label>
          <input
            id="join-left"
            className="input"
            value={cfg.leftKey ?? ""}
            onChange={(e) => onChange(node.id, { config: updateConfig(node, { leftKey: e.target.value }) })}
          />
        </div>
        <div className="field">
          <label htmlFor="join-right">Right key</label>
          <input
            id="join-right"
            className="input"
            value={cfg.rightKey ?? ""}
            onChange={(e) => onChange(node.id, { config: updateConfig(node, { rightKey: e.target.value }) })}
          />
        </div>
      </div>
      <div className="field">
        <label htmlFor="join-type">Join type</label>
        <select
          id="join-type"
          className="select"
          value={cfg.type ?? "inner"}
          onChange={(e) => onChange(node.id, { config: updateConfig(node, { type: e.target.value }) })}
        >
          <option value="inner">Inner</option>
          <option value="left">Left</option>
        </select>
      </div>
      <div className="field">
        <label htmlFor="join-prefix">Right column prefix</label>
        <input
          id="join-prefix"
          className="input"
          placeholder="right_"
          value={cfg.rightPrefix ?? ""}
          onChange={(e) => onChange(node.id, { config: updateConfig(node, { rightPrefix: e.target.value }) })}
        />
      </div>
    </>
  );
}

interface Aggregation {
  field: string;
  op: string;
  as: string;
}

function AggregateForm({ node, onChange }: { node: WorkflowNode; onChange: NodeConfigPanelProps["onChange"] }) {
  const cfg = node.config as { groupBy?: string[]; aggregations?: Aggregation[] };
  const groupBy = cfg.groupBy ?? [];
  const aggregations = cfg.aggregations ?? [];

  function setAggregations(next: Aggregation[]) {
    onChange(node.id, { config: updateConfig(node, { aggregations: next }) });
  }

  return (
    <>
      <div className="field">
        <label htmlFor="agg-groupby">Group by (comma-separated columns)</label>
        <input
          id="agg-groupby"
          className="input"
          value={groupBy.join(", ")}
          onChange={(e) =>
            onChange(node.id, {
              config: updateConfig(node, {
                groupBy: e.target.value
                  .split(",")
                  .map((s) => s.trim())
                  .filter(Boolean),
              }),
            })
          }
        />
      </div>

      <label>Aggregations</label>
      {aggregations.map((agg, i) => (
        <div className="agg-row" key={i}>
          <input
            className="input"
            placeholder="field"
            value={agg.field}
            onChange={(e) => setAggregations(aggregations.map((a, idx) => (idx === i ? { ...a, field: e.target.value } : a)))}
          />
          <select
            className="select"
            value={agg.op}
            onChange={(e) => setAggregations(aggregations.map((a, idx) => (idx === i ? { ...a, op: e.target.value } : a)))}
          >
            <option value="sum">sum</option>
            <option value="avg">avg</option>
            <option value="count">count</option>
            <option value="min">min</option>
            <option value="max">max</option>
          </select>
          <input
            className="input"
            placeholder="as"
            value={agg.as}
            onChange={(e) => setAggregations(aggregations.map((a, idx) => (idx === i ? { ...a, as: e.target.value } : a)))}
          />
          <button className="icon-btn" type="button" onClick={() => setAggregations(aggregations.filter((_, idx) => idx !== i))}>
            <IconTrash width={13} height={13} />
          </button>
        </div>
      ))}
      <button
        className="btn btn-sm"
        type="button"
        onClick={() => setAggregations([...aggregations, { field: "", op: "sum", as: "" }])}
      >
        Add aggregation
      </button>
    </>
  );
}
