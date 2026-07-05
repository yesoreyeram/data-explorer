# internal/rbac

## What this package does

`internal/rbac` defines the **role-based access control model**: a `Principal` (the authenticated caller) carries a resolved, flattened set of permission codes, and callers check them with a single `Has` call.

## Core type: Principal

```go
type Principal struct {
    UserID      string
    Email       string
    Roles       []string
    Permissions map[string]struct{}  // O(1) lookup
}

func (p Principal) Has(permission string) bool
```

The `Principal` is created once at JWT verification time (middleware) and attached to the request context. Every downstream handler/service accesses it via `rbac.FromContext(ctx)`.

## Permission constants

| Constant | Code | Description |
|---|---|---|
| `PermUsersRead` | `users:read` | List and view user accounts |
| `PermUsersWrite` | `users:write` | Create, update, suspend users; manage roles |
| `PermRolesRead` | `roles:read` | List roles and their permissions |
| `PermRolesWrite` | `roles:write` | Create/update custom roles |
| `PermConnectionsRead` | `connections:read` | List and view connections |
| `PermConnectionsWrite` | `connections:write` | Create, update, delete connections |
| `PermConnectionsTest` | `connections:test` | Test connections and run ad-hoc queries with inline credentials |
| `PermWorkflowsRead` | `workflows:read` | List and view workflow definitions and execution history |
| `PermWorkflowsWrite` | `workflows:write` | Create, update, delete workflows; manage schedules |
| `PermWorkflowsExecute` | `workflows:execute` | Trigger workflow runs |
| `PermAuditRead` | `audit:read` | View audit log |

## Built-in roles (seeded by `db/migrations/0002_seed_rbac.sql`)

| Role | Permissions |
|---|---|
| `admin` | All permissions |
| `editor` | connections:read/write/test, workflows:read/write/execute, catalog browsing |
| `viewer` | connections:read, workflows:read |

## How permissions are enforced

1. **At login/refresh** — the user's roles are queried from the database, their permissions are unioned and flattened into a `[]string`, and embedded in the JWT payload.
2. **At middleware** — the JWT is parsed; `rbac.NewPrincipal(…)` builds a `Principal` with an `O(1)` map; it is attached to the request context.
3. **At the router** — every route is wrapped with `RequirePermission(code)` middleware in `internal/api/router.go`. No "if admin, skip the check" escape hatch exists.
4. **In handlers** — some handlers perform a second, conditional check (e.g., ad-hoc explore requiring `connections:test` only when the request body contains inline credentials).

## Architecture decisions (ADRs)

| Decision | Rationale |
|---|---|
| Permissions resolved at token issuance, not per-request | Eliminates a DB join on every authenticated request; 15-min JWT TTL bounds staleness |
| One permission code per route | Auditable: reading `router.go` tells you exactly what permission each route requires |
| `map[string]struct{}` (set) for permissions | O(1) `Has` lookup; no iteration needed |
| Context-based propagation | Passes through middleware layers without threading the Principal through every function signature |

## Scope and responsibilities

- Define all permission code constants.
- Define `Principal` and context helpers (`WithPrincipal`, `FromContext`).
- Nothing else — role/permission CRUD is in the `auth`/`users` service; enforcement is in `api/middleware` and `api/router`.

## Limitations and todos

- [ ] Role changes take effect on the next login or token refresh (up to 15-minute staleness window).
- [ ] No permission hierarchy or wildcard matching (e.g. `connections:*`); each code is a discrete string.
- [ ] Custom roles can be added via SQL but there is no UI for role management yet.
- [ ] No row-level security; permissions are resource-type-level, not per-row.
