import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";

import { listConnections } from "../api/connections";
import { listWorkflows } from "../api/workflows";
import { useAuthStore } from "../state/authStore";
import { PERMISSIONS } from "../lib/permissions";
import { StatusBadge } from "../components/StatusBadge";

export function DashboardPage() {
  const user = useAuthStore((s) => s.user);
  const hasPermission = useAuthStore((s) => s.hasPermission);

  const connectionsQuery = useQuery({
    queryKey: ["connections"],
    queryFn: listConnections,
    enabled: hasPermission(PERMISSIONS.connectionsRead),
  });
  const workflowsQuery = useQuery({
    queryKey: ["workflows"],
    queryFn: listWorkflows,
    enabled: hasPermission(PERMISSIONS.workflowsRead),
  });

  const connections = connectionsQuery.data ?? [];
  const workflows = workflowsQuery.data ?? [];
  const healthyConnections = connections.filter((c) => c.status === "healthy").length;
  const publishedWorkflows = workflows.filter((w) => w.status === "published").length;

  return (
    <div>
      <div className="page-header">
        <div>
          <h1 className="panel-title">Welcome back{user ? `, ${user.displayName.split(" ")[0]}` : ""}</h1>
          <p className="panel-subtitle">Here&rsquo;s what&rsquo;s happening across your workspace.</p>
        </div>
      </div>

      <div className="stat-grid">
        <div className="stat-tile">
          <div className="stat-label">Connections</div>
          <div className="stat-value">{connections.length}</div>
        </div>
        <div className="stat-tile">
          <div className="stat-label">Healthy connections</div>
          <div className="stat-value">{healthyConnections}</div>
        </div>
        <div className="stat-tile">
          <div className="stat-label">Workflows</div>
          <div className="stat-value">{workflows.length}</div>
        </div>
        <div className="stat-tile">
          <div className="stat-label">Published workflows</div>
          <div className="stat-value">{publishedWorkflows}</div>
        </div>
      </div>

      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 16 }}>
        <div className="card">
          <div className="card-header">
            <h3>Recent connections</h3>
            <Link to="/connections" className="btn btn-sm btn-ghost">
              View all
            </Link>
          </div>
          <div className="card-body">
            {connections.length === 0 ? (
              <div className="empty-state">No connections yet.</div>
            ) : (
              <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
                {connections.slice(0, 6).map((c) => (
                  <div key={c.id} style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
                    <span>
                      {c.name} <span style={{ color: "var(--text-tertiary)" }}>({c.type})</span>
                    </span>
                    <StatusBadge status={c.status} />
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>

        <div className="card">
          <div className="card-header">
            <h3>Recent workflows</h3>
            <Link to="/workflows" className="btn btn-sm btn-ghost">
              View all
            </Link>
          </div>
          <div className="card-body">
            {workflows.length === 0 ? (
              <div className="empty-state">No workflows yet.</div>
            ) : (
              <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
                {workflows.slice(0, 6).map((w) => (
                  <div key={w.id} style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
                    <span>{w.name}</span>
                    <StatusBadge status={w.status} />
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
