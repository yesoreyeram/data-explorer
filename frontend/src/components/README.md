# src/components

## What this package does

`src/components` contains **shared, application-aware React components** that are used across multiple pages. Unlike `components/ui` (pure design-system primitives), these components know about the application's data model and API types.

## Components

### DataFrameView (`DataFrameView.tsx`)

The primary component for rendering a `DataFrame` API response. Provides:

- **Table / Chart / Split** view switcher (tab bar).
- **Schema panel** — column type badges, nullable indicators, per-column stats.
- **DataTable** — virtualized table for up to 100,000 rows; sticky header.
- **Chart view** — line, bar, scatter with locally-persisted chart settings.
- **Metadata bar** — row count, column count, source type/ID, duration, truncation warning.
- **CSV/JSON export** buttons (via `lib/exportFrame.ts`).
- **"Save chart"** action — saves the current chart config + query result to `savedChartsStore`.

Used by: `ExplorePage`, `WorkflowBuilderPage` (node output preview), connection query modal.

### DataTable (`DataTable.tsx`)

Virtualized table component. Accepts `schema` + `rows` directly (no `DataFrame` wrapper). Handles:
- Sticky header row.
- Column width auto-sizing.
- Cell truncation with tooltip for long values.
- Keyboard navigation (arrow keys).

### Modal (`Modal.tsx`)

Accessible modal dialog. Manages focus trap, `Escape` to close, and `aria-modal`. Used throughout the app for forms and confirmations.

### PermissionGate (`PermissionGate.tsx`)

```tsx
<PermissionGate permission="connections:write">
  <CreateConnectionButton />
</PermissionGate>
```

Renders children only if the current user has the specified permission (from `authStore`). Falls back to `null` (or a custom `fallback` prop). **This is a UX nicety — authorization is also enforced server-side on every API call.**

### ProtectedRoute (`ProtectedRoute.tsx`)

React Router route wrapper that redirects to `/login` if the user is not authenticated. Used by `App.tsx` to protect all app routes.

### ThemeSwitcher (`ThemeSwitcher.tsx`)

Dropdown to toggle between light / dark / system theme. Reads and writes `themeStore`.

### StatusBadge (`StatusBadge.tsx`)

Maps `ConnectionStatus` and `WorkflowExecutionStatus` values to a `<Badge>` with the appropriate `status` prop. Centralized so status → color mapping is in one place.

### QuerySpecFields (`QuerySpecFields.tsx`)

Shared form fragment for authoring a `QuerySpec` (SQL query, REST request, GraphQL query, or cloud-specific query shape). Renders the correct fields based on the connection type. Used by `ExplorePage` and the connection-test modal.

### CloudQueryFields (`CloudQueryFields.tsx`)

Form fields specific to cloud query specs (Athena SQL, CloudWatch log group/time range, DynamoDB key expressions, etc.). Rendered by `QuerySpecFields` for `aws`/`gcp`/`azure` connections.

### PaginationFields (`PaginationFields.tsx`)

Form fields for configuring REST pagination spec (strategy, page size, cursor path, etc.).

### icons (`icons.tsx`)

SVG icon components used throughout the app. Single file to avoid icon library dependencies for the small set of icons needed.

### layout/

`AppLayout.tsx` — the main shell: collapsible sidebar + topbar + content area. Sidebar width and topbar height are CSS custom properties (`--sidebar-width`, `--topbar-height`) from `tokens.css`.

### charts/

Chart wrappers (Recharts or similar) used by `DataFrameView`. Isolated so the charting library can be swapped without touching `DataFrameView`.

## Architecture decisions (ADRs)

| Decision | Rationale |
|---|---|
| `PermissionGate` wraps children, not routes | Fine-grained UI hiding; `ProtectedRoute` handles the coarser "logged in?" check |
| `QuerySpecFields` extracted | Shared between `ExplorePage` and the connection-test modal; extracted to avoid duplication |
| `DataFrameView` owns export and chart | Every screen that renders a frame gets export and charting for free without extra wiring |

## Scope and responsibilities

- Provide reusable, application-aware UI components.
- Render `DataFrame` wire responses as tables and charts.
- Manage permission-gated rendering.
- Provide the application layout shell.

## Limitations and todos

- [ ] `DataTable` is not fully accessible for screen readers (virtualization complicates ARIA live region support).
- [ ] Chart types are limited (line, bar, scatter); no pie, heatmap, or time-series zoom.
- [ ] `QuerySpecFields` grows as new connector types are added; may need to be split into per-type components.
- [ ] No dark mode for chart tooltip backgrounds (inherits Recharts defaults).
