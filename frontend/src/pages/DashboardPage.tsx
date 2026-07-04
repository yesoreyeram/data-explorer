import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";

import { listConnections } from "../api/connections";
import { listWorkflows } from "../api/workflows";
import { useAuthStore } from "../state/authStore";
import { PERMISSIONS } from "../lib/permissions";
import { StatusBadge } from "../components/StatusBadge";
import { Card, CardBody, CardHeader, CardTitle, StatTile } from "../components/ui";

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
        <StatTile label="Connections" value={connections.length} />
        <StatTile label="Healthy connections" value={healthyConnections} />
        <StatTile label="Workflows" value={workflows.length} />
        <StatTile label="Published workflows" value={publishedWorkflows} />
      </div>

      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
        <Card>
          <CardHeader>
            <CardTitle>Recent connections</CardTitle>
            <Link to="/connections" className="btn btn-sm btn-ghost">
              View all
            </Link>
          </CardHeader>
          <CardBody>
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
          </CardBody>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Recent workflows</CardTitle>
            <Link to="/workflows" className="btn btn-sm btn-ghost">
              View all
            </Link>
          </CardHeader>
          <CardBody>
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
          </CardBody>
        </Card>
      </div>
    </div>
  );
}
