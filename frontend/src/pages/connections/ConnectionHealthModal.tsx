import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";

import { Modal } from "../../components/Modal";
import { StatusBadge } from "../../components/StatusBadge";
import { listConnections, testConnection } from "../../api/connections";
import { listAuditLogs } from "../../api/audit";
import { extractErrorMessage } from "../../api/client";
import type { Connection } from "../../api/types";
import { Badge, Button } from "../../components/ui";

// ErrorCode -> a plain-language label for the badge (see backend/internal/connections.ErrorCode).
const ERROR_CODE_LABEL: Record<string, string> = {
  timeout: "Timeout",
  network_unreachable: "Network unreachable",
  auth_failed: "Authentication failed",
  permission_denied: "Permission denied",
  not_found: "Not found",
  rate_limited: "Rate limited",
  invalid_config: "Invalid configuration",
  unknown: "Unknown error",
};

function formatDuration(ms?: number): string {
  if (ms == null) return "";
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(2)}s`;
}

export function ConnectionHealthModal({ connection: initial, onClose }: { connection: Connection; onClose: () => void }) {
  const queryClient = useQueryClient();
  const [running, setRunning] = useState(false);
  const [runError, setRunError] = useState<string | null>(null);

  // Subscribes to the same "connections" cache the list page uses, so this
  // modal reflects a fresh status/error immediately after running a check
  // (rather than the snapshot passed in when it was opened).
  const { data: connections } = useQuery({ queryKey: ["connections"], queryFn: listConnections });
  const connection = connections?.find((c) => c.id === initial.id) ?? initial;

  const historyQuery = useQuery({
    queryKey: ["audit-logs", "connection-health", connection.id],
    queryFn: () => listAuditLogs({ resourceType: "connection", resourceId: connection.id, action: "connection.test", limit: 10 }),
  });

  async function runHealthCheck() {
    setRunning(true);
    setRunError(null);
    try {
      await testConnection(connection.id);
    } catch (err) {
      setRunError(extractErrorMessage(err));
    } finally {
      setRunning(false);
      queryClient.invalidateQueries({ queryKey: ["connections"] });
      queryClient.invalidateQueries({ queryKey: ["audit-logs", "connection-health", connection.id] });
    }
  }

  const history = historyQuery.data?.items ?? [];

  return (
    <Modal title={`Health – ${connection.name}`} onClose={onClose} width={560}>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 14 }}>
        <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
          <StatusBadge status={connection.status} />
          <span className="field-hint" style={{ margin: 0 }}>
            {connection.lastTestedAt
              ? `Last checked ${new Date(connection.lastTestedAt).toLocaleString()}${
                  connection.lastCheckDurationMs != null ? ` · took ${formatDuration(connection.lastCheckDurationMs)}` : ""
                }`
              : "Never checked"}
          </span>
        </div>
        <Button variant="primary" size="sm" onClick={runHealthCheck} disabled={running}>
          {running ? "Checking..." : "Run health check"}
        </Button>
      </div>

      {runError && <div className="error-banner">{runError}</div>}

      {connection.status === "unhealthy" && connection.lastError && (
        <div className="error-banner" style={{ display: "flex", flexDirection: "column", gap: 6 }}>
          <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
            {connection.lastErrorCode && <Badge tone="danger">{ERROR_CODE_LABEL[connection.lastErrorCode] ?? connection.lastErrorCode}</Badge>}
            <strong>{connection.lastError}</strong>
          </div>
          {connection.lastErrorRemediation && (
            <div>
              <span style={{ color: "var(--text-tertiary)" }}>Next step: </span>
              {connection.lastErrorRemediation}
            </div>
          )}
        </div>
      )}

      <h4 style={{ margin: "16px 0 8px" }}>Recent checks</h4>
      {historyQuery.isLoading && <p className="field-hint">Loading…</p>}
      {!historyQuery.isLoading && history.length === 0 && <p className="empty-state">No health checks recorded yet.</p>}
      {history.length > 0 && (
        <div className="table-wrap">
          <table className="data-table">
            <thead>
              <tr>
                <th>Time</th>
                <th>Outcome</th>
                <th>Detail</th>
              </tr>
            </thead>
            <tbody>
              {history.map((log) => (
                <tr key={log.id}>
                  <td>{new Date(log.createdAt).toLocaleString()}</td>
                  <td>
                    <Badge tone={log.outcome === "success" ? "success" : "danger"}>{log.outcome === "success" ? "healthy" : "unhealthy"}</Badge>
                  </td>
                  <td style={{ color: "var(--text-tertiary)", fontSize: 11.5 }}>{log.metadata?.error ? String(log.metadata.error) : "—"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </Modal>
  );
}
