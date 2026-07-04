import type { Meta, StoryObj } from "@storybook/react-vite";

const meta: Meta = {
  title: "Foundations/Design Tokens",
  parameters: { layout: "fullscreen" },
};
export default meta;

type Swatch = { name: string; token: string };

const SURFACE_TOKENS: Swatch[] = [
  { name: "Canvas", token: "--bg-canvas" },
  { name: "Surface", token: "--bg-surface" },
  { name: "Surface raised", token: "--bg-surface-raised" },
  { name: "Sunken", token: "--bg-sunken" },
  { name: "Hover", token: "--bg-hover" },
  { name: "Active", token: "--bg-active" },
];

const BORDER_TOKENS: Swatch[] = [
  { name: "Subtle", token: "--border-subtle" },
  { name: "Default", token: "--border" },
  { name: "Strong", token: "--border-strong" },
];

const TEXT_TOKENS: Swatch[] = [
  { name: "Primary", token: "--text-primary" },
  { name: "Secondary", token: "--text-secondary" },
  { name: "Tertiary", token: "--text-tertiary" },
];

const ACCENT_TOKENS: Swatch[] = [
  { name: "Accent", token: "--accent" },
  { name: "Accent strong", token: "--accent-strong" },
  { name: "Accent contrast", token: "--accent-contrast" },
  { name: "Accent soft", token: "--accent-soft" },
];

const STATUS_TOKENS: Swatch[] = [
  { name: "Success", token: "--success" },
  { name: "Success soft", token: "--success-soft" },
  { name: "Warning", token: "--warning" },
  { name: "Warning soft", token: "--warning-soft" },
  { name: "Danger", token: "--danger" },
  { name: "Danger soft", token: "--danger-soft" },
  { name: "Info", token: "--info" },
  { name: "Info soft", token: "--info-soft" },
];

function Swatches({ title, tokens }: { title: string; tokens: Swatch[] }) {
  return (
    <section style={{ marginBottom: 32 }}>
      <div className="section-label">{title}</div>
      <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(180px, 1fr))", gap: 12 }}>
        {tokens.map((t) => (
          <div
            key={t.token}
            style={{
              border: "1px solid var(--border)",
              borderRadius: "var(--radius-lg)",
              background: "var(--bg-surface)",
              overflow: "hidden",
            }}
          >
            <div style={{ height: 60, background: `var(${t.token})` }} />
            <div style={{ padding: "8px 12px" }}>
              <div style={{ fontWeight: 600, fontSize: 12 }}>{t.name}</div>
              <div className="mono" style={{ fontSize: 11, color: "var(--text-tertiary)" }}>{t.token}</div>
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}

const TYPE_SCALE = [
  { name: "3xl / hero", size: "--font-size-3xl" },
  { name: "2xl / stat value", size: "--font-size-2xl" },
  { name: "xl / page title", size: "--font-size-xl" },
  { name: "lg / section title", size: "--font-size-lg" },
  { name: "md / body", size: "--font-size-md" },
  { name: "sm / caption", size: "--font-size-sm" },
  { name: "xs / eyebrow", size: "--font-size-xs" },
];

const RADII = ["--radius-sm", "--radius-md", "--radius-lg", "--radius-xl", "--radius-pill"];
const SPACES = ["--space-1", "--space-2", "--space-3", "--space-4", "--space-5", "--space-6", "--space-7", "--space-8"];
const SHADOWS = ["--shadow-sm", "--shadow-md", "--shadow-lg"];

export const Colors: StoryObj = {
  render: () => (
    <div style={{ padding: 24 }}>
      <h2 style={{ marginTop: 0 }}>Color tokens</h2>
      <p style={{ color: "var(--text-secondary)", marginTop: 0 }}>
        Every color in the app resolves from one of these tokens. Toggle the
        Storybook theme toolbar to see the light/dark values side by side.
      </p>
      <Swatches title="Surfaces" tokens={SURFACE_TOKENS} />
      <Swatches title="Borders" tokens={BORDER_TOKENS} />
      <Swatches title="Text" tokens={TEXT_TOKENS} />
      <Swatches title="Accent" tokens={ACCENT_TOKENS} />
      <Swatches title="Status hues" tokens={STATUS_TOKENS} />
    </div>
  ),
};

export const Typography: StoryObj = {
  render: () => (
    <div style={{ padding: 24 }}>
      <h2 style={{ marginTop: 0 }}>Typography scale</h2>
      <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
        {TYPE_SCALE.map((t) => (
          <div key={t.size} style={{ display: "flex", alignItems: "baseline", gap: 24 }}>
            <span className="mono" style={{ minWidth: 200, fontSize: 11, color: "var(--text-tertiary)" }}>
              {t.size}
            </span>
            <span style={{ fontSize: `var(${t.size})` }}>{t.name}</span>
          </div>
        ))}
      </div>
    </div>
  ),
};

export const Radii: StoryObj = {
  render: () => (
    <div style={{ padding: 24, display: "flex", gap: 24, flexWrap: "wrap" }}>
      {RADII.map((r) => (
        <div key={r} style={{ textAlign: "center" }}>
          <div
            style={{
              width: 72,
              height: 72,
              background: "var(--accent-soft)",
              border: "1px solid var(--border-strong)",
              borderRadius: `var(${r})`,
              marginBottom: 8,
            }}
          />
          <div className="mono" style={{ fontSize: 11, color: "var(--text-tertiary)" }}>{r}</div>
        </div>
      ))}
    </div>
  ),
};

export const Spacing: StoryObj = {
  render: () => (
    <div style={{ padding: 24, display: "flex", flexDirection: "column", gap: 12 }}>
      {SPACES.map((s) => (
        <div key={s} style={{ display: "flex", alignItems: "center", gap: 16 }}>
          <span className="mono" style={{ minWidth: 120, fontSize: 11, color: "var(--text-tertiary)" }}>{s}</span>
          <span style={{ display: "inline-block", height: 12, width: `var(${s})`, background: "var(--accent)" }} />
        </div>
      ))}
    </div>
  ),
};

export const Shadows: StoryObj = {
  render: () => (
    <div style={{ padding: 24, display: "flex", gap: 32, flexWrap: "wrap" }}>
      {SHADOWS.map((s) => (
        <div key={s} style={{ textAlign: "center" }}>
          <div
            style={{
              width: 140,
              height: 90,
              background: "var(--bg-surface)",
              borderRadius: "var(--radius-lg)",
              boxShadow: `var(${s})`,
              marginBottom: 8,
            }}
          />
          <div className="mono" style={{ fontSize: 11, color: "var(--text-tertiary)" }}>{s}</div>
        </div>
      ))}
    </div>
  ),
};
