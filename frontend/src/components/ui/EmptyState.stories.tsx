import type { Meta, StoryObj } from "@storybook/react-vite";
import { EmptyState } from "./EmptyState";
import { Button } from "./Button";
import { IconDatabase } from "../icons";

const meta: Meta<typeof EmptyState> = {
  title: "Primitives/EmptyState",
  component: EmptyState,
};
export default meta;
type Story = StoryObj<typeof EmptyState>;

export const Minimal: Story = {
  args: { title: "No connections yet" },
};

export const WithDescription: Story = {
  args: {
    icon: <IconDatabase width={16} height={16} />,
    title: "No connections yet",
    description: "Add a data source to start querying and building workflows.",
  },
};

export const WithAction: Story = {
  args: {
    icon: <IconDatabase width={16} height={16} />,
    title: "No connections yet",
    description: "Add a data source to start querying and building workflows.",
    action: <Button variant="primary" size="sm">+ New connection</Button>,
  },
};
