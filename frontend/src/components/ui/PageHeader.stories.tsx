import type { Meta, StoryObj } from "@storybook/react-vite";
import { PageHeader } from "./PageHeader";
import { Button } from "./Button";

const meta: Meta<typeof PageHeader> = {
  title: "Primitives/PageHeader",
  component: PageHeader,
};
export default meta;
type Story = StoryObj<typeof PageHeader>;

export const TitleOnly: Story = {
  args: { title: "Connections" },
};

export const TitleAndSubtitle: Story = {
  args: {
    title: "Welcome back, Jane",
    subtitle: "Here's what's happening across your workspace.",
  },
};

export const WithActions: Story = {
  args: {
    title: "Workflows",
    subtitle: "Automate exports and transforms across your data sources.",
    actions: (
      <>
        <Button size="sm">Import</Button>
        <Button size="sm" variant="primary">+ New workflow</Button>
      </>
    ),
  },
};
