import type { Meta, StoryObj } from "@storybook/react-vite";
import { Card, CardBody, CardHeader, CardTitle } from "./Card";
import { Button } from "./Button";

const meta: Meta<typeof Card> = {
  title: "Primitives/Card",
  component: Card,
};
export default meta;
type Story = StoryObj<typeof Card>;

export const Basic: Story = {
  render: () => (
    <Card style={{ maxWidth: 420 }}>
      <CardHeader>
        <CardTitle>Recent connections</CardTitle>
        <Button variant="ghost" size="sm">View all</Button>
      </CardHeader>
      <CardBody>
        <div style={{ display: "flex", justifyContent: "space-between" }}>
          <span>prod-analytics</span>
          <span style={{ color: "var(--text-tertiary)" }}>postgres</span>
        </div>
      </CardBody>
    </Card>
  ),
};

export const BodyOnly: Story = {
  render: () => (
    <Card style={{ maxWidth: 420 }}>
      <CardBody>
        A card with no header — useful for embedding rich content that owns
        its own top-level UI.
      </CardBody>
    </Card>
  ),
};
