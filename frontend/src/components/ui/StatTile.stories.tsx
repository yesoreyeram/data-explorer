import type { Meta, StoryObj } from "@storybook/react-vite";
import { StatTile } from "./StatTile";
import { IconDatabase, IconWorkflow, IconShield } from "../icons";

const meta: Meta<typeof StatTile> = {
  title: "Primitives/StatTile",
  component: StatTile,
  args: { label: "Total runs", value: "12,847" },
};
export default meta;
type Story = StoryObj<typeof StatTile>;

export const Minimal: Story = {};

export const WithIcon: Story = {
  args: { icon: <IconDatabase width={14} height={14} /> },
};

export const WithDeltaUp: Story = {
  args: {
    icon: <IconDatabase width={14} height={14} />,
    delta: { direction: "up", value: "+12%", description: "from last week" },
  },
};

export const WithDeltaDown: Story = {
  args: {
    label: "AI tokens",
    value: "842K",
    icon: <IconShield width={14} height={14} />,
    delta: { direction: "down", value: "−2%", description: "from last week" },
  },
};

export const Grid: Story = {
  render: () => (
    <div className="stat-grid" style={{ maxWidth: 900 }}>
      <StatTile
        label="Total runs"
        value="12,847"
        icon={<IconWorkflow width={14} height={14} />}
        delta={{ direction: "up", value: "+12%", description: "from last week" }}
      />
      <StatTile
        label="Success rate"
        value="98.2%"
        icon={<IconShield width={14} height={14} />}
        delta={{ direction: "up", value: "+0.4%", description: "from last week" }}
      />
      <StatTile
        label="Active workflows"
        value="23"
        icon={<IconWorkflow width={14} height={14} />}
        delta={{ direction: "flat", value: "0", description: "no change" }}
      />
      <StatTile
        label="AI tokens"
        value="842K"
        icon={<IconDatabase width={14} height={14} />}
        delta={{ direction: "down", value: "−2%", description: "from last week" }}
      />
    </div>
  ),
};
