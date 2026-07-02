import { useState } from "react";
import { useQuery } from "@tanstack/react-query";

import { listAuditLogs, type AuditLogFilter } from "../api/audit";
import { extractErrorMessage } from "../api/client";

const PAGE_SIZE = 50;

export function AuditLogPage() {
  const [filter, setFilter] = useState<AuditLogFilter>({ limit: PAGE_SIZE, offset: 0 });

  const { data, isLoading, error } = useQuery({
    queryKey: ["audit-logs", filter],
    queryFn: () => listAuditLogs(filter),
  });

  const logs = data?.items ?? [];
  const total = data?.total ?? 0;

  function updateFilter(patch: Partial<AuditLogFilter>) {
    setFilter((f) => ({ ...f, ...patch, offset: 0 }));
  }

  return (
    <div>
      <div className="page-header">
        <div>
          <h1 className="panel-title">Audit log</h1>
          <p className="panel-subtitle">Append-only record of who did what, from where, and whether it succeeded.</p>
        </div>
      </div>

      <div className="toolbar" style={{ marginBottom: 12 }}>
        <input
          className="input"
          style={{ width: 160 }}
          placeholder="Action (e.g. connection.create)"
          value={filter.action ?? ""}
          onChange={(e) => updateFilter({ action: e.target.value || undefined })}
        />
        <input
          className="input"
          style={{ width: 160 }}
          placeholder="Resource type"
          value={filter.resourceType ?? ""}
          onChange={(e) => updateFilter({ resourceType: e.target.value || undefined })}
        />
        <input
          className="input"
          style={{ width: 200 }}
          placeholder="Actor ID"
          value={filter.actorId ?? ""}
          onChange={(e) => updateFilter({ actorId: e.target.value || undefined })}
        />
      </div>

      {error && <div className="error-banner">{extractErrorMessage(error)}</div>}

      <div className="table-wrap">
        <table className="data-table">
          <thead>
            <tr>
              <th>Time</th>
              <th>Actor</th>
              <th>Action</th>
              <th>Resource</th>
              <th>Outcome</th>
              <th>IP address</th>
            </tr>
          </thead>
          <tbody>
            {isLoading && (
              <tr>
                <td colSpan={6}>Loading…</td>
              </tr>
            )}
            {!isLoading && logs.length === 0 && (
              <tr>
                <td colSpan={6} className="empty-state">
                  No audit entries match this filter.
                </td>
              </tr>
            )}
            {logs.map((log) => (
              <tr key={log.id}>
                <td>{new Date(log.createdAt).toLocaleString()}</td>
                <td>{log.actorEmail || "system"}</td>
                <td className="mono">{log.action}</td>
                <td>
                  {log.resourceType}
                  {log.resourceId && <span style={{ color: "var(--text-tertiary)" }}> · {log.resourceId.slice(0, 8)}</span>}
                </td>
                <td>
                  <span className={`badge ${log.outcome === "success" ? "badge-success" : "badge-danger"}`}>{log.outcome}</span>
                </td>
                <td className="mono">{log.ipAddress}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="toolbar" style={{ marginTop: 12, justifyContent: "space-between" }}>
        <span className="field-hint">
          {total > 0 ? `Showing ${(filter.offset ?? 0) + 1}-${Math.min((filter.offset ?? 0) + PAGE_SIZE, total)} of ${total}` : ""}
        </span>
        <div className="toolbar">
          <button
            className="btn btn-sm"
            type="button"
            disabled={(filter.offset ?? 0) === 0}
            onClick={() => setFilter((f) => ({ ...f, offset: Math.max(0, (f.offset ?? 0) - PAGE_SIZE) }))}
          >
            Previous
          </button>
          <button
            className="btn btn-sm"
            type="button"
            disabled={(filter.offset ?? 0) + PAGE_SIZE >= total}
            onClick={() => setFilter((f) => ({ ...f, offset: (f.offset ?? 0) + PAGE_SIZE }))}
          >
            Next
          </button>
        </div>
      </div>
    </div>
  );
}
