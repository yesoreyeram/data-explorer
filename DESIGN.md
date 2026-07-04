# Design system

This document is the reference for Data Explorer's design system: the
philosophy behind the visual language, the token scale, the primitive
components, and the conventions for extending them. It complements the
[Developer Guide](docs/DEVELOPER_GUIDE.md) (which covers *when* and *where*
to use the design system) and Storybook (which is the live, interactive
catalog — `npm run storybook` from `frontend/`).

- **Token source of truth:** [`frontend/src/styles/tokens.css`](frontend/src/styles/tokens.css)
- **CSS component layer:** [`frontend/src/styles/app.css`](frontend/src/styles/app.css)
- **Typed React primitives:** [`frontend/src/components/ui/`](frontend/src/components/ui/)
- **Live stories:** `npm run storybook` (dev) or `npm run build-storybook` (static)

## Philosophy

Data Explorer is a dense, information-first product — the whole point is
to browse tables, run queries, and wire up workflows without the tool
getting in the way. Every design decision below flows from three rules.

1. **Near-monochrome by default.** Structural color (surfaces, borders,
   text) is pure grayscale. The accent itself is *ink* — near-black on
   light, near-white on dark — so a screen never reads as "colorful". The
   four status hues (`success`, `warning`, `danger`, `info`) are the only
   survivors, kept desaturated and confined to a 6-px dot on badges or a
   trend delta on stat tiles rather than a filled background. This keeps
   the eye on the data instead of on the chrome.
2. **Compact but not cramped.** The default body size is 12.5 px, the
   default row height is ~26 px, and gutters land on a 4-based scale. This
   is deliberately tighter than a marketing site and looser than an IDE.
3. **One source of truth.** Every color, radius, spacing, shadow, and
   motion value is a CSS custom property in `tokens.css`. A hard-coded hex
   or px in a `.tsx`/`.css` file is a bug. Themes swap by toggling
   `data-theme` on `<html>`; no per-component branching.

## Design tokens

All tokens live in [`frontend/src/styles/tokens.css`](frontend/src/styles/tokens.css)
and are documented visually in Storybook under **Foundations → Design
Tokens**. The categories below are exhaustive — nothing else is a token,
and nothing else should be.

### Typography

| Token                     | Value  | Typical use                       |
| ------------------------- | ------ | --------------------------------- |
| `--font-size-xs`          | 10.5px | Eyebrow labels, dot-badge text    |
| `--font-size-sm`          | 11.5px | Form labels, nav links, captions  |
| `--font-size-md`          | 12.5px | Default body / card titles        |
| `--font-size-lg`          | 14px   | Section headings inside a page    |
| `--font-size-xl`          | 16px   | Page titles                       |
| `--font-size-2xl`         | 21px   | Stat tile values                  |
| `--font-size-3xl`         | 26px   | Hero numbers (auth, welcome)      |
| `--font-weight-regular`   | 400    | Body copy                         |
| `--font-weight-medium`    | 500    | Nav links, buttons                |
| `--font-weight-semibold`  | 600    | Labels, headings, badges          |
| `--font-weight-bold`      | 650    | Panel titles, stat values         |
| `--letter-spacing-tight`  | -0.02em| Big numbers, page titles          |
| `--letter-spacing-eyebrow`| 0.07em | Uppercase section labels          |

### Spacing (4-based scale)

`--space-0` (0) · `--space-1` (4) · `--space-2` (8) · `--space-3` (12) ·
`--space-4` (16) · `--space-5` (20) · `--space-6` (28) · `--space-7` (40) ·
`--space-8` (56).

### Radii

`--radius-sm` (4) · `--radius-md` (6) · `--radius-lg` (8) · `--radius-xl`
(12, for outer shell / hero cards) · `--radius-pill` (999).

### Motion

`--duration-fast` (90ms) · `--duration-base` (150ms) · `--duration-slow`
(240ms) — paired with `--ease-standard` and `--ease-emphasized`.
`prefers-reduced-motion: reduce` collapses every transition/animation to
1ms globally (see `index.css`).

### Semantic color tokens (per theme)

Defined once for `light`, once for `dark`, and once for `system` (via
`prefers-color-scheme`). Every component reads only these — never a raw
hex.

| Token                                              | Role                                       |
| -------------------------------------------------- | ------------------------------------------ |
| `--bg-canvas` / `--bg-surface` / `--bg-surface-raised` | Page background · card · elevated card |
| `--bg-sunken` / `--bg-hover` / `--bg-active`       | Table header · hover state · pressed       |
| `--border-subtle` / `--border` / `--border-strong` | Hairline · default · high-contrast         |
| `--text-primary` / `--text-secondary` / `--text-tertiary` / `--text-inverse` | Content hierarchy                        |
| `--accent` / `--accent-strong` / `--accent-contrast` / `--accent-soft` | Primary button, focus ring source, subtle fill |
| `--focus-ring`                                     | Focus outline color (paired with `--focus-ring-width`) |
| `--shadow-sm` / `--shadow-md` / `--shadow-lg`      | Cards · popovers · modals                  |

### Status hues (theme-independent)

`--success`, `--warning`, `--danger`, `--info` and their `-soft` (14 %
opacity) variants. **Never** used as a filled background outside `Badge
variant="soft"` and the `StatTile` trend delta.

### Layout

`--sidebar-width` (208) · `--sidebar-width-collapsed` (52) · `--topbar-height` (44).

## Primitive components

Every primitive lives in `frontend/src/components/ui/` with a matching
`*.stories.tsx` file. Import from the barrel:

```ts
import { Button, Card, StatTile, PageHeader } from "../components/ui";
```

| Primitive       | Purpose                                                       | Key props                                        |
| --------------- | ------------------------------------------------------------- | ------------------------------------------------ |
| `Button`        | Standard action button                                        | `variant` (default · primary · danger · ghost), `size` (md · sm) |
| `IconButton`    | Icon-only button with mandatory `aria-label`                  | `label` (required)                               |
| `Field`         | `<label>` + control + hint wrapper                            | `htmlFor`, `label`, `hint`                       |
| `Input` / `Select` / `Textarea` | Themed form controls                          | Native props                                     |
| `Badge`         | Monochrome status chip                                        | `tone` (neutral · success · warning · danger · info), `variant` (dot · soft) |
| `Card` / `CardHeader` / `CardBody` / `CardTitle` | Content surface w/ optional title bar     | –                                                |
| `StatTile`      | KPI tile with optional icon and trend delta                   | `label`, `value`, `icon?`, `delta?`              |
| `Kbd`           | Keyboard-shortcut chip (renders semantic `<kbd>`)             | –                                                |
| `Divider`       | Horizontal or vertical rule (`<hr>` / `role="separator"`)     | `orientation`                                    |
| `PageHeader`    | Consistent page-level title + subtitle + actions              | `title`, `subtitle?`, `actions?`                 |
| `SectionLabel`  | Uppercase eyebrow label above content groups                  | –                                                |
| `EmptyState`    | Icon + title + description + CTA for zero-data views          | `icon?`, `title`, `description?`, `action?`      |

Consult Storybook for props, variants, and copy-pasteable snippets.

## Accessibility

- **Focus.** A single `:focus-visible` ring in `index.css` applies to every
  focusable element (native or third-party). Ring color and width are
  tokens (`--focus-ring`, `--focus-ring-width`) so a11y tuning is one edit.
- **Icon-only controls.** `IconButton` requires a `label` prop that becomes
  both `aria-label` and `title`. Do not render a bare `<button>` around an
  icon.
- **Color independence.** Semantic tone is always paired with a shape
  (dot, arrow) and/or text — never color alone. Verify with the "Colors"
  Storybook story in both themes.
- **Reduced motion.** All transitions honor `prefers-reduced-motion: reduce`
  through a single global rule; no per-component work is required.
- **Screen-reader utility.** Use the `.sr-only` class for text that must
  be announced but not shown (e.g., "trending up" alongside the delta
  arrow in `StatTile`).

## Extending the system

1. **Prefer composition over new primitives.** A new page-level UI should
   be reachable as a combination of existing primitives + a small local
   layout. Only promote to `ui/` when the same pattern shows up in ≥ 2
   pages.
2. **Add a story.** Every primitive in `ui/` has a `*.stories.tsx`
   sibling. Adding a new one without stories will block review.
3. **No new tokens without a use case.** Adding to `tokens.css` needs a
   concrete first caller in the same PR. Speculative tokens rot.
4. **Never a new hue.** If you truly need a fifth semantic color, discuss
   it in the PR before adding. The near-monochrome constraint is the
   product identity.
5. **Never a hard-coded value.** `hsl(...)`, `#…`, or a raw px in a
   `.tsx` or `.css` file outside `tokens.css` is a review blocker. Extract
   to a token.

## Design references

The current visual pass draws inspiration from:

- **FlowAI dashboard** (attached PR reference image) — for the stat-tile
  pattern (icon + label + big value + delta footer), the soft-pill
  "Active" badges in table rows, and the outer 12-px rounded shell.
- **[clapet.app/d](https://clapet.app/d)** — for the compact, monochrome
  workflow customization experience, keyboard-first affordances (`⌘K`),
  and grouped sidebar sections with eyebrow labels.

Both were used as *pattern* references — the palette, weight, and density
are Data Explorer's own.
