import type { Meta, StoryObj } from "@storybook/react-vite";
import { SectionLabel } from "./SectionLabel";
import { Divider } from "./Divider";
import { IconButton } from "./IconButton";
import { IconSun } from "../icons";

const sectionMeta: Meta<typeof SectionLabel> = {
  title: "Primitives/SectionLabel",
  component: SectionLabel,
  args: { children: "Workspaces" },
};
export default sectionMeta;
type SectionStory = StoryObj<typeof SectionLabel>;

export const Default: SectionStory = {};

export const IconButtonStory: StoryObj = {
  name: "IconButton (with visible label for a11y)",
  render: () => (
    <div style={{ display: "flex", gap: 8 }}>
      <IconButton label="Toggle theme"><IconSun width={14} height={14} /></IconButton>
    </div>
  ),
};

export const DividerHorizontal: StoryObj = {
  name: "Divider — horizontal",
  render: () => (
    <div style={{ maxWidth: 320 }}>
      <div>Above</div>
      <Divider />
      <div>Below</div>
    </div>
  ),
};

export const DividerVertical: StoryObj = {
  name: "Divider — vertical",
  render: () => (
    <div style={{ display: "inline-flex", alignItems: "center", height: 40 }}>
      <span>Left</span>
      <Divider orientation="vertical" />
      <span>Right</span>
    </div>
  ),
};
