import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";

import { getGuardrailStats } from "../api/admin";
import { listConnections } from "../api/connections";
import { listWorkflows } from "../api/workflows";
import { useAuthStore } from "../state/authStore";
import { useSavedChartsStore } from "../state/savedChartsStore";
import { PERMISSIONS } from "../lib/permissions";
import { StatusBadge } from "../components/StatusBadge";
import { FrameChart } from "../components/charts/FrameChart";
import { IconDatabase, IconShield, IconTrash, IconWorkflow } from "../components/icons";
import { Card, CardBody, CardHeader, CardTitle, EmptyState, IconButton, PageHeader, StatTile, type StatTileDelta } from "../components/ui";

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
  const roles = useAuthStore((s) => s.roles);
  const savedCharts = useSavedChartsStore((s) => s.items);
  const removeChart = useSavedChartsStore((s) => s.removeChart);

  const connectionsQuery = useQuery({ queryKey: ["connections"], queryFn: listConnections, enabled: hasPermission(PERMISSIONS.connectionsRead) });
  const workflowsQuery = useQuery({ queryKey: ["workflows"], queryFn: listWorkflows, enabled: hasPermission(PERMISSIONS.workflowsRead) });
  const guardrailsQuery = useQuery({ queryKey: ["guardrails"], queryFn: getGuardrailStats, enabled: roles.includes("admin") });

  const connections = connectionsQuery.data ?? [];
  const workflows = workflowsQuery.data ?? [];
  const healthyConnections = connections.filter((c) => c.status === "healthy").length;
  const publishedWorkflows = workflows.filter((w) => w.status === "published").length;
  const guardrailTrips = guardrailsQuery.data?.items.reduce((sum, item) => sum + item.count, 0) ?? 0;

  const prev = { connections: Math.max(0, connections.length - 1), healthy: Math.max(0, healthyConnections - 1), workflows: Math.max(0, workflows.length - 1), published: publishedWorkflows };

  return (
    <div>
      <PageHeader title={`Welcome back${user ? `, ${user.displayName.split(" ")[0]}` : ""}`} subtitle="Here's what's happening across your workspace." />

      <div className="stat-grid">
        <StatTile label="Connections" value={connections.length} icon={<IconDatabase width={14} height={14} />} delta={delta(connections.length, prev.connections)} />
        <StatTile label="Healthy connections" value={healthyConnections} icon={<IconShield width={14} height={14} />} delta={delta(healthyConnections, prev.healthy)} />
        <StatTile label="Workflows" value={workflows.length} icon={<IconWorkflow width={14} height={14} />} delta={delta(workflows.length, prev.workflows)} />
        <StatTile label="Published workflows" value={publishedWorkflows} icon={<IconWorkflow width={14} height={14} />} delta={delta(publishedWorkflows, prev.published)} />
        {roles.includes("admin") && (
          <StatTile label="Guardrail trips (24h)" value={guardrailTrips} icon={<IconShield width={14} height={14} />} delta={guardrailsQuery.data?.items[0] ? { direction: "flat", value: guardrailsQuery.data.items[0].limitType, description: "top limit type" } : undefined} />
        )}
      </div>

      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12, marginBottom: 12 }}>
        <Card>
          <CardHeader>
            <CardTitle>Recent connections</CardTitle>
            <Link to="/connections" className="btn btn-sm btn-ghost">View all</Link>
          </CardHeader>
          <CardBody>
            {connections.length === 0 ? (
              <EmptyState icon={<IconDatabase width={16} height={16} />} title="No connections yet" description="Add a data source to start querying and building workflows." />
            ) : (
              <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
                {connections.slice(0, 6).map((c) => (
                  <div key={c.id} style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
                    <span>{c.name} <span style={{ color: "var(--text-tertiary)" }}>({c.type})</span></span>
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
            <Link to="/workflows" className="btn btn-sm btn-ghost">View all</Link>
          </CardHeader>
          <CardBody>
            {workflows.length === 0 ? (
              <EmptyState icon={<IconWorkflow width={16} height={16} />} title="No workflows yet" description="Build a pipeline from your connections to automate exports and transforms." />
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

      <Card>
        <CardHeader>
          <CardTitle>Saved charts</CardTitle>
        </CardHeader>
        <CardBody>
          {savedCharts.length === 0 ? (
            <EmptyState icon={<IconWorkflow width={16} height={16} />} title="No saved charts yet" description="Save a chart from Explore or a workflow result to pin it to the dashboard." />
          ) : (
            <div className="saved-chart-grid">
              {savedCharts.map((item) => (
                <div key={item.id} className="saved-chart-card">
                  <div className="saved-chart-header">
                    <div>
                      <strong>{item.config.title}</strong>
                      <div className="field-hint">{item.sourceLabel}</div>
                    </div>
                    <IconButton label="Remove saved chart" onClick={() => removeChart(item.id)}>
                      <IconTrash width={13} height={13} />
                    </IconButton>
                  </div>
                  <FrameChart frame={item.frame} config={item.config} height={220} />
                </div>
              ))}
            </div>
          )}
        </CardBody>
      </Card>
    </div>
  );
}
