# FR-12 — User Interface, Theming & Accessibility

## Overview

Data Explorer's frontend is a **compact, near-monochrome, dense-but-legible
web application** written in React 18 + TypeScript, built with Vite, and
deployed as static assets served alongside the Go backend. Every visible
surface — auth flow, connections, catalog, explore, workflows, audit —
composes a small set of primitives from `src/components/ui/` and design
tokens from `src/index.css`. This FRD captures the **product-visible
UI/UX requirements** that cross every feature: theming, layout,
navigation, accessibility, keyboard handling, empty/error states, and
the design system's boundaries. Individual feature FRDs point back at
this document for the shared UI contract.

The visual identity is deliberately restrained: **status colors — and
only status colors — are the sole hues**, reserved for green/yellow/red
status dots. The accent color is near-black on light theme and
near-white on dark theme. This lets the *data* stand out, not the
chrome.

## Product goals

- Present a **consistent visual language** across every page so users
  don't have to relearn the product when moving between Connections,
  Explore, and Workflows.
- Support **light and dark themes** with a persistent user preference
  synced to the browser's `prefers-color-scheme`.
- Meet **WCAG 2.1 AA** contrast ratios throughout by design-token
  constraint, not by ad-hoc adjustment.
- Provide **full keyboard reachability** for every interactive
  element — the product should be usable without a mouse.
- Compose complex screens from a **small library of primitives**
  (`Button`, `Field`, `Input`, `Select`, `Card`, `Badge`, `StatTile`,
  `Modal`, `DataFrameView`) so consistency is enforced structurally.
- Handle **empty, loading, and error states first-class** — every list
  and every form has a defined non-happy-path presentation.

## User personas

| Persona                     | Description                                                                                                |
| --------------------------- | ---------------------------------------------------------------------------------------------------------- |
| **All users**               | Everyone touching the UI benefits from the shared conventions and accessibility features.                 |
| **Keyboard-first user**     | Uses tab / shift+tab / arrow keys / space / enter; must reach and operate every action.                    |
| **Screen-reader user**      | Uses NVDA / JAWS / VoiceOver; needs labelled controls, region landmarks, and status announcements.         |
| **Low-vision user**         | Relies on high contrast, sufficient sizing, and non-color-only signalling.                                 |
| **Color-blind user**        | Cannot distinguish red from green reliably; needs shape / label reinforcement of status.                   |
| **Non-native English user** | UI copy is in English but must be legible, unambiguous, and free of idiom.                                 |
| **Enterprise user**         | Runs the platform inside a corporate laptop with strict CSP; must not violate its content-security policy.  |

## User stories

- **US-12.1** As a user, I want to toggle between light and dark
  theme, so I can match my ambient environment.
- **US-12.2** As a returning user, I want my theme preference
  remembered across sessions, so I don't re-toggle every login.
- **US-12.3** As a keyboard-first user, I want every button, link,
  and form control reachable by Tab in a logical order, so I can
  navigate without a mouse.
- **US-12.4** As a screen-reader user, I want form fields to have
  associated labels and errors announced when they appear, so I
  understand what a form is asking for.
- **US-12.5** As a low-vision user, I want text to have sufficient
  contrast in both themes, so I can read it comfortably.
- **US-12.6** As a color-blind user, I want status to be signalled by
  shape / text and not only by color, so a red dot alone is not the
  only signal.
- **US-12.7** As any user, I want empty states with helpful next
  steps, so I never see a blank screen without a hint.
- **US-12.8** As any user, I want errors to be clearly presented near
  the failing action, with a remediation string, so I know what to
  try next.
- **US-12.9** As an admin, I want the navigation menu to only show
  pages I have access to, so I don't click into a dead-end.
- **US-12.10** As any user, I want the application to load quickly
  and remain responsive on a mid-range laptop, so the tool feels
  crisp.

## Functional requirements

### FR-12.1 — Design tokens

All colors, spacing, font sizes, and radii SHALL be defined as CSS
custom properties in `src/index.css`. Components SHALL reference
tokens (e.g. `var(--surface-0)`), never hard-coded hex values.

Required token families:

- **Surfaces**: `--surface-0` (page), `--surface-1` (card),
  `--surface-2` (elevated / hover).
- **Text**: `--text-primary`, `--text-secondary`, `--text-muted`,
  `--text-inverse`.
- **Borders**: `--border-subtle`, `--border-strong`.
- **Accent**: `--accent-primary`, `--accent-primary-hover`.
- **Status** (single-hue-per-status, used only for status dots):
  `--status-success`, `--status-warning`, `--status-danger`,
  `--status-info`.
- **Spacing scale**: `--space-1` … `--space-6`.
- **Radii**: `--radius-sm`, `--radius-md`, `--radius-lg`.
- **Focus ring**: `--focus-ring`.

### FR-12.2 — Theme switching

- The UI SHALL support **light** and **dark** themes selected by a
  visible toggle in the top-right of the app shell.
- The initial theme SHALL follow the browser's `prefers-color-scheme`
  media query if the user has no stored preference.
- The chosen theme SHALL be persisted in `localStorage` under a
  namespaced key and re-applied on the next page load.
- Theme swapping SHALL update only design-token custom properties;
  no page re-render or JS re-evaluation of components SHALL be
  necessary.

### FR-12.3 — Primitive component library

The `src/components/ui/` directory SHALL provide the following
composable primitives, and application pages SHALL compose them
rather than raw HTML elements or ad-hoc class strings:

- `Button` (variants: primary, secondary, ghost, danger).
- `IconButton`.
- `Field` (label + description + error slot for a form control).
- `Input`, `Textarea`, `Select`, `Combobox`.
- `Checkbox`, `Toggle`, `Radio`.
- `Badge`, `Chip`.
- `Card`, `CardHeader`, `CardBody`, `CardFooter`.
- `StatTile` (for dashboard tiles).
- `Modal`, `Drawer`.
- `Table`, `DataTable`, `DataFrameView`.
- `Tabs`, `Segment`.
- `Toast` (via a `useToast` hook).
- `PermissionGate` (JSX-level RBAC gate — see FR-02).

### FR-12.4 — Application shell and navigation

- The app shell SHALL comprise a persistent left navigation, a
  top bar with theme toggle and user menu, and a main content
  area.
- The left nav SHALL show links to Dashboard, Connections,
  Catalog, Explore, Workflows, Audit, Users & Roles.
- Each nav item SHALL be **hidden** when the current user lacks
  the corresponding permission (rather than displayed and
  disabled).
- The active nav item SHALL be visually distinguished by
  background surface and accent border.

### FR-12.5 — Empty states

Every list-oriented page SHALL define a distinct empty state
that includes:

- A short heading describing what the page shows.
- A one-line explanation.
- A primary call-to-action (e.g. "Create your first connection")
  gated by the appropriate permission.

Empty states SHALL NOT be a blank canvas.

### FR-12.6 — Loading states

- Buttons SHALL show a spinner and become non-interactive while
  their action is pending.
- Full-page loads (route change) SHALL show a top-of-page progress
  strip.
- List loads SHALL show a **skeleton** matching the target row
  shape, not a generic spinner.
- Tables mid-refresh SHALL keep prior rows visible with a subtle
  overlay to preserve context.

### FR-12.7 — Error states

- Form errors SHALL be shown inline in a `Field`'s error slot,
  described by `aria-describedby`, and announced by screen readers
  via `role="alert"` on the containing region.
- Global errors (network failure, 500) SHALL be shown in a
  bottom-right `Toast` with a "Retry" action where applicable.
- Unrecoverable full-page errors SHALL be shown in a page-level
  error component with the classified error code and remediation
  string (see FR-05 / FR-11).

### FR-12.8 — Accessibility (WCAG 2.1 AA)

- All text SHALL meet a minimum 4.5:1 contrast ratio against its
  surface. Large text SHALL meet 3:1.
- All interactive elements SHALL be reachable and operable via
  keyboard (Tab focus, Space/Enter to activate).
- All form controls SHALL be labelled — either visibly via a
  `Field` label or via `aria-label`. Placeholder is never the
  sole label.
- Icons SHALL have textual companions or `aria-label` when they
  are the only affordance.
- Status color SHALL always be accompanied by a shape / text
  label (a red dot is always next to a "Failed" chip; a green
  dot next to "Healthy").
- Focus SHALL be visible via the `--focus-ring` token on every
  focusable element.
- The tab order SHALL follow document flow; there SHALL NOT be a
  `tabindex > 0`.
- Modals SHALL trap focus while open and return focus to the
  invoking element on close.
- Route changes SHALL announce the new page title to screen
  readers.

### FR-12.9 — Keyboard shortcuts

- `Ctrl/Cmd+K` opens a global command palette (future
  enhancement stub) — until implemented, opens the search modal
  for the current page.
- `Ctrl/Cmd+S` saves the current form / workflow.
- `Ctrl/Cmd+Enter` runs the current query / workflow.
- `Escape` closes modals, drawers, and command palettes.
- `?` opens a keyboard-shortcut cheat sheet.

### FR-12.10 — Responsive layout

- The UI SHALL render usably on screen widths ≥ 1024px.
- Screen widths < 1024px SHALL show a "This app is designed for
  a wider screen" advisory but SHALL still render pages so a user
  can perform read-only inspection.
- Layout SHALL NOT rely on pixel-perfect widths; components
  SHALL flex within their container.

### FR-12.11 — Content Security Policy compatibility

- The frontend SHALL be built to run under a strict CSP: no
  inline scripts, no eval, no dynamic style attributes.
- Any icon-font / inline SVG usage SHALL be inline JSX
  components, not `dangerouslySetInnerHTML`.
- Vite build SHALL emit content-hashed asset filenames so
  aggressive caching (immutable, 1-year `max-age`) is safe.

### FR-12.12 — Internationalization readiness

- The UI copy SHALL be authored in English but strings SHALL live
  in dedicated modules keyed by semantic slug, so a future i18n
  layer can wrap them.
- Dates SHALL be rendered in the browser's locale via
  `Intl.DateTimeFormat`.
- Numbers SHALL be rendered via `Intl.NumberFormat`.

## UI/UX requirements

- Screenshots documenting the design language:
  - [`docs/screenshots/01-login.png`](../screenshots/01-login.png) — auth flow.
  - [`docs/screenshots/02-dashboard-light.png`](../screenshots/02-dashboard-light.png) — light theme.
  - [`docs/screenshots/03-dashboard-dark.png`](../screenshots/03-dashboard-dark.png) — dark theme.
  - [`docs/screenshots/04-connections.png`](../screenshots/04-connections.png) — a data table.
  - [`docs/screenshots/07-workflows.png`](../screenshots/07-workflows.png) — a resource list.
  - [`docs/screenshots/08-workflow-builder-dark.png`](../screenshots/08-workflow-builder-dark.png) — canvas surface.
  - [`docs/screenshots/10-audit-log.png`](../screenshots/10-audit-log.png) — dense data table.
  - [`docs/screenshots/11-users-roles.png`](../screenshots/11-users-roles.png) — admin surface.
- **The design language forbids**:
  - Multi-hue palettes for anything other than status.
  - Gradients as primary surface fills.
  - Drop shadows deeper than 4dp equivalent.
  - Border radii above `--radius-lg` (rounded pills are the
    exception, applied only to Chips).
- **The design language mandates**:
  - Consistent left-alignment of text; right-alignment reserved
    for numeric columns.
  - Consistent iconography from a single set (Lucide-style stroke
    icons at a fixed weight).
  - Tables that are dense but breathable (adequate cell padding).

## Acceptance criteria

- [ ] The theme toggle switches every surface, text, and border
  color instantly with no flash-of-unstyled-content.
- [ ] Theme preference persists across full-page reloads.
- [ ] On a fresh install with no preference, the theme matches
  `prefers-color-scheme`.
- [ ] Every interactive element on Login, Connections, Explore,
  Workflows, and Audit is reachable via Tab in a sensible order.
- [ ] Focus outlines are visible in both themes on every focusable
  element.
- [ ] `aria-label` or a visible label exists on every form control
  (verified by automated axe scan).
- [ ] Contrast for body text meets ≥ 4.5:1 in both themes
  (verified by automated axe scan).
- [ ] The Connections page shows a first-run empty state with a
  "Create connection" CTA when zero connections exist.
- [ ] Submitting an invalid form surfaces the error under the
  offending Field with `role="alert"` on the error region.
- [ ] Route change to a new page announces the page title to
  screen readers (`document.title` update + `aria-live` region).
- [ ] Nav items are hidden entirely for permissions the user does
  not have (not merely disabled).
- [ ] Modal open traps focus; Escape closes; focus returns to the
  invoking element on close.
- [ ] Vite build produces content-hashed asset filenames and
  emits no inline `<script>` tags.

## Edge cases & error handling

- **User overrides system theme repeatedly**: The stored preference
  wins over `prefers-color-scheme` until the user explicitly
  clears it (Settings action, future).
- **CSS custom-property support**: Data Explorer supports evergreen
  browsers only (last two versions of Chrome, Firefox, Safari,
  Edge). Older browsers SHALL show a compatibility banner.
- **JS disabled**: The app is a SPA; JS is required. A no-JS
  fallback shows an explanatory static page and a support link.
- **Slow network**: The app shell renders as soon as the entry
  bundle downloads; individual pages fetch data with skeletons and
  progressive rendering.
- **RTL languages**: Not supported in the current release; the
  UI is LTR only.
- **Very small viewport**: A `< 1024px` viewport shows an advisory
  banner but does not block rendering.
- **Zoom to 200%**: The layout SHALL not clip content at 200%
  zoom in a 1440x900 viewport.
- **User system theme changes mid-session**: The app respects the
  live media-query change only if the user has not chosen a
  preference.

## Non-functional requirements

- **Bundle size**: Initial JS bundle SHALL be ≤ 500 KB gzipped
  measured on a fresh Vite production build.
- **First contentful paint**: SHALL be ≤ 1500 ms on a mid-range
  laptop over LAN.
- **Time to interactive**: SHALL be ≤ 3 s on a mid-range laptop
  over LAN.
- **Component consistency**: 100% of interactive controls SHALL
  be built from `src/components/ui/` primitives (enforceable via
  a lint rule).
- **Design token discipline**: Zero hex color literals SHALL
  appear outside `src/index.css` (enforceable via a lint rule).
- **Automated accessibility**: An `axe` scan SHALL run in CI on
  a representative page set with zero critical issues.
- **Cross-browser**: The app SHALL render correctly in the last
  two major versions of Chrome, Firefox, Safari, and Edge.
- **Security**: The frontend SHALL NOT embed backend URLs in
  build-time constants; all API calls go through same-origin
  paths (`/api/v1/…`).

## Market context & differentiation

| Product           | Visual identity                                   | Notes                                                                              |
| ----------------- | ------------------------------------------------- | ---------------------------------------------------------------------------------- |
| **Grafana**       | Dense, dark-first, chart-forward                  | Excellent theming; more colorful; visualization-centric.                            |
| **Retool**        | Neutral, professional                             | App-builder look; can feel busy in complex apps.                                    |
| **Metabase**      | Friendly, brand-forward                           | Approachable colors; less compact than Data Explorer.                               |
| **Superset**      | Utilitarian, chart-forward                        | Less refined visual system.                                                         |
| **n8n**           | Blue accent, playful                              | Node canvas is polished; palette is more colorful.                                  |
| **Airbyte**       | Product-forward, illustration-heavy                | Marketing-y feel bleeds into product.                                               |
| **Postman**       | Orange accent, dense sidebar                      | Iconic; brand-heavy.                                                                |
| **Hex / Deepnote**| Modern, whitespace-heavy                          | Notebook-focused; less dense list surfaces.                                         |

Data Explorer's differentiators for UI/UX:

- **Near-monochrome intentional restraint.** Status hues are the
  only color; the data stands out.
- **Compact but breathable density.** More information per
  viewport than most competitors without becoming cramped.
- **One design-token system.** Every color, radius, and space
  value is a token, enforced structurally.
- **A11y from day one, not as an add-on.** Focus rings, alt
  text, keyboard traps, and contrast targets are part of the
  baseline.
- **Permission-hidden nav.** Users don't see clickable roads to
  dead ends.
- **Same primitives on every screen.** Learning curve is a
  single design system, not per page.
- **CSP-friendly out of the box.** No inline scripts, no eval,
  hashed asset filenames.

## Future enhancements (out of scope)

- Command palette (`Ctrl/Cmd+K`) with fuzzy search across
  connections, workflows, runs, and settings.
- Full internationalization (multiple language packs).
- RTL layout support.
- User-customizable themes (arbitrary accent color).
- Per-user layout preferences (compact vs comfortable density).
- Accessibility statement page linked from the footer.
- Automated visual-regression testing (screenshot diff on every
  PR).
- Mobile-friendly layout for tablet/phone screens.
- Onboarding tour / product walkthrough for new users.
- User preference for reduced motion.

## Cross-references

- [FR-01 Authentication & Session Management](./FR-01-authentication-and-sessions.md)
- [FR-02 Role-Based Access Control (RBAC)](./FR-02-role-based-access-control.md) —
  `PermissionGate` primitive.
- [FR-05 Connection Health Monitoring](./FR-05-connection-health-monitoring.md) —
  status-dot conventions.
- [FR-06 Ad-Hoc Data Exploration](./FR-06-ad-hoc-exploration.md) —
  `DataFrameView` primitive.
- [FR-07 Visual Workflow Builder](./FR-07-visual-workflow-builder.md) —
  canvas surface conventions.
- [FR-11 Observability, Guardrails & Reliability](./FR-11-observability-and-guardrails.md) —
  error-code taxonomy surfaced in the UI.
- [`../screenshots/`](../screenshots/) — visual reference for every screen.
