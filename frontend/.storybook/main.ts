import type { StorybookConfig } from "@storybook/react-vite";

const config: StorybookConfig = {
  stories: ["../src/**/*.mdx", "../src/**/*.stories.@(js|jsx|ts|tsx)"],
  framework: {
    name: "@storybook/react-vite",
    options: {},
  },
  addons: [],
  core: {
    disableTelemetry: true,
    disableWhatsNewNotifications: true,
  },
  typescript: {
    check: false,
  },
};

export default config;
