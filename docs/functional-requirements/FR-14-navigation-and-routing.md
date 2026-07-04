# FR-14 — Navigation & Routing Improvements

## Overview

Data Explorer's current navigation model is a persistent left rail plus
per-page URLs (see FR-12). It works, but it treats every page as an
island: users can't jump between related resources without traversing
menus, cannot bookmark deep-linked filtered views, cannot go
back/forward reliably through workflow edits, and cannot search for a
resource by name. This FRD introduces a **coherent navigation and
routing improvement plan** covering breadcrumbs, deep links,
back/forward semantics, a global command palette, tabbed multi-resource
work, per-user recents, saved views, cross-resource "jump to" actions,
and canonical URL shapes.

The goal is to make the product **navigable by intent** — "go to that
workflow", "come back to what I was doing", "find that connection I
touched yesterday" — not merely by clicking through the tree.

## Product goals

- Reduce the number of clicks required to reach a known resource
  from any starting point to **one** (via search / command palette)
  or **two** (via nav + list).
- Make **every meaningful application state URL-addressable** so
  users can bookmark, share, and browser-history their way around.
- Give users a **stable notion of "back"** — the browser back button
  should always work sensibly, including inside modals and drawers.
- Provide **cross-resource jumps**: from a connection to the
  workflows using it, from an audit entry to the resource it
  references, from a run to the workflow it belongs to.
- Give operators one place to see **recent work** without hunting
  through each list.
- Support **tabbed workspaces** so users can compare, cross-reference,
  and switch between related resources without losing state.

## User personas

| Persona                | Description                                                                                          |
| ---------------------- | ---------------------------------------------------------------------------------------------------- |
| **Power user**         | Uses the platform daily; wants keyboard shortcuts and command palettes to move at speed.             |
| **Casual analyst**     | Uses the platform weekly; wants clear breadcrumbs and "back" that behaves like every other web app.  |
| **Ops engineer**       | Investigates incidents; needs to jump between an alert → audit entry → workflow → run → connection.  |
| **Admin**              | Manages users, roles, and connections; wants to bookmark filtered views for repeated checks.         |
| **New user**           | Doesn't know the sitemap yet; needs discoverability from any starting page.                          |
| **Screen-reader user** | Needs each page transition and modal to be announced.                                                 |

## User stories — **existing functionality (10)**

These stories extend or refine navigation surfaces that already exist
today (left nav, page routes, permission-gated menus per FR-12).

- **US-14.1** *(refines FR-12.4 left nav.)* As a returning user, I
  want the left nav to **remember which section I was on** across
  full-page reloads so I don't have to re-navigate after a refresh.
- **US-14.2** *(refines the Connections list.)* As an analyst, I want
  the Connections page URL to **encode the current filter state**
  (`?type=postgres&status=unhealthy`) so I can bookmark it and share
  the exact view.
- **US-14.3** *(refines the Workflows list.)* As an editor, I want
  the Workflows list to support **URL-encoded sort and search**
  (`?sort=updatedAt&search=orders`) so my saved link opens the
  exact list I expect.
- **US-14.4** *(refines the Audit page.)* As a compliance officer,
  I want the audit log URL to **capture the full filter state**
  (`?actor=alice&action=connections.updated&from=…&to=…`) so I can
  bookmark the query I use for weekly compliance sweeps.
- **US-14.5** *(refines FR-07 canvas.)* As an editor, I want the
  Workflow Builder URL to include the **selected node id**
  (`/workflows/{id}?node=filter-1`) so a link opens the builder with
  that node's config panel already open.
- **US-14.6** *(refines FR-12.7 error states.)* As any user, I want a
  **404 page that suggests plausible destinations** (nearest
  resource, top-level pages I have access to) instead of a dead
  end.
- **US-14.7** *(refines FR-06 Recent queries.)* As an analyst, I want
  the Explore Recent-queries entries to be **URL-addressable**
  (`/explore?recent=<id>`) so re-opening a query survives a browser
  restart.
- **US-14.8** *(refines FR-08 schedule modal.)* As an editor, I want
  the Schedule modal on the workflow builder to be **URL-addressable**
  (`/workflows/{id}?modal=schedule`) so refreshing the page reopens
  the modal rather than losing the context.
- **US-14.9** *(refines FR-12.10 responsive.)* As a keyboard-first
  user, I want the left nav to be **collapsible** with a shortcut
  (`Ctrl/Cmd+B`) to give my content area more width without hiding
  the nav permanently.
- **US-14.10** *(refines FR-12.4 permission-hidden nav.)* As an
  admin, I want a **"Restricted pages"** tooltip on the nav footer
  showing the count of pages hidden from my current session because
  of role, so I don't wonder whether the platform has more.

## User stories — **new features (10)**

- **US-14.11** As a power user, I want a **global command palette**
  (`Ctrl/Cmd+K`) that searches across connections, workflows, runs,
  users, and audit entries and jumps to the chosen result.
- **US-14.12** As any user, I want **breadcrumbs at the top of every
  detail page** (`Workflows › Daily orders rollup › Run #1247`) so I
  can jump up one level in a click.
- **US-14.13** As an ops engineer, I want a **cross-resource "Jump
  to"** menu on every detail page (from a connection: "3 workflows
  using this", "12 recent runs", "8 audit entries").
- **US-14.14** As a power user, I want **tabbed workspaces** — open
  multiple resources (workflow A, connection B, audit entry C) as
  tabs inside the app shell so I can flip between them without
  losing state.
- **US-14.15** As any user, I want a **Recents drawer** listing the
  last 20 resources I touched (any kind), reachable from the top bar
  and from `Ctrl/Cmd+R`.
- **US-14.16** As an admin, I want a **Saved views** feature per
  list page — save the current filter/sort/search state under a
  name and reload it later.
- **US-14.17** As an analyst, I want the **URL to reflect scroll
  position** in long list pages (or at least the top-of-page id)
  so returning via back re-anchors correctly.
- **US-14.18** As any user, I want **navigation history to survive
  drawer/modal state** — closing a modal opened by URL SHALL
  navigate back one step rather than remove the modal in place.
- **US-14.19** As a keyboard-first user, I want **inline vim-style
  navigation** on lists (`j` / `k` move selection, `Enter`
  opens) so I never need the mouse for triage tasks.
- **US-14.20** As an admin, I want the ability to **pin favorite
  resources** to the top of the left nav (up to 10 favorites,
  drag to reorder).

## Functional requirements

### FR-14.1 — Canonical URL shape

Every navigable state SHALL be encoded in the URL:

- List pages: `/connections`, `/workflows`, `/audit`, etc.
- Filter/sort/search: query-string parameters with stable, snake-
  case keys (`type`, `status`, `sort`, `search`, `from`, `to`).
- Detail pages: `/connections/{id}`, `/workflows/{id}`,
  `/workflows/{id}/runs/{run_id}`, `/audit/{id}`.
- Sub-state (open modal, selected node, active tab): query
  parameters (`?modal=schedule`, `?node=filter-1`, `?tab=history`).

Every URL parameter SHALL be documented in the frontend's routes
module and covered by a routing test.

### FR-14.2 — Breadcrumbs

Every detail page SHALL render a breadcrumb chain at the top of
the content area. Each hop SHALL be a link (except the current
page) and SHALL show the resource's snapshot name (falling back to
id if the resource is deleted).

### FR-14.3 — Command palette

The command palette SHALL:

- Open with **`Ctrl/Cmd+K`** from anywhere in the app.
- Search across **Connections**, **Workflows**, **Workflow runs**,
  **Users**, **Roles**, **Audit entries**, and **Pages**.
- Show the top 10 matches ranked by (recency ✕ text score).
- Support "action" entries (`Create connection`, `Run this
  workflow`, `Open audit log`) filtered by user permissions.
- Support keyboard-only operation (`↑` / `↓` / `Enter` / `Esc`).

The palette SHALL fetch results client-side from a lightweight
`/api/v1/search` endpoint that returns at most 200 results across
all resource types, gated by RBAC.

### FR-14.4 — Cross-resource jump menu

Every detail page SHALL surface a **"Related"** popover in the top
bar with counts and links to related resources:

- From a **connection**: workflows using this connection, recent
  Explore runs against it, audit events referencing it.
- From a **workflow**: recent runs, the schedule (if any), the
  connections it references, audit events referencing it.
- From an **audit entry**: the resource it references (link), the
  actor's other actions, the request-id timeline.
- From a **run**: the workflow it belongs to, the schedule (if
  applicable), the connection nodes touched, downstream audit
  events.

Each link SHALL respect RBAC: if the user lacks permission on the
target resource, the link SHALL be omitted (not shown as
disabled).

### FR-14.5 — Tabbed workspaces

The app shell SHALL support up to **10 open tabs**. Each tab is
an in-app resource pinned to the top bar. Tabs:

- persist across full-page reloads (stored in `localStorage`);
- have a close affordance;
- support middle-click / `Ctrl+W` to close;
- can be reordered by drag;
- can be opened via `Ctrl/Cmd+Click` on a link (from any list,
  breadcrumb, or Related menu).

Tabs do NOT survive logout.

### FR-14.6 — Recents drawer

The top bar SHALL expose a **Recents** button opening a drawer
with the last 20 resources the user touched (across kinds),
grouped by kind. Recents are stored per-user server-side so they
survive device changes.

### FR-14.7 — Saved views

Each list page (Connections, Workflows, Audit, Users, Runs) SHALL
support saving the current URL query state as a **named view**.
Saved views:

- appear in a top-of-list menu;
- are scoped per user (private by default);
- MAY be marked "team-shared" by users with the appropriate
  permission (`views:share`);
- store only URL parameters (no server-side computation).

### FR-14.8 — Browser history semantics

The application SHALL use HTML5 history (`pushState` /
`replaceState`) consistently:

- Navigating a link → `pushState` (adds to history).
- Changing a filter / sort → `replaceState` (does not clutter
  history).
- Opening a modal that has a distinct meaning (e.g. schedule
  modal) → `pushState` (so Back closes the modal).
- Opening a transient popover → does not touch history.

### FR-14.9 — Vim-style list navigation

List pages SHALL respond to keyboard keys when a row is focused:

- `j` / `↓` → move selection down.
- `k` / `↑` → move selection up.
- `Enter` → open the selected row.
- `g g` → jump to top; `G` → jump to bottom.
- `/` → focus the search input.

Shortcuts SHALL NOT hijack keys when a text input is focused.

### FR-14.10 — Pinned favorites

Users SHALL be able to pin up to **10 resources** as favorites
that appear at the top of the left nav. Pins are stored per-user
server-side. Pinning a resource the user later loses permission
on SHALL hide it from the nav automatically.

### FR-14.11 — 404 behaviour

Unmatched routes SHALL render a **helpful 404** page with:

- A short "This page doesn't exist" message.
- The best-guess intent (e.g. "did you mean this workflow?").
- Links to top-level pages the user has access to.
- A search box focused by default so the user can look up what
  they were after.

### FR-14.12 — Skip-to-content link

Every page SHALL expose a keyboard-focusable **"Skip to
content"** link as the first tab stop, jumping past the nav
directly to the main content region.

### FR-14.13 — Route-change announcement

Route changes SHALL update `document.title` and announce the new
page title via an `aria-live="polite"` region so screen-reader
users hear each transition.

### FR-14.14 — Deep-link permission gate

If a URL references a resource the user cannot see:

- **404** if the resource exists but the user lacks read
  permission (deliberately indistinguishable from "not
  found" — see FR-05 taxonomy).
- **404** if the resource genuinely does not exist.

The 404 page hides the distinction to prevent id enumeration.

### FR-14.15 — Left-nav collapse

The left nav SHALL be collapsible via a chevron in its header or
`Ctrl/Cmd+B`. The collapsed state is remembered per user in
`localStorage`. When collapsed, only icons show; hover reveals a
tooltip with the item name.

## UI/UX requirements

- The command palette uses the standard `Modal` primitive but is
  positioned near-top of viewport (offset ~20%) so it doesn't
  cover the main content area completely.
- Breadcrumb links use the `--text-secondary` token; the current
  page uses `--text-primary` and is not a link.
- Saved-view chips use the `Badge` primitive with a "pin" icon.
- The Recents drawer uses the `Drawer` primitive with grouped
  sections (Connections / Workflows / Runs / Audit / Users).
- The Related popover uses the `Popover` primitive anchored to the
  Related button in the top bar.
- Tab strip sits under the top bar; each tab is a `Chip` with the
  resource type icon and truncated name (truncated names have a
  tooltip showing the full name).
- Vim-style key hints are shown at the bottom of list pages when
  the list has focus.

## Acceptance criteria

- [ ] Copying the URL from the Connections page after applying
  `type=postgres&status=unhealthy` and pasting into a new tab
  restores the identical filtered view.
- [ ] Pressing `Ctrl/Cmd+K` on any page opens the command palette
  within 100ms.
- [ ] Typing "orders" in the palette returns matches from
  Connections, Workflows, and Runs whose names contain "orders",
  ranked by recency.
- [ ] Opening the Schedule modal from a URL parameter and pressing
  browser Back closes the modal (does not navigate away).
- [ ] The 404 page for a non-existent workflow shows a search box
  focused by default.
- [ ] Fetching `/workflows/<id-of-workflow-i-cannot-see>` returns a
  404 page indistinguishable from an actually-missing workflow.
- [ ] The Related popover on a connection detail page correctly
  shows the count of workflows using that connection.
- [ ] Middle-clicking a workflow row from the list opens the
  workflow as a new tab in the app shell, not a new browser tab.
- [ ] The Recents drawer contains resources touched in the last
  session and persists across full page reload.
- [ ] Saving a Connections filter view under a name, reloading the
  page, and choosing the saved view restores exactly the same URL.
- [ ] `j` / `k` on the Audit list moves the selection; `Enter`
  opens the entry; typing in the search box does not trigger the
  shortcuts.
- [ ] The "Skip to content" link is the first tab stop on every
  page.
- [ ] Route changes announce the new page title to screen readers.
- [ ] Pinning a workflow shows it at the top of the left nav
  across sessions.

## Edge cases & error handling

- **URL parameter that no longer exists**: e.g. a saved view
  references a filter option that has been removed. The page
  applies the remaining valid parameters and shows a subtle banner
  noting the drop.
- **Tab pointing to a deleted resource**: The tab becomes an
  error state ("This resource was deleted") with a close-tab
  affordance; opening it doesn't error the app shell.
- **Recents entry for a deleted resource**: Marked "(deleted)"
  and shown with reduced contrast; clicking opens the 404 page.
- **Command palette on slow network**: Renders immediately with
  local action entries; server results stream in when the
  `/api/v1/search` response arrives, with a loading skeleton in
  the results list.
- **Concurrent nav across tabs (browser tabs)**: A change in one
  browser tab does not force navigation in another; Recents
  reconcile via websocket or on next page load.
- **Very long resource names in breadcrumbs**: Names are
  truncated with an ellipsis and full name shown in a title
  attribute.
- **Deep-linked filter with invalid values**: Invalid values are
  silently discarded; a toast notes the drop.
- **Collapsed left nav on narrow viewports**: Nav auto-collapses
  below 1280px; user preference to expand overrides.
- **Screen reader with modal open**: Focus is trapped; Escape
  returns focus to invoking element; live region announces
  modal open/close.
- **Command palette with zero results**: Show "No matches — try
  a broader search" and offer common action entries.

## Non-functional requirements

- **Palette latency**: Client-side action search SHALL respond in
  ≤ 50ms at p95; server-side results SHALL respond in ≤ 250ms at
  p95.
- **Tabs memory**: 10 open tabs SHALL add ≤ 150 MB to the browser
  process's memory footprint at p95.
- **Recents storage**: Server-side recents SHALL be capped at 200
  entries per user with LRU eviction.
- **Saved views storage**: Capped at 25 saved views per user per
  list page.
- **Deep-link consistency**: The URL SHALL always be the source
  of truth for filter/sort/tab state; a state-mutation without a
  URL change SHALL be considered a bug.
- **Accessibility**: The command palette, tab strip, Recents
  drawer, and 404 page SHALL each pass an automated `axe` scan
  with zero critical issues.
- **Analytics**: Route changes SHALL emit a client-side
  navigation event with the request-id so backend and frontend
  telemetry correlate.
- **Backward compatibility**: URL schemes SHALL be versioned; a
  future URL change SHALL preserve old URLs via redirects for at
  least one major version.

## Market context & differentiation

| Product           | Navigation surface                                                          | Notes                                                                    |
| ----------------- | --------------------------------------------------------------------------- | ------------------------------------------------------------------------ |
| **Linear**        | Command palette (`Ctrl/Cmd+K`), tabbed workspaces, deep URLs                | Gold standard for navigability in a SaaS app.                             |
| **GitHub**        | Global search, breadcrumbs, deep URLs                                        | Excellent URL scheme; command palette is enterprise-tier.                 |
| **VS Code**       | Command palette (`Ctrl/Cmd+P` file, `Ctrl/Cmd+Shift+P` cmd), tabs, favs      | Product-defining palette + tab UX.                                        |
| **Slack**         | `Ctrl/Cmd+K` quick switcher, threads-as-tabs                                 | Excellent switcher; less URL-addressable.                                 |
| **Notion**        | `Ctrl/Cmd+P` navigation, breadcrumbs                                        | Good deep-linking; less resource-graph awareness.                         |
| **Retool**        | Left nav + editor tabs                                                       | Editor tabs are strong; palette is limited.                               |
| **Metabase**      | Search bar, breadcrumbs                                                      | Modest scope.                                                             |
| **Grafana**       | Search + dashboard tabs                                                      | Command palette added recently; good but chart-centric.                  |
| **n8n**           | Left nav, workflow tabs (browser tabs)                                       | Less URL-addressable state.                                              |

Data Explorer's differentiators for navigation & routing:

- **URL-addressable everything.** Filter, sort, search, modal,
  selected-node, tab — all in the URL.
- **Palette that respects RBAC.** No dead ends: only actions and
  resources the user can act on appear.
- **Related popover on every resource.** The resource graph
  (connections ↔ workflows ↔ runs ↔ audit) is a first-class
  navigational surface, not a search problem.
- **Tabbed workspaces in the app shell.** Compare two workflows,
  or a workflow + its audit trail, without losing state.
- **Vim-style shortcuts on lists.** Keyboard triage without a
  mouse.
- **Saved views + Recents.** Frequent work is one click away.
- **RBAC-safe 404s.** Deep-linking without id-enumeration risk.
- **Announced route changes.** Accessibility of navigation, not
  just of controls.

## Future enhancements (out of scope)

- Full-text search across every field of every resource (uses a
  dedicated search index).
- Workspaces / boards that combine multiple resources into a
  named collection with layout.
- Custom keyboard shortcut mapping per user.
- Shared saved views across an org with reader/editor
  permissions.
- "Follow" a resource for change notifications.
- Predictive command palette (learned from personal usage).
- Tab groups within a browser session.
- URL versioning with automatic migration on breaking changes.

## Cross-references

- [FR-02 Role-Based Access Control](./FR-02-role-based-access-control.md) — palette and jump menus respect RBAC.
- [FR-05 Connection Health Monitoring](./FR-05-connection-health-monitoring.md) — 404 vs `not_found` classification.
- [FR-06 Ad-Hoc Data Exploration](./FR-06-ad-hoc-exploration.md) — Recent-queries deep links.
- [FR-07 Visual Workflow Builder](./FR-07-visual-workflow-builder.md) — selected-node URL parameter.
- [FR-08 Workflow Scheduling & Automation](./FR-08-workflow-scheduling.md) — schedule modal URL parameter.
- [FR-10 Audit Logging & Compliance](./FR-10-audit-logging.md) — filter-state URL parameters.
- [FR-12 UI, Theming & Accessibility](./FR-12-user-interface-and-accessibility.md) — the shell this FRD extends.
