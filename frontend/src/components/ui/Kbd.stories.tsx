import type { Meta, StoryObj } from "@storybook/react-vite";
import { Kbd } from "./Kbd";

const meta: Meta<typeof Kbd> = {
  title: "Primitives/Kbd",
  component: Kbd,
  args: { children: "K" },
};
export default meta;
type Story = StoryObj<typeof Kbd>;

export const Single: Story = {};

export const Shortcut: Story = {
  render: () => (
    <div style={{ display: "inline-flex", gap: 4 }}>
      <Kbd>⌘</Kbd>
      <Kbd>K</Kbd>
    </div>
  ),
};

export const InlineWithText: Story = {
  render: () => (
    <p style={{ color: "var(--text-secondary)" }}>
      Press <Kbd>⌘</Kbd><Kbd>K</Kbd> to open the command palette, or <Kbd>Esc</Kbd> to dismiss.
    </p>
  ),
};
