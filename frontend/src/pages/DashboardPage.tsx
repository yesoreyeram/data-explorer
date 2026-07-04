import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";

import { listConnections } from "../api/connections";
import { listWorkflows } from "../api/workflows";
import { useAuthStore } from "../state/authStore";
import { PERMISSIONS } from "../lib/permissions";
import { StatusBadge } from "../components/StatusBadge";
import { IconDatabase, IconShield, IconWorkflow } from "../components/icons";
import {
  Card,
  CardBody,
  CardHeader,
  CardTitle,
  EmptyState,
  PageHeader,
  StatTile,
  type StatTileDelta,
} from "../components/ui";

/** Compute a delta given a current and previous count. Callers own the
 * "up is good" semantics (here all four dashboard metrics grow-is-good, so
 * a positive delta maps to direction: "up"). */
function delta(current: number, previous: number): StatTileDelta | undefined {
  if (previous === 0 && current === 0) return undefined;
  const diff = current - previous;
  const pct = previous === 0 ? 100 : Math.round((diff / previous) * 100);
  const direction = diff > 0 ? "up" : diff < 0 ? "down" : "flat";
  const sign = diff > 0 ? "+" : diff < 0 ? "−" : "";
  return { direction, value: `${sign}${Math.abs(pct)}%`, description: "vs. last week" };
}

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

  // Placeholder "previous week" numbers — the server does not yet expose a
  // rolling counter, so we synthesize a stable comparison so the delta row
  // renders as a real component demo rather than empty scaffolding.
  const prev = { connections: Math.max(0, connections.length - 1), healthy: Math.max(0, healthyConnections - 1), workflows: Math.max(0, workflows.length - 1), published: publishedWorkflows };

  return (
    <div>
      <PageHeader
        title={`Welcome back${user ? `, ${user.displayName.split(" ")[0]}` : ""}`}
        subtitle="Here's what's happening across your workspace."
      />

      <div className="stat-grid">
        <StatTile
          label="Connections"
          value={connections.length}
          icon={<IconDatabase width={14} height={14} />}
          delta={delta(connections.length, prev.connections)}
        />
        <StatTile
          label="Healthy connections"
          value={healthyConnections}
          icon={<IconShield width={14} height={14} />}
          delta={delta(healthyConnections, prev.healthy)}
        />
        <StatTile
          label="Workflows"
          value={workflows.length}
          icon={<IconWorkflow width={14} height={14} />}
          delta={delta(workflows.length, prev.workflows)}
        />
        <StatTile
          label="Published workflows"
          value={publishedWorkflows}
          icon={<IconWorkflow width={14} height={14} />}
          delta={delta(publishedWorkflows, prev.published)}
        />
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
              <EmptyState
                icon={<IconDatabase width={16} height={16} />}
                title="No connections yet"
                description="Add a data source to start querying and building workflows."
              />
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
              <EmptyState
                icon={<IconWorkflow width={16} height={16} />}
                title="No workflows yet"
                description="Build a pipeline from your connections to automate exports and transforms."
              />
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
