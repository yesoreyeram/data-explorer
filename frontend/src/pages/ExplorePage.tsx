import { useState } from "react";
import { useQuery } from "@tanstack/react-query";

import { listConnections } from "../api/connections";
import { exploreQuery } from "../api/explore";
import { extractErrorMessage } from "../api/client";
import { useAuthStore } from "../state/authStore";
import { PERMISSIONS } from "../lib/permissions";
import { useConnectionFields } from "../lib/connectionFields";
import { buildQuerySpec, defaultQueryFormState, summarizeQuery } from "../lib/querySpec";
import { clearExploreHistory, loadExploreHistory, pushExploreHistory, type ExploreHistoryEntry } from "../lib/exploreHistory";
import type { ConnectionType, DataFrame } from "../api/types";
import { DataFrameView } from "../components/DataFrameView";
import { QuerySpecFields } from "../components/QuerySpecFields";
import { IconTrash } from "../components/icons";
import { ConnectionTypeConfigFields } from "./connections/ConnectionTypeConfigFields";
import { Button, Card, CardBody, Field, IconButton, Input, Select } from "../components/ui";

type SourceMode = "saved" | "temporary";

const TYPE_LABELS: Record<ConnectionType, string> = {
  postgres: "PostgreSQL",
  mysql: "MySQL",
  rest: "REST API",
  graphql: "GraphQL API",
  aws: "AWS",
  gcp: "Google Cloud",
  azure: "Microsoft Azure",
};

export function ExplorePage() {
  const hasPermission = useAuthStore((s) => s.hasPermission);
  const canUseTemporary = hasPermission(PERMISSIONS.connectionsTest);

  const { data: connections = [] } = useQuery({ queryKey: ["connections"], queryFn: listConnections });

  const [mode, setMode] = useState<SourceMode>("saved");
  const [connectionId, setConnectionId] = useState("");
  const selectedConnection = connections.find((c) => c.id === connectionId);

  const temp = useConnectionFields();

  const [queryForm, setQueryForm] = useState(defaultQueryFormState());
  function patchQueryForm(patch: Partial<typeof queryForm>) {
    setQueryForm((prev) => ({ ...prev, ...patch }));
  }

  const [result, setResult] = useState<DataFrame | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [running, setRunning] = useState(false);
  const [history, setHistory] = useState<ExploreHistoryEntry[]>(() => loadExploreHistory());

  function clearResult() {
    setResult(null);
    setError(null);
  }

  function selectMode(m: SourceMode) {
    setMode(m);
    clearResult();
  }
  function selectConnectionId(id: string) {
    setConnectionId(id);
    clearResult();
  }
  function selectTempType(t: ConnectionType) {
    temp.setType(t);
    clearResult();
  }

  function applyHistoryEntry(entry: ExploreHistoryEntry) {
    setMode("saved");
    setConnectionId(entry.connectionId);
    setQueryForm(entry.queryForm);
    clearResult();
  }

  const effectiveType: ConnectionType | "" = mode === "saved" ? (selectedConnection?.type ?? "") : temp.type;
  const effectiveCloudService = String((mode === "saved" ? selectedConnection?.config.service : temp.cloudConfig.service) ?? "");
  const canRun = mode === "saved" ? Boolean(selectedConnection) : true;

  async function runQuery() {
    if (!canRun) return;
    setRunning(true);
    setError(null);
    setResult(null);
    try {
      const spec = buildQuerySpec(effectiveType, queryForm);
      const res =
        mode === "saved"
          ? await exploreQuery({ connectionId, spec })
          : await exploreQuery({ connection: { type: temp.type, ...temp.buildConfigAndSecret() }, spec });
      setResult(res);
      if (mode === "saved" && selectedConnection) {
        setHistory(
          pushExploreHistory({
            connectionId,
            connectionLabel: `${selectedConnection.name} (${selectedConnection.type})`,
            summary: summarizeQuery(effectiveType, queryForm),
            queryForm,
          }),
        );
      }
    } catch (err) {
      setError(extractErrorMessage(err));
    } finally {
      setRunning(false);
    }
  }

  return (
    <div>
      <div className="page-header">
        <div>
          <h1 className="panel-title">Explore</h1>
          <p className="panel-subtitle">
            Query a saved connection, or a temporary one that's never persisted - author a query and see results right here.
          </p>
        </div>
      </div>

      <Card style={{ marginBottom: 12 }}>
        <CardBody>
          <div className="toolbar" style={{ marginBottom: 12 }}>
            <Button variant={mode === "saved" ? "primary" : "default"} onClick={() => selectMode("saved")}>
              Saved connection
            </Button>
            {canUseTemporary && (
              <Button variant={mode === "temporary" ? "primary" : "default"} onClick={() => selectMode("temporary")}>
                Temporary connection
              </Button>
            )}
          </div>

          {mode === "saved" ? (
            <Field htmlFor="explore-connection" label="Connection">
              <Select id="explore-connection" value={connectionId} onChange={(e) => selectConnectionId(e.target.value)}>
                <option value="" disabled>
                  Select a connection…
                </option>
                {connections.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.name} ({c.type})
                  </option>
                ))}
              </Select>
            </Field>
          ) : (
            <>
              <Field htmlFor="explore-type" label="Type">
                <Select id="explore-type" value={temp.type} onChange={(e) => selectTempType(e.target.value as ConnectionType)}>
                  {Object.entries(TYPE_LABELS).map(([value, label]) => (
                    <option key={value} value={value}>
                      {label}
                    </option>
                  ))}
                </Select>
              </Field>
              <p className="field-hint" style={{ marginBottom: 10 }}>
                Nothing here is saved - the connection details and any credentials you enter are sent directly with your
                query and never stored.
              </p>
              <ConnectionTypeConfigFields {...temp} isEdit={false} />
            </>
          )}
        </CardBody>
      </Card>

      {history.length > 0 && (
        <Card style={{ marginBottom: 12 }}>
          <CardBody>
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 8 }}>
              <strong style={{ fontSize: 12 }}>Recent queries</strong>
              <IconButton
                label="Clear recent queries"
                onClick={() => setHistory(clearExploreHistory())}
              >
                <IconTrash width={13} height={13} />
              </IconButton>
            </div>
            <div style={{ display: "flex", flexDirection: "column", gap: 2 }}>
              {history.map((entry) => (
                <button
                  key={entry.id}
                  type="button"
                  onClick={() => applyHistoryEntry(entry)}
                  style={{
                    display: "flex",
                    justifyContent: "space-between",
                    gap: 12,
                    textAlign: "left",
                    padding: "6px 8px",
                    border: "none",
                    borderRadius: "var(--radius-sm)",
                    background: "transparent",
                    cursor: "pointer",
                    font: "inherit",
                  }}
                  onMouseEnter={(e) => (e.currentTarget.style.background = "var(--bg-hover)")}
                  onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
                >
                  <span className="mono" style={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                    {entry.summary || "(empty query)"}
                  </span>
                  <span style={{ color: "var(--text-tertiary)", fontSize: 11, flexShrink: 0 }}>
                    {entry.connectionLabel} · {new Date(entry.ranAt).toLocaleTimeString()}
                  </span>
                </button>
              ))}
            </div>
          </CardBody>
        </Card>
      )}

      {effectiveType && (
        <Card style={{ marginBottom: 12 }}>
          <CardBody>
            <QuerySpecFields
              type={effectiveType}
              cloudService={effectiveCloudService}
              value={queryForm}
              onChange={patchQueryForm}
              idPrefix="explore-query"
            />
            <div className="toolbar" style={{ marginTop: 4 }}>
              <Field htmlFor="explore-limit" label="Row limit" style={{ margin: 0, width: 120 }}>
                <Input
                  id="explore-limit"
                  type="number"
                  min={1}
                  max={10000}
                  value={queryForm.rowLimit}
                  onChange={(e) => patchQueryForm({ rowLimit: Number(e.target.value) })}
                />
              </Field>
              <Button variant="primary" onClick={runQuery} disabled={running || !canRun} style={{ alignSelf: "flex-end" }}>
                {running ? "Running..." : "Run"}
              </Button>
            </div>
          </CardBody>
        </Card>
      )}

      {error && <div className="error-banner">{error}</div>}
      {result && <DataFrameView frame={result} />}
    </div>
  );
}
