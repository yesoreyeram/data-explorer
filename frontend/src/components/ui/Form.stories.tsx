import type { Meta, StoryObj } from "@storybook/react-vite";
import { Field } from "./Field";
import { Input } from "./Input";
import { Select } from "./Select";
import { Textarea } from "./Textarea";

const meta: Meta = {
  title: "Primitives/Form",
};
export default meta;
type Story = StoryObj;

export const InputWithField: Story = {
  render: () => (
    <div style={{ maxWidth: 340 }}>
      <Field htmlFor="conn-name" label="Connection name" hint="Human-readable identifier">
        <Input id="conn-name" placeholder="prod-analytics" />
      </Field>
      <Field htmlFor="conn-type" label="Type">
        <Select id="conn-type" defaultValue="postgres">
          <option value="postgres">PostgreSQL</option>
          <option value="mysql">MySQL</option>
          <option value="rest">REST</option>
        </Select>
      </Field>
      <Field htmlFor="conn-notes" label="Notes" hint="Optional. Markdown supported.">
        <Textarea id="conn-notes" placeholder="Owned by data-platform team…" />
      </Field>
    </div>
  ),
};

export const Focused: Story = {
  render: () => (
    <div style={{ maxWidth: 340 }}>
      <Field htmlFor="focus-demo" label="Focused input">
        <Input id="focus-demo" autoFocus placeholder="Click me to see the focus ring" />
      </Field>
    </div>
  ),
};
