# internal/auth

## What this package does

`internal/auth` handles **user registration, login, JWT access token issuance, refresh token rotation, OIDC (OpenID Connect) login, and password management**. It is the sole owner of the authentication flow; RBAC (what a user is allowed to do) is handled separately in `internal/rbac`.

## Sub-components

### Service (`service.go`)

The `Service` type is the public API for authentication:

| Method | Description |
|---|---|
| `Register(email, password, displayName)` | Creates a new `viewer`-role user with Argon2id-hashed password |
| `Login(email, password)` | Verifies credentials (timing-uniform on "not found" vs "wrong password"); issues JWT + refresh token |
| `Refresh(refreshToken)` | Validates the opaque refresh token, rotates it, issues new JWT + refresh token |
| `Logout(refreshToken)` | Revokes the refresh token |
| `ChangePassword(userID, old, new)` | Re-verifies old password before hashing and storing the new one |
| `OIDCCallback(provider, code, verifier)` | Exchanges the OIDC authorization code for an ID token; provisions a `viewer` user on first login |

### JWT (`jwt.go`)

- Short-lived HS256 tokens (15 minutes by default)
- Payload: `sub` (user ID), `email`, `roles[]`, `permissions[]` (flattened set)
- Permissions are resolved once at issuance — every subsequent API request is an O(1) in-memory set lookup with no database round trip

### Refresh tokens

- Opaque, high-entropy random values (`crypto/rand`)
- Only the **SHA-256 hash** is stored in `refresh_tokens` — a stolen database dump does not grant sessions
- Stored in an `httpOnly`, `SameSite=Strict` cookie scoped to `/api/v1/auth` — inaccessible to JavaScript
- **Rotated on every use** (old token revoked, new one issued in the same transaction)

### OIDC (`oidc.go`)

- Authorization Code + PKCE flow
- CSRF `state` and PKCE `code_verifier` held in short-lived `SameSite=Lax` cookies across the redirect
- ID tokens verified statelessly against the provider's JWKS — no session store
- `email_verified` claim required; providers that don't verify email cannot be used to hijack accounts
- First login provisions a `viewer` user (no local password); subsequent logins match by email

### Repository (`repository.go`)

All SQL access for `users`, `refresh_tokens`, and role/permission lookups. Never called directly by handlers — only by `Service`.

## Security properties

- Login responses are **timing-uniform** on the "user not found" vs "wrong password" paths — the API does not reveal which emails are registered.
- New self-registered accounts and first SSO logins get the **`viewer`** role only (least privilege by default).
- Refresh tokens are rotated on every use; a stolen token is usable only once before it is invalidated.

## Architecture decisions (ADRs)

| Decision | Rationale |
|---|---|
| Permissions in JWT, not DB lookup per request | Eliminates a DB round trip on every authenticated request; 15-min TTL bounds the staleness window for role changes |
| SHA-256 of refresh token stored, not plaintext | A DB dump alone is not sufficient to hijack sessions |
| `httpOnly` + `SameSite=Strict` refresh cookie | Prevents JavaScript (XSS) from reading the refresh token |
| PKCE for OIDC | Prevents authorization code interception even in public clients |

## Scope and responsibilities

- Issue, validate, and revoke JWT access tokens and opaque refresh tokens.
- Hash and verify passwords using `platform/crypto`.
- Implement OIDC login (PKCE + JWKS verification).
- Create new user accounts with the `viewer` role.
- Expose `Service` to the API layer; never call `Repository` from handlers.

## Limitations and todos

- [ ] No MFA (TOTP/WebAuthn) support.
- [ ] No account lockout after repeated failed login attempts (rate limiting is at the IP level only).
- [ ] OIDC session expiry (ID token `exp`) is not enforced after login — only JWT TTL governs session validity.
- [ ] No device/session management UI (listing and revoking individual sessions).
- [ ] Single OIDC provider active at a time in practice; multi-provider config exists but the UI exposes only one.
