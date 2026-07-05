# src/pages

## What this package does

`src/pages` contains **one React component per application route**. Each page component is responsible for:
- Fetching data for its route via `useQuery` (TanStack Query).
- Rendering the appropriate layout, components, and actions.
- Handling loading and error states.

All pages are rendered within `AppLayout` (sidebar + topbar) except for `LoginPage` and `RegisterPage`.

## Pages

### LoginPage (`LoginPage.tsx`)

- Email + password form with `POST /auth/login`.
- OIDC login buttons (if providers configured).
- Redirects to dashboard on success.
- No `AppLayout`; standalone full-screen form.

### RegisterPage (`RegisterPage.tsx`)

- Email + password + display name form with `POST /auth/register`.
- No `AppLayout`; standalone full-screen form.

### DashboardPage (`DashboardPage.tsx`)

- Overview: stat tiles (connection count, workflow count, recent executions).
- Saved charts gallery from `savedChartsStore` (locally persisted).
- Quick links to recent activity.

### ConnectionsPage (`ConnectionsPage.tsx`) + `connections/`

- List of all connections with status badges.
- Create / Edit connection modal (`ConnectionFormModal.tsx`):
  - Type selector (postgres, mysql, rest, graphql, aws, gcp, azure).
  - Config fields per type.
  - Secret fields (never pre-populated from the server).
  - Catalog browser integration (prefill from catalog entry).
- Test connection with inline health result display.
- Delete connection with confirmation.
- Connection health panel (`ConnectionHealthModal.tsx`):
  - Current status badge + error code + remediation message.
  - Recent check history (from audit log, scoped to this connection).

### ExplorePage (`ExplorePage.tsx`)

- Source picker: saved connection or temporary (inline) connection.
- Query authoring: `QuerySpecFields` (SQL, REST, GraphQL, or cloud-specific).
- Run query → `DataFrameView` result.
- Recent queries list (saved-connection mode only, from `localStorage`).
- Temporary connections: same per-type config form, credentials never persisted.

### WorkflowsPage (`WorkflowsPage.tsx`) + `workflow/`

- List of all workflows with status, last execution time, schedule badge.
- Create workflow (navigates to builder).
- Schedule modal: cron expression + presets + enable/disable toggle.
- Delete workflow.

### WorkflowBuilderPage (`WorkflowBuilderPage.tsx`)

- React Flow canvas: drag-and-drop node palette, edge wiring.
- Node config panel (right sidebar): per-node type config form.
- "Run" button: executes the workflow; per-node output preview in the canvas.
- Execution history panel: list of past runs with status, duration, per-node results.
- Auto-save on canvas changes.

### AuditLogPage (`AuditLogPage.tsx`)

- Filterable, paginated table of audit log entries.
- Filters: actor, action, resource type/ID, outcome, date range.
- Requires `audit:read` permission (gated by `PermissionGate` and the backend).

### UsersPage (`UsersPage.tsx`)

- List of all users with role badges and status.
- Assign/remove roles.
- Suspend/activate users.
- Requires `users:read` / `users:write` permissions.

### NotFoundPage (`NotFoundPage.tsx`)

- 404 fallback for unknown routes.

## Route structure

```
/                        → DashboardPage
/connections             → ConnectionsPage
/explore                 → ExplorePage
/workflows               → WorkflowsPage
/workflows/:id           → WorkflowBuilderPage
/audit                   → AuditLogPage
/users                   → UsersPage
/login                   → LoginPage (no AppLayout)
/register                → RegisterPage (no AppLayout)
*                        → NotFoundPage
```

All routes except `/login` and `/register` are wrapped in `ProtectedRoute`.

## Architecture decisions (ADRs)

| Decision | Rationale |
|---|---|
| One component per route | Clear mapping between URL and code; easy navigation |
| TanStack Query for server data | Caching, loading states, background refresh, and error handling in one pattern |
| Sub-folders for complex pages | `connections/` and `workflow/` have multiple modals/sub-components that belong to that page only |

## Scope and responsibilities

- Render the UI for each application route.
- Fetch and display data via TanStack Query.
- Handle user actions by calling `src/api` functions.
- Gate UI elements with `PermissionGate`.

## Limitations and todos

- [ ] `WorkflowBuilderPage` auto-save fires on every canvas change without debouncing — generates many API calls on rapid edits.
- [ ] No optimistic UI updates; mutations wait for server confirmation before updating the list.
- [ ] `ExplorePage` recent queries are per-browser (`localStorage`); no server-side history.
- [ ] `DashboardPage` saved charts are per-browser; no sharing.
- [ ] No deep-linking for modal open state (e.g., `/connections?create=true`).
