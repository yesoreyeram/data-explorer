import type { Meta, StoryObj } from "@storybook/react-vite";
import { Badge } from "./Badge";

const meta: Meta<typeof Badge> = {
  title: "Primitives/Badge",
  component: Badge,
  args: { children: "Active" },
  argTypes: {
    tone: { control: "select", options: ["neutral", "success", "warning", "danger", "info"] },
    variant: { control: "select", options: ["dot", "soft"] },
  },
};
export default meta;
type Story = StoryObj<typeof Badge>;

export const Neutral: Story = { args: { tone: "neutral", children: "Draft" } };
export const Success: Story = { args: { tone: "success", children: "Healthy" } };
export const Warning: Story = { args: { tone: "warning", children: "Unverified" } };
export const Danger: Story = { args: { tone: "danger", children: "Unhealthy" } };
export const SoftSuccess: Story = { args: { tone: "success", variant: "soft", children: "Active" } };

export const AllTones: Story = {
  render: () => (
    <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
      <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
        <Badge tone="neutral">Draft</Badge>
        <Badge tone="success">Healthy</Badge>
        <Badge tone="warning">Unverified</Badge>
        <Badge tone="danger">Unhealthy</Badge>
        <Badge tone="info">Info</Badge>
      </div>
      <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
        <Badge tone="neutral" variant="soft">Draft</Badge>
        <Badge tone="success" variant="soft">Active</Badge>
        <Badge tone="warning" variant="soft">Running</Badge>
        <Badge tone="danger" variant="soft">Failed</Badge>
        <Badge tone="info" variant="soft">Beta</Badge>
      </div>
    </div>
  ),
};
