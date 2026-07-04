# FR-02: Role-Based Access Control (RBAC)

## Overview

Data Explorer connects to systems containing real credentials and real
data, so every mutating action and every sensitive read is gated by a
fine-grained permission. This functional requirement defines the role
and permission model, how permissions are enforced across API and UI,
how administrators manage roles, and what safeguards prevent
misconfiguration.

## Product goals

- **Least privilege by construction.** Every route, every button, and
  every side effect is gated by one permission code — there is no
  "if admin, skip check" bypass anywhere in the request lifecycle.
- **Defense in depth.** The frontend hides UI a user cannot use, but
  the API is the sole authoritative check. A user who bypasses the SPA
  gets no additional access.
- **Instant clarity for admins.** Assigning or revoking a role is a
  single UI click, propagates as soon as the user's next token is
  minted (≤ 15 minutes), and produces an audit-log entry.
- **Stable permission vocabulary.** Permission codes are treated as a
  small, deliberate contract between the backend and the frontend; new
  permissions are added only when a new capability genuinely requires
  one.

## User personas

| Persona                | Description                                                                                                                             |
| ---------------------- | --------------------------------------------------------------------------------------------------------------------------------------- |
| Admin                  | Creates and manages users, assigns roles, suspends users. Holds the `admin` role, which bundles every permission.                       |
| Editor                 | Creates and manages connections and workflows; can execute workflows. Cannot manage users or roles.                                     |
| Viewer                 | Read-only access to connections, workflows, and their executions. Cannot see the audit log.                                             |
| Security officer       | Read the audit log; may hold no other roles.                                                                                            |
| Custom role holder     | Any bundle of permissions built by an admin for a team-specific need (e.g. "SRE" = connections:read + connections:test + audit:read).   |

## User stories

- **US-02.1** As an admin, I want to see every user in the system with
  their assigned roles at a glance so that I can quickly audit "who has
  access to what".
- **US-02.2** As an admin, I want to change a user's roles in a single
  modal without editing individual permissions so that role assignment
  is fast and hard to get wrong.
- **US-02.3** As an admin, I want the three system roles (`admin`,
  `editor`, `viewer`) to be locked so that nobody — including another
  admin — can accidentally remove the `users:write` permission from
  `admin` and lock everyone out.
- **US-02.4** As an admin, I want a suspended user to lose access
  immediately, not "eventually", so that a compromised or departing
  account is safely contained.
- **US-02.5** As an editor, I want the UI to hide administrative
  buttons (delete a user, view audit log) that I cannot use so that I
  am not confused by dead-end interactions.
- **US-02.6** As a viewer, I want a clear, respectful message when I
  navigate to a page I cannot use, rather than a raw 403 or a blank
  screen.
- **US-02.7** As a security officer, I want every role change to be
  recorded in the audit log with who did it, so that I can review
  privilege grants after the fact.

## Functional requirements

### FR-02.1 — Permission vocabulary

The system SHALL define a fixed set of permission codes. Each code
follows the pattern `<resource>:<verb>` where `<verb>` is one of
`read`, `write`, `execute`, or a resource-specific action.

| Permission code       | Grants                                                                            |
| --------------------- | --------------------------------------------------------------------------------- |
| `users:read`          | View the list of users                                                            |
| `users:write`         | Activate/suspend users                                                            |
| `roles:read`          | View the list of roles                                                            |
| `roles:write`         | Assign roles to users, create custom roles                                        |
| `connections:read`    | View connections, run saved queries against them, browse the integration catalog |
| `connections:write`   | Create / edit / delete connections                                                |
| `connections:test`    | Run a health-check test; run a temporary (never-persisted) connection query      |
| `workflows:read`      | View workflows and execution history                                              |
| `workflows:write`     | Create / edit / delete / schedule workflows                                       |
| `workflows:execute`   | Run a workflow manually                                                           |
| `audit:read`          | View the audit log                                                                |

Adding a new permission code is a schema change (`db/migrations/*.sql`)
and is a deliberate, PR-reviewed act.

### FR-02.2 — System roles

The system SHALL ship three system roles seeded on first boot:

- **`admin`** — has every permission listed in FR-02.1.
- **`editor`** — has `connections:*`, `workflows:*`, and `roles:read`,
  `users:read` (for context in modals).
- **`viewer`** — has `connections:read`, `workflows:read`,
  `roles:read`, `users:read`.

System roles SHALL be marked with `is_system = true` in the database
and SHALL NOT be deletable through any UI or API. Their permission
bundle SHALL NOT be editable through the UI, though an operator can
alter the seed migration in a fork.

### FR-02.3 — Custom roles

The system MAY support additional roles seeded through migrations or
(in a future release) created via the UI. Custom roles combine any
subset of the FR-02.1 permission codes.

### FR-02.4 — Route-level enforcement

Every non-public API route SHALL be gated by **exactly one** permission
code at the router level (chi middleware `RequirePermission`). There
SHALL be no "admin bypass" branch anywhere in the handler layer — the
check is uniform.

One documented exception exists: `POST /api/v1/explore/query`
additionally requires `connections:test` in the handler itself, but
only when the request body supplies a temporary (never-persisted)
connection instead of an existing connection ID. See
[FR-06](./FR-06-ad-hoc-exploration.md) for why.

### FR-02.5 — Frontend permission gating

The SPA SHALL:

- Hide sidebar links to pages the current user cannot access.
- Wrap every mutating button (Create/Edit/Delete/Suspend/…) in a
  `<PermissionGate permission="…">` so users cannot click into a
  guaranteed 403.
- Never rely on a hidden button as a security boundary; the API SHALL
  reject the request even if a determined user reconstructs it.

### FR-02.6 — Permission propagation and staleness

- The caller's flattened permission set SHALL be embedded in their JWT
  access token so that authorization does not require a database
  round-trip on every request.
- A role change SHALL take effect on the target user's **next** token
  mint (login or refresh). Access tokens are short-lived (15 minutes)
  specifically to bound this staleness window.
- The Users page SHALL make this trade-off visible where relevant, e.g.
  "Role changes take effect on the user's next sign-in or after their
  session refreshes (typically within 15 minutes)."

### FR-02.7 — User status: active vs suspended

- A user's status is either `active` or `suspended`.
- Only `users:write` holders may change a user's status.
- A user SHALL NOT be able to suspend their own account (defensive
  check in the UI and the handler). This prevents an admin from locking
  themselves out with a single misclick.
- Suspending a user immediately revokes all of their existing refresh
  tokens (see FR-01.6). Their currently-issued access tokens continue
  to work for up to 15 minutes as a documented trade-off; because every
  authorization check goes through a single middleware chokepoint, a
  future enhancement can add a "suspended user" fast-fail there.

### FR-02.8 — Users administration UI

The system SHALL provide a `/users` page (screenshot:
[`docs/screenshots/11-users-roles.png`](../screenshots/11-users-roles.png))
that lists every user with:

- Display name and email
- Roles as badges
- Status badge (`active` / `suspended`)
- Joined date
- Actions column: **Roles** button (edit assigned roles) and
  **Suspend** / **Reactivate** button.

The Roles button opens a modal titled **"Edit roles for <email>"** that
lists every role in the system with a checkbox and a short description,
plus **Cancel** / **Save** actions. Saving submits a single
`PUT /api/v1/users/:id/roles` request with the full desired role set
(idempotent replace, not per-role add/remove).

## UI/UX requirements

- The Users page SHALL be visible only to users holding `users:read`.
- The `Roles` action SHALL be visible only to users holding
  `roles:write`.
- The `Suspend` / `Reactivate` action SHALL be visible only to users
  holding `users:write` and SHALL be disabled on the caller's own row.
- Role checkboxes for system roles SHALL be labelled with a small
  "(system)" tag so administrators know deleting them is not offered.
- The save button in the Roles modal SHALL show "Saving…" while the
  request is in flight and re-enable after success or failure.
- A user without any of `users:read` / `users:write` / `roles:read` /
  `roles:write` SHALL NOT see the "Users" sidebar link at all.
- Attempting to navigate to `/users` without permission SHALL redirect
  to `/` — not show a broken page.

## Acceptance criteria

- [ ] Every non-public API route in `internal/api/router.go` is
  wrapped by exactly one `RequirePermission(...)` middleware call
  (verified by inspection and by a test that fails if a new route is
  added without one).
- [ ] The three system roles are present after the first boot on a
  fresh database, with the permission bundles defined in FR-02.2.
- [ ] `is_system = true` roles cannot be deleted via any API endpoint.
- [ ] A newly-registered user has exactly the `viewer` role assigned.
- [ ] Assigning a role to a user updates their permission set on their
  next login/refresh, verified by comparing the `permissions` array
  returned by `GET /auth/me` before and after.
- [ ] The Users page renders correctly with 0 users, 1 user, and many
  users, using standard `Loading…` and empty-state affordances (see
  [FR-12](./FR-12-user-interface-and-accessibility.md)).
- [ ] A user cannot suspend their own account, either from the UI or by
  crafting a direct API call.
- [ ] Every user-status change and every role assignment writes an
  `audit_logs` row.
- [ ] A user without `users:read` who navigates directly to `/users`
  is redirected without seeing any user data.
- [ ] An API call from a user without the required permission returns
  `403 forbidden` with a generic message and does not leak resource
  identifiers.

## Edge cases & error handling

- **Removing all roles from a user.** SHALL be allowed. That user
  retains a valid session but every subsequent API call except
  `/auth/me` will return `403`. The UI SHALL show them the
  dashboard with all cards empty and no sidebar entries besides the
  dashboard itself.
- **Removing the last admin.** SHALL be prevented at the API layer:
  the handler counts remaining users holding `roles:write` after the
  change and rejects the mutation if the count would drop to zero. The
  UI SHALL surface the resulting error as a clear message: **"There
  must be at least one administrator."**
- **Role name collisions.** Role names are unique (a database
  constraint). A future custom-role UI SHALL surface the resulting
  duplicate-name error.
- **A caller with a stale JWT.** If a role has been revoked from the
  caller but their JWT still lists it, the API is authoritative on the
  actual database state for anything that re-reads the database (e.g.
  `roles:write` cross-check). Otherwise the caller's permission set is
  in effect until refresh.
- **Concurrent role edits.** Two admins editing the same user's roles
  at the same time SHALL result in a last-writer-wins outcome; the API
  SHALL NOT produce a partial update.

## Non-functional requirements

- **Authorization performance.** Permission check on every request
  is a hash-set lookup on a permission code embedded in the JWT — no
  database round-trip per request.
- **Permission list stability.** The set of permission codes is versioned
  with the schema; renaming a permission requires a data migration to
  update all `role_permissions` rows.
- **Auditability.** Every role-affecting action MUST produce exactly
  one audit-log entry with actor, target user, before/after roles
  (in metadata), and outcome.

## Market context & differentiation

| Product                | RBAC model                                                                                              | Notes                                                                                                          |
| ---------------------- | ------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------- |
| Retool                 | Groups + resource-level permissions + audit                                                             | Retool exposes per-resource ACLs. Data Explorer starts with role-level, keeping the model simpler and easier to audit. |
| Grafana                | Basic roles + fine-grained (paid) + resource ACLs                                                       | Grafana OSS's coarser model is closer to Data Explorer's; the fine-grained model is paid.                     |
| Metabase               | Groups with per-collection and per-database permissions                                                 | Metabase mixes RBAC with resource ACLs. Data Explorer keeps them separate for now.                             |
| n8n                    | Roles: owner / admin / member / editor / viewer, plus SSO groups                                        | Similar to Data Explorer, without fine-grained per-permission composition.                                     |
| Superset               | Fine-grained (permission → role → user), Flask-AppBuilder-based                                         | Superset has arguably the most flexible model — and also the steepest learning curve. Data Explorer prefers a small fixed vocabulary. |

**Where Data Explorer is intentionally different.** The system deliberately
does *not* provide per-resource ACLs. Access to a connection is decided
by "does this user hold `connections:read`" — not "does this user have
`connections:read` **on this specific connection**." This is a
scope-simplicity trade-off, appropriate for organizations where the set
of connections is small and every editor is trusted with every source.
See "Future enhancements" for the natural extension point.

## Future enhancements (out of scope for this FR)

- **Custom-role builder UI.** A page under `/users` for creating,
  editing, and deleting non-system roles.
- **Resource-level ACLs.** Optional per-connection and per-workflow
  visibility ("only this team can see the production database
  connection").
- **SSO/OIDC group → role mapping.** When SSO ships
  ([FR-01](./FR-01-authentication-and-sessions.md) Future), map IdP
  groups to Data Explorer roles at login time.
- **Time-boxed role grants.** Grant a user the `editor` role for a
  named window ("for the next 24 hours") that automatically expires.
- **Approval workflows.** Require a second admin's approval before a
  role change takes effect on privileged users.

## Cross-references

- Implementation: `backend/internal/rbac/rbac.go`,
  `backend/internal/api/middleware/rbac.go`,
  `backend/internal/api/handlers/users.go`,
  `frontend/src/pages/UsersPage.tsx`,
  `frontend/src/components/PermissionGate.tsx`,
  `frontend/src/lib/permissions.ts`.
- Security posture: [`../SECURITY.md`](../SECURITY.md) section
  "Authorization (RBAC)".
- Auth integration: [FR-01](./FR-01-authentication-and-sessions.md).
- Audit integration: [FR-10](./FR-10-audit-logging.md).
