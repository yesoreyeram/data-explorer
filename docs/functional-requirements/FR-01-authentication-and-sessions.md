# FR-01: Authentication & Session Management

## Overview

Data Explorer requires every user of the web application and the JSON API
to prove who they are before performing any authorized action. This
functional requirement defines the sign-up, sign-in, session-refresh,
and sign-out journeys — including the security-critical properties that
make those journeys safe (password strength, brute-force resistance,
token lifetime, cookie scope) without making the user experience feel
onerous.

## Product goals

- **Frictionless first login.** A new operator should be able to reach a
  usable, permission-scoped workspace within seconds of running the
  server for the first time — no side-channel setup, no email
  verification wall for a self-hosted installation.
- **Safe by default for shared accounts.** Because Data Explorer is
  frequently deployed inside a company that shares a single tenant
  across many analysts and engineers, sessions must resist common
  attacks (credential stuffing, XSS token theft, session fixation)
  without asking the user to think about any of it.
- **Predictable session lifetime.** Users should not be surprised by
  sudden mid-work sign-outs, but a session that has been idle for a
  long time or that has been used from a new device should ultimately
  end.
- **Least-privilege by default.** A brand new account should never gain
  more access than "look around". Elevating that account is an explicit,
  audit-logged administrative action.

## User personas

| Persona                | Description                                                                                                 |
| ---------------------- | ----------------------------------------------------------------------------------------------------------- |
| First-time user        | Signs up through the UI; expects a low-friction path from `/register` to a working workspace.               |
| Returning user         | Signs in daily; expects to stay signed in across tabs and page refreshes but not indefinitely on a shared machine. |
| Admin                  | Bootstraps the deployment, promotes another account, or suspends a compromised account.                    |
| Attacker (adversary)   | Attempts credential stuffing, XSS-based token theft, or replay of a leaked refresh token.                   |

## User stories

- **US-01.1** As a first-time user, I want to create an account with an
  email and password so that I can start exploring data without waiting
  for an invitation.
- **US-01.2** As a returning user, I want to sign in once per session and
  remain signed in as I move between pages so that my flow is not
  interrupted.
- **US-01.3** As a returning user, I want to sign out explicitly and be
  sure the session is unusable afterwards, even from another tab.
- **US-01.4** As an admin, I want new self-registered accounts to have
  the least possible privileges so that a compromised registration
  endpoint cannot grant broad access.
- **US-01.5** As an admin, I want to suspend a specific user so that a
  departing employee or a compromised account can be locked out
  immediately without changing anyone else's password.
- **US-01.6** As an admin, I want failed logins to be indistinguishable
  from "user does not exist" so that the API cannot be used to enumerate
  which emails are registered.
- **US-01.7** As a returning user, I want the sign-in page to keep track
  of where I was trying to go, so that after signing in I land on that
  page instead of the dashboard.

## Functional requirements

### FR-01.1 — Account registration

The system SHALL provide a `POST /api/v1/auth/register` endpoint and a
matching `/register` page that create a new local account from an email,
a display name, and a password.

- The email SHALL be treated as case-insensitive and unique across the
  system.
- The password SHALL be at least **12 characters** long; shorter values
  SHALL be rejected client-side and server-side.
- The password SHALL be hashed with a memory-hard, tunable password
  hashing algorithm (Argon2id) before being persisted.
- The plaintext password SHALL NEVER be logged, echoed back in
  responses, or written anywhere on disk.
- The new account SHALL be granted the **`viewer`** role only (least
  privilege by default). Any elevation SHALL require an admin action
  (see [FR-02](./FR-02-role-based-access-control.md)).
- Successful registration SHALL immediately sign the user in — i.e.
  return the same access + refresh token pair as `login` — so the user
  does not have to type their password a second time.

### FR-01.2 — Sign in

The system SHALL provide a `POST /api/v1/auth/login` endpoint and a
matching `/login` page that authenticate an email + password.

- Both the "user does not exist" and "password is wrong" cases SHALL
  return the **same generic error message** with the same HTTP status.
- Both cases SHALL take approximately the same wall-clock time to
  respond (timing-uniform), so an attacker cannot distinguish them by
  latency.
- A successful sign-in SHALL issue:
  1. A short-lived **access token** (JWT, ~15 minutes) delivered in the
     response body.
  2. A long-lived **refresh token** (opaque high-entropy random string)
     set as an `httpOnly`, `SameSite=Strict`, `Secure` cookie scoped to
     `/api/v1/auth`.
- Only the SHA-256 hash of the refresh token SHALL be stored server-side.
- The response SHALL include the caller's user profile (id, email,
  display name, roles) and their flattened permission set so the SPA
  can render permission-gated UI without a second round-trip.

### FR-01.3 — Session refresh

The system SHALL provide a `POST /api/v1/auth/refresh` endpoint that
takes a valid refresh token cookie and returns a new access token.

- Each refresh SHALL **rotate** the refresh token: the old token is
  revoked and a new one is issued. Replaying an already-used refresh
  token SHALL fail and SHALL mark the entire user's session tree as
  compromised (best-effort revocation of sibling refresh tokens).
- The SPA SHALL transparently refresh the access token when a
  request fails with `401 token_expired`, retrying the original
  request once so the user never sees the interruption.
- Refresh SHALL fail if the refresh token has been revoked, has
  expired, or belongs to a suspended user.

### FR-01.4 — Sign out

The system SHALL provide a `POST /api/v1/auth/logout` endpoint that:

- Revokes the caller's current refresh token immediately, in the
  database.
- Clears the refresh cookie in the response.
- Returns a 204 regardless of whether the caller was authenticated
  (calling logout twice is not an error).

The SPA SHALL clear any in-memory copy of the access token and redirect
to `/login` after a successful logout.

### FR-01.5 — Current-user endpoint

The system SHALL expose `GET /api/v1/auth/me` that returns the caller's
profile (id, email, display name, roles, permissions). The SPA uses this
at startup to rehydrate the session from an existing refresh cookie
without requiring a fresh sign-in.

### FR-01.6 — Suspended accounts

An account with `status = 'suspended'` SHALL NOT be able to sign in,
refresh, or use any existing access token. The behavior at each layer:

- `POST /auth/login`: SHALL return the same generic "invalid
  credentials" error as any other failed login (does not disclose that
  the account exists but is suspended).
- `POST /auth/refresh`: SHALL fail with `401` and revoke the refresh
  token being presented.
- Access token still within its 15-minute TTL: SHALL be rejected by
  `Authenticate` middleware on the next request against a
  permission-gated route because the middleware SHALL re-check user
  status when performing authorization (defence-in-depth).

### FR-01.7 — Rate limiting on auth endpoints

`POST /auth/login`, `POST /auth/register`, and `POST /auth/refresh`
SHALL be rate-limited per source IP more aggressively than the general
API rate limit to blunt credential stuffing (see also
[FR-11](./FR-11-observability-and-guardrails.md)).

### FR-01.8 — Audit trail

Every sign-in (successful and failed), every registration, every
refresh, every logout, and every user-status change SHALL emit an
`audit_logs` row with the actor, action, resource, outcome, source IP,
and user agent. See [FR-10](./FR-10-audit-logging.md).

## UI/UX requirements

### Sign-in page (`/login`)

Reference screenshot: [`docs/screenshots/01-login.png`](../screenshots/01-login.png).

- Single centered card with the product name, a short tagline, an email
  field (`type="email"`, autocomplete `username`), a password field
  (`type="password"`, autocomplete `current-password`), and a primary
  **Sign in** button.
- Below the form: a link to `/register` labelled **"No account? Create
  one"**.
- Client-side: both fields are `required`. The Sign in button is
  disabled while a request is in-flight and shows a **"Signing in…"**
  label.
- When a user was redirected here from a protected route, the router
  SHALL remember that origin (`location.state.from`) and navigate back
  to it after successful sign-in. Direct visitors SHALL land on the
  dashboard `/`.
- Failures SHALL display a single inline red error banner with the
  generic message (never a specific "no such user" / "wrong password").
- The form SHALL work fully with keyboard-only input (Tab through
  fields, Enter to submit).

### Register page (`/register`)

- Same visual shell as the sign-in page.
- Fields: **Full name** (required, `autocomplete="name"`), **Email**
  (required), **Password** (required, `minLength=12` with a
  hint text "**At least 12 characters.**").
- A **note** near the top of the form clarifies: **"New accounts start
  with read-only viewer access."**
- Primary button label toggles **Create account** / **Creating
  account…**.
- A link to `/login` at the bottom labelled **"Already have an account?
  Sign in"**.

### Session lifetime UX

- Session refresh SHALL be silent. The SPA SHALL NOT display a modal or
  a full-page reload for a routine refresh.
- If a refresh ultimately fails (token revoked, network unreachable, or
  the account has been suspended), the SPA SHALL redirect to `/login`
  with a small banner "**Your session has expired. Please sign in
  again.**" and preserve the intended destination URL.

## Acceptance criteria

- [ ] A user can register with a 12+ character password and land in the
  dashboard immediately, with the `viewer` role assigned.
- [ ] A user cannot register with a password shorter than 12 characters
  (client-side validation + a server-side 400 with a clear message).
- [ ] Registering a duplicate email fails with a **generic** validation
  error that does not disclose whether the target email existed already.
- [ ] Signing in with a wrong password and signing in with an unknown
  email produce the **same** error message and the **same** HTTP status
  code, within a small timing tolerance.
- [ ] Signing in successfully sets a refresh cookie with `HttpOnly`,
  `Secure`, `SameSite=Strict`, and a `Path` restricted to `/api/v1/auth`.
- [ ] The stored refresh token in the database is the SHA-256 hash of
  the raw cookie value, never the raw value.
- [ ] After ~15 minutes, an access token expires; the SPA silently
  refreshes and the user does not notice.
- [ ] Signing out revokes the refresh token in the database and clears
  the cookie; the same refresh cookie replayed after logout SHALL fail.
- [ ] Using an already-used refresh token SHALL fail and SHALL revoke
  all sibling refresh tokens for that user.
- [ ] Suspending a user prevents further sign-in and invalidates any
  currently-active refresh token.
- [ ] Every one of these actions appears in `audit_logs` with an
  outcome (`success`/`failure`) and an actor (or `""` for anonymous
  requests).
- [ ] More than 10 login attempts per IP per short window are rate
  limited with `429 rate_limited`.

## Edge cases & error handling

- **Clock skew between browser and server.** JWT expiry checks use the
  server's clock; the SPA does not rely on decoding the JWT for expiry.
- **Refresh cookie set from HTTP (dev) vs HTTPS (prod).** In production
  the `Secure` flag is mandatory. In development against a plain HTTP
  loopback the flag is disabled — this is a deliberate developer
  ergonomics affordance, documented in `docs/DEVELOPER_GUIDE.md`.
- **Multi-tab sign-out.** If a user signs out in one tab, other tabs
  SHOULD detect the missing session on their next network call and
  redirect to `/login`. (Implementation: the shared `authStore` clears
  its in-memory user on 401.)
- **Password reset.** Out of scope for this FR; see "Future
  enhancements".
- **Two-factor authentication.** Out of scope for this FR; see "Future
  enhancements".
- **Browser without cookies.** If a browser or an API client refuses to
  store the refresh cookie, `refresh` will keep failing. The SPA SHALL
  detect this consistently-failing pattern and surface a clear message
  ("**Cookies must be enabled to stay signed in**").

## Non-functional requirements

- **Cryptographic strength.** Passwords hashed with Argon2id at parameters
  that take ≥250 ms on the target hardware. Refresh tokens have ≥256 bits
  of entropy.
- **Timing safety.** Login response time variance between "unknown
  user" and "wrong password" SHALL be < 20 ms at the 99th percentile.
- **Token secrets.** The JWT signing key is loaded from the
  `JWT_SIGNING_KEY` environment variable; the config validator SHALL
  refuse to start in `production` mode without one set.
- **Session hijacking window.** A stolen access token expires within 15
  minutes; a stolen refresh token is invalidated on next legitimate use
  (rotation).

## Market context & differentiation

| Product                                | Auth model                                                                          | Differences vs Data Explorer                                                                                     |
| -------------------------------------- | ----------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------- |
| Retool self-hosted                     | Local + SSO/SAML/OIDC, MFA optional                                                 | Retool has full SSO. Data Explorer today ships local email + password only, deliberately (see Future).           |
| Grafana OSS                            | Local + LDAP/OAuth/OIDC/SAML, anonymous access mode                                 | Grafana supports "public dashboards". Data Explorer requires authentication for every route (no anonymous mode). |
| Metabase Community                     | Local + LDAP/SAML (paid), Google OAuth                                              | Similar local-first posture; Data Explorer is stricter on password length (12 vs 8).                             |
| n8n                                    | Local + LDAP/SAML (paid tiers)                                                      | Similar local baseline.                                                                                          |
| Postman                                | Cloud identity only (SSO)                                                           | Data Explorer is self-hosted and offline-friendly.                                                              |

**Where Data Explorer is intentionally different.** Rather than emulate
the full "identity provider" surface of Retool or Grafana, this product
optimizes for a single security-conscious default that works well
without any external identity infrastructure: Argon2id + rotating
refresh tokens + short-lived JWT, timing-uniform login, and
`SameSite=Strict` cookies scoped to `/api/v1/auth`. That baseline is
higher than the OSS defaults of most comparable tools and is enforced
uniformly rather than left to operator configuration.

## Future enhancements (out of scope for this FR)

- **SSO / OIDC / SAML.** Federated identity is on the roadmap.
  ([`docs/SECURITY.md`](../SECURITY.md#known-limitations--not-yet-implemented)
  documents this as a known limitation.)
- **Two-factor authentication.** Time-based OTP as a second factor for
  admin accounts.
- **Password reset via email.** Requires an SMTP integration and an
  email-verified account flow; deliberately deferred because self-hosted
  installations without SMTP are a first-class use case.
- **Just-in-time provisioning from SSO claims.** Auto-map SSO groups
  to Data Explorer roles.
- **Session revocation UI.** An admin page listing all a user's active
  sessions with a per-session "sign out this device" action.

## Cross-references

- Implementation: `backend/internal/auth/`, `backend/internal/api/handlers/auth.go`,
  `frontend/src/pages/LoginPage.tsx`, `frontend/src/pages/RegisterPage.tsx`,
  `frontend/src/state/authStore.ts`.
- Security posture: [`../SECURITY.md`](../SECURITY.md) sections
  "Identity and sessions" and "Transport and HTTP hardening".
- RBAC integration: [FR-02](./FR-02-role-based-access-control.md).
- Audit integration: [FR-10](./FR-10-audit-logging.md).
