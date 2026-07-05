# src/components/ui

## What this package does

`src/components/ui` is the **first-party design-system primitive library**. Every interactive or display element in the application should be built from these components rather than from raw HTML elements or ad-hoc Tailwind/CSS class strings. They are typed wrappers around the class names defined in `src/styles/app.css`, so a raw `className="btn"` and a `<Button>` render identically — partially-migrated and fully-migrated screens never look inconsistent.

## Components

| Component | Props highlights | Description |
|---|---|---|
| `Button` | `variant` (`primary`\|`secondary`\|`ghost`\|`danger`), `size` (`sm`\|`md`\|`lg`), `loading`, `disabled` | Standard button with variant styles and loading spinner |
| `IconButton` | `icon`, `label` (accessible), `size` | Square button with a centered icon; `label` sets `aria-label` |
| `Field` | `label`, `required`, `error`, `hint` | Form field wrapper: label + input slot + error/hint text |
| `Input` | All native `<input>` props + `error` | Styled text input; `error` applies the error ring |
| `Select` | All native `<select>` props + `error` | Styled select dropdown |
| `Textarea` | All native `<textarea>` props + `error` | Styled multi-line text input |
| `Badge` | `status` (`success`\|`warning`\|`danger`\|`info`\|`neutral`) | Status dot + label; status hues confined to a 6px dot only |
| `Card` | — | Surface container |
| `CardHeader` | `title`, `actions` slot | Card header with optional actions slot |
| `CardBody` | — | Card body with standard padding |
| `StatTile` | `label`, `value`, `trend`, `trendDelta` | Metric tile for the dashboard |

## Design token conventions

- **Never** use hard-coded hex colors or pixel values — always `var(--token-name)`.
- All tokens are defined in `src/styles/tokens.css`.
- Accent is near-black on light / near-white on dark; `success`/`warning`/`danger`/`info` are the **only** hues — used only in `Badge` status dots and `StatTile` trend deltas.
- Themes swap by toggling `data-theme` on `<html>` — no per-component branching needed.

## Accessibility conventions

- `Button` and `IconButton` always have accessible text: `children` for `Button`, `label` (→ `aria-label`) for `IconButton`.
- `Field` associates its `<label>` with the input via `htmlFor`/`id`.
- `Badge` uses `role="status"` where appropriate.
- Focus rings use the `--focus-ring` token — visible in all themes without hard-coded colors.

## Storybook

Each component has stories in `frontend/.storybook/`. Run `npm run storybook` to browse them interactively.

```bash
cd frontend
npm run storybook   # dev server
npm run build-storybook  # static build
```

## Architecture decisions (ADRs)

| Decision | Rationale |
|---|---|
| Wrappers over `app.css` class names | Raw class names and components render identically; incremental migration is safe |
| Design tokens as CSS custom properties | Themes swap with one attribute; zero per-component branching |
| Near-monochrome palette | Dense information display; status hues stand out precisely because they are the only color |
| `status` prop on `Badge` (not `color`) | Semantic API; callers express intent (`success`) not appearance (`green`) |

## Scope and responsibilities

- Define the visual language for every interactive and display element.
- Export typed React components for every primitive.
- Own the design token conventions (documented in `DESIGN.md`).
- Provide Storybook stories for every component.

## Limitations and todos

- [ ] No `Table` primitive in `ui/`; data tables use `components/DataTable.tsx` directly.
- [ ] No `Modal` primitive; `components/Modal.tsx` wraps Radix/HeadlessUI but is not in `ui/`.
- [ ] `StatTile` `trend` rendering is rudimentary; no sparkline chart.
- [ ] No animation tokens; transitions are hard-coded in CSS.
- [ ] Storybook coverage does not include all component variants and states.
