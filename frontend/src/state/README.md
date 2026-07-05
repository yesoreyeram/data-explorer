# src/state

## What this package does

`src/state` contains **Zustand stores for client-side UI state**. Server data (connections, workflows, audit logs, etc.) is managed by TanStack Query (`useQuery`/`useMutation`); Zustand is used only for state that is purely client-side and needs to be shared across components.

## Stores

### authStore (`authStore.ts`)

The authenticated session.

| State | Description |
|---|---|
| `accessToken` | Current JWT access token (in memory, never localStorage) |
| `user` | Decoded user object: `{ id, email, displayName, roles, permissions }` |
| `isAuthenticated` | Derived: `accessToken !== null` |

Key actions: `setSession(token, user)`, `clearSession()`, `hasPermission(code)`.

The access token lives only in memory (never in `localStorage`) to limit XSS exposure. The refresh token is in an `httpOnly` cookie managed by the browser.

### themeStore (`themeStore.ts`)

Light / dark / system theme selection.

| State | Description |
|---|---|
| `theme` | `"light"` \| `"dark"` \| `"system"` |

- Persisted to `localStorage` via Zustand's `persist` middleware.
- The active resolved theme is applied by toggling `data-theme` on `<html>`; no per-component branching.

### sidebarStore (`sidebarStore.ts`)

Sidebar collapsed/expanded state.

| State | Description |
|---|---|
| `collapsed` | `boolean` |

- Persisted to `localStorage`.
- Consumed by the layout component to set sidebar width CSS variable.

### navigationStore (`navigationStore.ts`)

Command palette and recent-activity state.

| State | Description |
|---|---|
| `favorites` | Pinned nav items |
| `recentRoutes` | Last N visited routes (for "recent" section in command palette) |
| `commandPaletteOpen` | Whether the command palette (`⌘K`) is open |

- Partially persisted to `localStorage` (favorites, recent routes).

### savedChartsStore (`savedChartsStore.ts`)

Client-side saved chart snapshots for the dashboard.

| State | Description |
|---|---|
| `charts` | Array of `{ id, title, queryResult, chartConfig }` |

- Persisted to `localStorage`.
- Backs the dashboard's saved-chart gallery without any backend storage.

## Architecture decisions (ADRs)

| Decision | Rationale |
|---|---|
| Zustand for UI state, TanStack Query for server state | Clear separation: Zustand is for local UI; TanStack Query owns cache, loading states, background refresh |
| Access token in memory only | Not `localStorage` — an XSS attack cannot exfiltrate it; the `httpOnly` refresh cookie handles session persistence |
| `localStorage` for preferences (theme, sidebar) | Safe for non-sensitive UI preferences; survives page reload |
| Saved charts in `localStorage` | No backend storage needed for a personal, per-browser dashboard; acceptable trade-off for simplicity |

## Scope and responsibilities

- Hold and expose client-side UI state.
- Persist appropriate state to `localStorage`.
- Provide typed actions for state mutations.
- Never hold server data (use TanStack Query for that).

## Limitations and todos

- [ ] `savedChartsStore` is per-browser; charts saved on one device are not visible on another.
- [ ] `navigationStore.recentRoutes` are not filtered by permission — a route visited while having `admin` role may still appear in recents after a role downgrade.
- [ ] `authStore` has no token expiry check; the interceptor in `src/api/client.ts` handles silent refresh, but a proactive "token will expire soon" UX is absent.
- [ ] No cross-tab synchronization for `authStore`; logging out in one tab doesn't immediately affect other tabs.
