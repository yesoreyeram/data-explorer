import type { Preview } from "@storybook/react-vite";
import { useEffect } from "react";

// Import the full app stylesheet so every story renders in the same visual
// context as the real app — no divergence between "how it looks in
// Storybook" and "how it looks in the product".
import "../src/index.css";
import "../src/styles/app.css";

/** Toolbar-driven theme switcher — sets `data-theme` on <html> so every
 * design token resolves to the correct value. Same mechanism the app uses
 * in production (see `state/themeStore.ts`), just wired to a story arg. */
function useAppliedTheme(theme: string) {
  useEffect(() => {
    const previous = document.documentElement.getAttribute("data-theme");
    document.documentElement.setAttribute("data-theme", theme);
    return () => {
      if (previous) document.documentElement.setAttribute("data-theme", previous);
    };
  }, [theme]);
}

const preview: Preview = {
  parameters: {
    layout: "padded",
    controls: {
      matchers: {
        color: /(background|color)$/i,
        date: /Date$/i,
      },
    },
    backgrounds: { disable: true },
  },
  globalTypes: {
    theme: {
      description: "Design-system theme",
      defaultValue: "light",
      toolbar: {
        title: "Theme",
        icon: "circlehollow",
        items: [
          { value: "light", title: "Light" },
          { value: "dark", title: "Dark" },
        ],
        dynamicTitle: true,
      },
    },
  },
  decorators: [
    (Story, context) => {
      useAppliedTheme(String(context.globals.theme ?? "light"));
      return (
        <div
          style={{
            background: "var(--bg-canvas)",
            color: "var(--text-primary)",
            padding: "var(--space-4)",
            minHeight: "100vh",
          }}
        >
          <Story />
        </div>
      );
    },
  ],
};

export default preview;
