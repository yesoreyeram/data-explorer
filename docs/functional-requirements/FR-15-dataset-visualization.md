# FR-15 — Dataset Visualization

## Overview

Today the primary way to look at a query result in Data Explorer is a
**tabular grid** (`DataFrameView`), and the primary way to leave with
the answer is CSV / JSON export or a downstream workflow node
(see FR-06 and FR-09). Tables are truthful but rarely
*persuasive* — a stakeholder skimming a chat message wants a chart,
not a table. This FRD introduces **dataset visualization** as a
first-class output of Explore and workflow runs: users can render
their query result as a chart (line, bar, area, pie, scatter,
histogram, single-value KPI, heatmap, table variant) with a small
visual editor, embed the chart into a lightweight dashboard, and
export the chart as an image.

The scope is deliberately narrower than a full BI product: no
multi-source joins in the visualization layer (that's the workflow
builder's job), no drag-drop dashboard layout with tens of tiles,
no complex calculated-field DSL beyond what already exists in
workflow transforms (JSONata). The visualization surface is
purpose-built for **communicating a single result** cleanly.

## Product goals

- Turn every Explore result and every workflow output into a
  chart with **at most three interactions** (choose type, choose
  columns, save).
- Let users **switch between table and chart** views on the same
  frame without re-running the query.
- Provide a **small, opinionated chart library** — 10 chart types,
  each with sensible defaults, rather than a customization
  labyrinth.
- Enable **image export** (PNG / SVG) for pasting a chart into
  chat / docs / email.
- Introduce **lightweight dashboards** — a page that renders
  several saved queries side by side, updated on schedule.
- Make **accessibility and colorblindness** first-class: every
  chart palette is verified colorblind-safe; every chart has a
  data-table fallback exposed to assistive tech.

## User personas

| Persona                    | Description                                                                                              |
| -------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Analyst**                | Wants to convert a SQL result into a bar chart to send to a colleague.                                   |
| **Ops engineer**           | Wants a single-value KPI tile ("Current open incidents = 3") on a dashboard.                             |
| **Product manager**        | Wants a small dashboard showing daily signups, weekly active users, and error rate.                      |
| **Executive**              | Wants a screenshotable chart of last-quarter's numbers.                                                  |
| **Colorblind user**        | Needs a palette that is distinguishable without color perception.                                        |
| **Screen-reader user**     | Needs the underlying tabular data accessible to assistive tech even when the chart is a canvas element.  |
| **Non-technical viewer**   | Wants to look at a dashboard without editing anything.                                                   |

## User stories — **existing functionality (10)**

These stories refine or extend rendering surfaces that already exist
today (the `DataFrameView` grid, the Explore result panel, the
workflow run panel).

- **US-15.1** *(refines FR-06 result grid.)* As an analyst, I want to
  **toggle** the Explore result between "Table" and "Chart" views
  without re-running my query, so I can pick the best presentation
  for the same underlying frame.
- **US-15.2** *(refines FR-12 DataFrameView.)* As an analyst, I want
  the table view to gain **inline column histograms** (a tiny
  sparkline in each column header) so I can eyeball distributions
  without leaving the grid.
- **US-15.3** *(refines FR-12 DataFrameView.)* As an analyst, I want
  the table view to gain **column sort by clicking a header** so I
  can inspect top/bottom rows quickly.
- **US-15.4** *(refines FR-12 DataFrameView.)* As an analyst, I want
  a **column pinning** affordance so long-frame scrolling keeps
  key columns (e.g. id, name) visible.
- **US-15.5** *(refines FR-06 result panel.)* As an analyst, I want
  the result panel to show **summary statistics** (row count,
  column count, null count per column, min/max per numeric column)
  in a collapsible header, so I understand the frame's shape at a
  glance.
- **US-15.6** *(refines FR-09 export.)* As an analyst, I want the
  existing **Export CSV / JSON** buttons to be joined by **Export
  PNG / SVG** when a chart is showing, so I can share the chart
  the same way I share the data.
- **US-15.7** *(refines FR-07 workflow run panel.)* As an editor, I
  want the workflow run panel to render an **inline chart per
  node** where the node's frame has an obvious 2-column shape,
  so I get instant visual feedback while iterating.
- **US-15.8** *(refines FR-12 accessibility.)* As a screen-reader
  user, I want the table view to gain **`aria-rowcount` and
  `aria-colcount` attributes** so my assistive tech announces
  frame size correctly.
- **US-15.9** *(refines FR-05 error taxonomy.)* As an analyst, I
  want **visualization failures** (unknown column, incompatible
  type for a chart type) to classify to `invalid_config` with a
  remediation naming the offending column and chart type, so
  errors feel consistent with the rest of the product.
- **US-15.10** *(refines FR-12 theming.)* As a user, I want charts
  to **respect the app's light/dark theme** and to update
  instantly on theme toggle, so my visualizations look native to
  the app.

## User stories — **new features (10)**

- **US-15.11** As an analyst, I want to pick from a **library of
  10 chart types** (line, bar, stacked bar, area, pie/donut,
  scatter, histogram, heatmap, single-value KPI, sparkline
  table).
- **US-15.12** As an analyst, I want a **visual editor** that
  binds columns to visual encodings (x, y, series, color, size)
  with dropdowns, so I don't need to write chart configuration
  by hand.
- **US-15.13** As an analyst, I want to **save the chart config**
  as a "Saved chart" resource tied to a query and reload it later.
- **US-15.14** As a product manager, I want a **lightweight
  dashboard** page composed of up to 12 saved charts arranged in
  a responsive grid.
- **US-15.15** As a dashboard owner, I want dashboards to
  **auto-refresh** on a configurable cadence (off, 30s, 1m,
  5m, 15m, 1h) so numbers stay live during a review.
- **US-15.16** As an executive, I want a **read-only "presentation
  mode"** that fills the browser window with a single dashboard,
  hides all chrome, and rotates through dashboards if configured.
- **US-15.17** As a colorblind user, I want to pick from
  **colorblind-safe palettes** (default) and see palette previews
  with simulated deficiency modes before selecting.
- **US-15.18** As an analyst, I want to **hover a chart** to see
  a tooltip with the underlying row values for the point hovered.
- **US-15.19** As an analyst, I want to **click-drag zoom** on
  time-series charts to narrow the visible window, with a "Reset
  zoom" affordance.
- **US-15.20** As any user, I want **shareable dashboard URLs**
  (subject to RBAC on the underlying queries) so I can send a
  link to my dashboard.

## Functional requirements

### FR-15.1 — Chart type library

The visualization module SHALL support these chart types out of
the box:

- **Line** (single or multi-series).
- **Bar** (grouped).
- **Stacked bar** (100% stack option).
- **Area** (grouped or stacked).
- **Pie / donut** (single dimension, single measure).
- **Scatter** (x, y, optional color, optional size).
- **Histogram** (single measure with automatic bucket sizing).
- **Heatmap** (x, y, value).
- **Single-value KPI** (measure + optional comparison / delta).
- **Sparkline table** (each row an inline mini-chart).

Chart types SHALL declare which frame shapes they accept; incompatible
frames SHALL be surfaced with the standard error taxonomy.

### FR-15.2 — Encoding editor

The chart-config panel SHALL expose:

- **Chart type** dropdown.
- **X axis** column dropdown (populated from the frame's columns).
- **Y axis** column dropdown, or **Value** for KPI.
- **Series** / **Color** / **Size** column dropdowns as
  appropriate to the chart type.
- Axis label + unit inputs (optional).
- Number-format dropdown (integer, decimal, percent, currency,
  compact).
- Date-format dropdown (iso, short, long) for time axes.
- Palette dropdown with colorblind-safe presets.

Change to any field SHALL re-render the chart in-place without a
server round trip.

### FR-15.3 — View toggle on the result panel

The Explore result panel and workflow run panel SHALL expose a
**View** control with three options: **Table** (default),
**Chart**, **Both** (side-by-side). Switching views SHALL preserve
the underlying frame and the chart config; the toggle is
per-view state, not per-frame.

### FR-15.4 — Summary statistics

Above the frame grid, the result panel SHALL show:

- Row count (with an "of N" indication when truncated by the row
  cap).
- Column count.
- Per-column summary chip on hover: null count, distinct-value
  count (up to 10K), and (for numeric columns) min / max / mean.

Summary computation SHALL be client-side and SHALL not exceed 50ms
at p95 for a 10K-row frame.

### FR-15.5 — Column histograms in headers

Each numeric column header in the table view SHALL show a small
histogram (10 buckets) reflecting the visible frame's distribution
in that column. Nulls SHALL be shown as a subtle "null" tick in
the histogram footer.

### FR-15.6 — Column sort and pin

Table view SHALL support:

- Click a header to cycle sort direction: none → ascending →
  descending → none.
- Right-click / kebab-menu on a header to **Pin left** or
  **Pin right**. Pinned columns SHALL remain visible during
  horizontal scroll.

Sorting SHALL be client-side over the visible frame and SHALL not
re-query the source.

### FR-15.7 — Chart tooltips and interactions

Every chart type SHALL support:

- **Hover tooltip** with the underlying row values for the
  hovered mark.
- **Legend** click to toggle series visibility.
- **Zoom** (click-drag on time-series charts; wheel-zoom on
  scatter). A **"Reset zoom"** button appears when zoomed.
- **Pan** on zoomed charts by drag.

### FR-15.8 — Image export

When a chart is showing, the export menu SHALL add:

- **Export PNG** — rasterized at 2× DPI for retina, transparent
  background option.
- **Export SVG** — vector, all fonts embedded as text (not
  outlines).

Image export SHALL happen client-side and SHALL match the on-screen
chart exactly.

### FR-15.9 — Saved chart resource

Charts SHALL be a savable resource:

- Fields: `id`, `name`, `query_ref` (Explore query snapshot or a
  workflow-node reference), `chart_config` (JSON), `owner_id`,
  `visibility` (`private`, `team`), timestamps.
- Saving a chart requires `charts:write`.
- Viewing a chart requires `charts:read` **and** read permission
  on the underlying resource (query/connection/workflow).

### FR-15.10 — Dashboards

A **Dashboard** SHALL be a resource with:

- Fields: `id`, `name`, `description`, `layout` (grid), `charts[]`
  (references to saved charts with per-cell size and position),
  `refresh_interval_seconds`, `visibility` (`private`, `team`),
  timestamps.
- Up to **12 charts per dashboard**.
- A responsive grid layout (12 columns, drag to reorder, resize
  via corner handles).
- Auto-refresh options: off, 30s, 1m, 5m, 15m, 1h.
- Presentation mode: fullscreen render, chrome hidden, ESC to
  exit.
- Dashboards SHALL be permission-checked against every underlying
  chart before rendering.

### FR-15.11 — Colorblind palettes

The palette selector SHALL include at least:

- **Categorical safe** (Wong / Okabe-Ito, 8 colors).
- **Sequential blue** (perceptually uniform).
- **Sequential viridis** (perceptually uniform).
- **Diverging red-blue** (colorblind-safe).

Palette preview SHALL show three modes: normal, deuteranopia,
protanopia so users see what each palette looks like under common
deficiencies.

### FR-15.12 — Theme-aware rendering

Charts SHALL read colors from design tokens where applicable:

- Axis lines / labels / gridlines from `--text-secondary` /
  `--border-subtle`.
- Chart background transparent so it blends with `--surface-1`.
- Only the data series use palette colors.

Theme toggle SHALL update charts within one animation frame.

### FR-15.13 — Accessibility

- Every chart SHALL render a hidden `<table>` companion element
  with the underlying data, associated with the chart via
  `aria-describedby`, so screen readers can read the values.
- Every chart SHALL have a text summary ("Line chart of daily
  revenue over 30 days, peak $12.4k on Feb 14") rendered in an
  `aria-live="polite"` region.
- Chart color SHALL never be the sole encoding; shape / pattern
  / label reinforcement SHALL be provided for series.
- Keyboard interaction on charts: `Tab` focuses each series;
  `←` / `→` step through points on a line/bar; a tooltip renders
  for the focused point.

### FR-15.14 — Rendering performance

Charts SHALL:

- Render an initial frame within **200ms** at p95 for a 1000-row
  frame on a mid-range laptop.
- Use a canvas-based renderer for scatter and heatmap where
  point count exceeds 500.
- Downsample time-series to at most 2× viewport-pixel-width points
  before rendering; downsampling SHALL be visually indistinguishable
  from the raw data at that resolution.

### FR-15.15 — Chart-config validation

Chart configs SHALL be validated on save:

- Referenced columns SHALL exist in the query result schema.
- Column types SHALL be compatible with the chart type (numeric
  measure, temporal or categorical dimension).
- Config JSON SHALL respect a bounded schema (documented alongside
  the frontend).

Invalid configs SHALL be rejected with `invalid_config` and a
remediation string naming the offending field.

## UI/UX requirements

- The chart panel replaces (in "Chart" view) or shares (in "Both"
  view) the result grid area of Explore / run panel.
- The chart-config side rail is collapsible and remembers its
  state per-user.
- Chart types are chosen from a compact 2×5 icon grid so the
  library is discoverable without a dropdown.
- The dashboard grid uses the design system's `Card` primitive as
  each cell; the drag handle is a subtle icon in the cell's top-
  right.
- Presentation mode uses the entire viewport including the top
  bar; `Esc` returns to normal mode.
- Empty dashboard shows "Add your first chart" CTA gated by
  `charts:write` and `dashboards:write`.
- Auto-refresh indicator is a subtle dot next to the dashboard
  title with a tooltip showing "Refreshes every N minutes".

## Acceptance criteria

- [ ] Toggling the Explore result from Table to Chart on a
  10-row frame renders a bar chart within 300ms.
- [ ] Choosing a chart type incompatible with the frame's column
  types shows an `invalid_config` error naming the offending
  column and chart type.
- [ ] Clicking a numeric column header sorts the visible frame
  descending on the second click.
- [ ] Pinning the "id" column keeps it visible while scrolling
  horizontally.
- [ ] Hovering a bar in a bar chart shows a tooltip with the
  underlying row values.
- [ ] Clicking a legend entry toggles the series' visibility.
- [ ] Click-drag on a line chart zooms into the selected time
  window and a "Reset zoom" button appears.
- [ ] Export PNG produces an image whose visible content matches
  the chart on screen at 2× DPI.
- [ ] Export SVG produces a valid SVG file that renders correctly
  in Chrome, Firefox, and Safari.
- [ ] Saving a chart requires `charts:write`; viewing requires
  the underlying resource permission.
- [ ] A dashboard with 6 charts refreshes every 5 minutes when
  the auto-refresh is set to 5m.
- [ ] Presentation mode hides all chrome; `Esc` returns to the
  editor.
- [ ] Toggling app theme updates chart backgrounds and axis
  labels within one animation frame without user action.
- [ ] Each chart renders a hidden data table with the underlying
  values, accessible via screen reader.
- [ ] The default palette passes colorblind-safety tests for
  deuteranopia and protanopia.
- [ ] A dashboard the user lacks permission on any referenced
  chart is rendered with a redacted placeholder for that cell
  and a clear "Access restricted" note (RBAC-safe).

## Edge cases & error handling

- **Empty frame → chart**: The chart area shows "No data to
  visualize" with the standard neutral empty-state visual.
- **Single-row frame → line chart**: The chart renders a single
  point with a friendly note "A line chart needs at least two
  points."
- **All-null column selected as measure**: The chart shows
  "Selected column is all null" with a link to change the
  encoding.
- **Very wide frame (500 cols)**: Column selectors are searchable
  dropdowns; the chart config never enumerates more than 100
  columns in the dropdown at once.
- **Time axis with strings that aren't dates**: The chart
  auto-detects and shows a hint "Column `date_str` does not look
  like a date — parse it as a date in a transform node."
- **Dashboard with a deleted saved chart**: The cell renders a
  neutral error card with a "Remove from dashboard" affordance.
- **Presentation mode on small screen**: The dashboard grid
  falls back to a single-column stack; charts scale to viewport
  width.
- **PNG export on transparent background with dark theme**:
  Users pick a background option: transparent (default),
  match-theme (opaque `--surface-1`), or explicit white / black.
- **Zoom past the data range**: Zooming further than 5% of the
  data range is clamped; a subtle indicator explains "Minimum
  zoom range."
- **Chart type unsupported by a frame with the wrong number of
  columns**: The chart-type selector disables incompatible
  types with an inline tooltip explaining why.
- **Auto-refresh while a run is in progress**: Auto-refresh is
  suspended until the current run completes to avoid stale
  overlays.

## Non-functional requirements

- **Latency**: Chart re-render on config change SHALL take ≤ 100ms
  at p95 for a 1000-row frame.
- **Bundle size**: The visualization module SHALL be
  code-split from the main bundle; opening a chart lazily loads
  the library.
- **Memory**: A single chart SHALL use ≤ 50 MB of browser memory
  at p95, including its data source frame.
- **Rendering fidelity**: Charts SHALL be pixel-consistent across
  the last two versions of Chrome, Firefox, and Safari.
- **Accessibility**: A representative set of chart types SHALL
  pass `axe` and manual screen-reader audits with zero critical
  issues.
- **Security**: SVG export SHALL sanitize any user-provided
  labels/titles to prevent XSS in downstream consumers.
- **Observability**: Chart-render errors SHALL emit metric
  `chart_render_error_total{type,code}`.
- **Determinism**: For a given frame and chart config, the chart
  SHALL be pixel-identical across renders (no time-based
  randomness).

## Market context & differentiation

| Product           | Visualization surface                                                | Notes                                                                     |
| ----------------- | -------------------------------------------------------------------- | ------------------------------------------------------------------------- |
| **Metabase**      | Rich chart library, dashboards, filters                              | Best-in-class for BI-style dashboards.                                    |
| **Superset**      | Deep chart library                                                   | Powerful; steeper learning curve.                                         |
| **Grafana**       | Time-series first, panels & dashboards                               | Best for observability; less for ad-hoc business charts.                  |
| **Looker Studio** | Report-first with rich charts                                        | SaaS-locked to Google; excellent Google Sheets integration.               |
| **Retool**        | Chart components inside apps                                         | Ties visualization to app authoring.                                      |
| **Hex / Deepnote**| Notebook cells with Vega / Plotly                                    | Powerful; heavyweight for a single-chart task.                            |
| **Tableau**       | Deep visual analytics                                                | Desktop-first / SaaS; heavyweight.                                        |
| **Datadog**       | Time-series dashboards                                               | Observability-focused.                                                    |

Data Explorer's differentiators for visualization:

- **10 opinionated chart types**, not 100. Choosing is fast.
- **Chart is a lightweight overlay on the existing result panel.**
  No new authoring context to enter.
- **Theme-aware and colorblind-safe by default.**
- **Accessibility built in.** Hidden data tables, text summaries,
  keyboard focus on series.
- **Client-side render**, no per-chart server call — theme swap
  and encoding change are instant.
- **Dashboards are 12-cell max on purpose.** Focus over
  everything.
- **RBAC-safe dashboards.** Cell-level permission enforcement.
- **Instant PNG / SVG export.** No plugin, no login-to-share.
- **Single-value KPI + sparkline table**, first-class citizens.
- **No custom DSL to learn.** Encodings map to columns via
  dropdowns.

## Future enhancements (out of scope)

- Free-form annotation layer on charts.
- Cross-chart filtering (click a bar to filter neighbouring
  charts).
- Templated dashboards with variables.
- Embedded / anonymous dashboards for external stakeholders.
- Alert rules on dashboard values (integrating with an alerting
  system).
- Custom color palettes.
- Custom chart types via plugin.
- Multi-source charts (joining frames at the visualization
  layer).
- Automated chart-type suggestion from frame shape.
- Chart versioning.
- PDF export for dashboards.

## Cross-references

- [FR-06 Ad-Hoc Data Exploration](./FR-06-ad-hoc-exploration.md) — the primary surface visualization augments.
- [FR-07 Visual Workflow Builder](./FR-07-visual-workflow-builder.md) — per-node chart previews.
- [FR-09 Query Result Export & Sharing](./FR-09-query-result-export.md) — image export extends the export menu.
- [FR-12 UI, Theming & Accessibility](./FR-12-user-interface-and-accessibility.md) — theme and a11y conventions charts follow.
- [FR-14 Navigation & Routing Improvements](./FR-14-navigation-and-routing.md) — dashboards and saved charts appear in the command palette.
- [FR-16 Data Frame Optimizations](./FR-16-data-frame-optimizations.md) — frames the visualization module renders.
