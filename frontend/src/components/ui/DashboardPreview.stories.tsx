import type { Meta, StoryObj } from "@storybook/react-vite";
import { Button } from "./Button";
import { Card, CardBody, CardHeader, CardTitle } from "./Card";
import { StatTile } from "./StatTile";
import { PageHeader } from "./PageHeader";
import { Badge } from "./Badge";
import { EmptyState } from "./EmptyState";
import { IconDatabase, IconWorkflow, IconShield } from "../icons";

const meta: Meta = {
  title: "Pages/Dashboard preview",
  parameters: { layout: "fullscreen" },
};
export default meta;

const CONNECTIONS = [
  { id: "1", name: "prod-analytics", type: "postgres", status: "healthy" },
  { id: "2", name: "customers-mysql", type: "mysql", status: "healthy" },
  { id: "3", name: "billing-api", type: "rest", status: "unverified" },
  { id: "4", name: "warehouse-bq", type: "bigquery", status: "healthy" },
  { id: "5", name: "cloudwatch-logs", type: "cloudwatch", status: "unhealthy" },
] as const;

const WORKFLOWS = [
  { id: "1", name: "Daily revenue export", status: "published" },
  { id: "2", name: "Customer 360 sync", status: "published" },
  { id: "3", name: "Support ticket digest", status: "draft" },
  { id: "4", name: "Weekly cohort refresh", status: "published" },
] as const;

const TONE: Record<string, "success" | "warning" | "danger" | "neutral"> = {
  healthy: "success", published: "success", unverified: "warning", unhealthy: "danger", draft: "neutral",
};

export const Overview: StoryObj = {
  render: () => (
    <div style={{ padding: 20, maxWidth: 1120 }}>
      <PageHeader
        title="Welcome back, Jane"
        subtitle="Here's what's happening across your workspace."
        actions={
          <>
            <Button size="sm">Import</Button>
            <Button size="sm" variant="primary">+ New workflow</Button>
          </>
        }
      />

      <div className="stat-grid">
        <StatTile
          label="Connections"
          value={12}
          icon={<IconDatabase width={14} height={14} />}
          delta={{ direction: "up", value: "+12%", description: "vs. last week" }}
        />
        <StatTile
          label="Healthy connections"
          value={11}
          icon={<IconShield width={14} height={14} />}
          delta={{ direction: "up", value: "+0.4%", description: "vs. last week" }}
        />
        <StatTile
          label="Workflows"
          value={23}
          icon={<IconWorkflow width={14} height={14} />}
          delta={{ direction: "flat", value: "0", description: "no change" }}
        />
        <StatTile
          label="Published workflows"
          value={19}
          icon={<IconWorkflow width={14} height={14} />}
          delta={{ direction: "down", value: "−2%", description: "vs. last week" }}
        />
      </div>

      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
        <Card>
          <CardHeader>
            <CardTitle>Recent connections</CardTitle>
            <Button variant="ghost" size="sm">View all</Button>
          </CardHeader>
          <CardBody>
            <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
              {CONNECTIONS.map((c) => (
                <div key={c.id} style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
                  <span>
                    {c.name} <span style={{ color: "var(--text-tertiary)" }}>({c.type})</span>
                  </span>
                  <Badge tone={TONE[c.status]}>{c.status}</Badge>
                </div>
              ))}
            </div>
          </CardBody>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Recent workflows</CardTitle>
            <Button variant="ghost" size="sm">View all</Button>
          </CardHeader>
          <CardBody>
            <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
              {WORKFLOWS.map((w) => (
                <div key={w.id} style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
                  <span>{w.name}</span>
                  <Badge tone={TONE[w.status]} variant="soft">{w.status}</Badge>
                </div>
              ))}
            </div>
          </CardBody>
        </Card>
      </div>
    </div>
  ),
};

export const EmptyStateExample: StoryObj = {
  name: "Empty state",
  render: () => (
    <div style={{ padding: 20, maxWidth: 600 }}>
      <Card>
        <CardHeader>
          <CardTitle>Recent connections</CardTitle>
        </CardHeader>
        <CardBody>
          <EmptyState
            icon={<IconDatabase width={16} height={16} />}
            title="No connections yet"
            description="Add a data source to start querying and building workflows."
            action={<Button variant="primary" size="sm">+ New connection</Button>}
          />
        </CardBody>
      </Card>
    </div>
  ),
};
