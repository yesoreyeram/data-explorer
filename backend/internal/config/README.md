# internal/config

## What this package does

`internal/config` provides **12-factor, environment-variable-based configuration** for the entire application. It is the single source of truth for every tuneable: database URL, JWT secrets, encryption keys, HTTP settings, guardrails, OIDC providers, and egress policy. Config is loaded once at startup and passed to every service via constructor injection.

## Design

```go
type Config struct {
    AppEnv          string          // "development" | "production"
    DatabaseURL     string
    HTTP            HTTPConfig
    Auth            AuthConfig
    EncryptionKey   []byte          // 32-byte AES-256 key; never logged
    Guardrails      GuardrailsConfig
    OIDC            OIDCConfig
    Egress          EgressConfig
    // …
}
```

`Load()` reads `os.Getenv` for every field, applies defaults, then validates. Validation fails fast with a clear error listing every missing/invalid key rather than panicking mid-startup.

## Security constraints enforced at load time

- `APP_ENV=production` **requires** `CONNECTION_ENCRYPTION_KEY` to be explicitly set (no fallback default in production).
- `JWT_SECRET` is validated to be at least 32 bytes.
- `CONNECTION_ENCRYPTION_KEY` must decode to exactly 32 bytes.

## GuardrailsConfig

Holds per-role hourly quotas for explore runs and workflow runs, plus the global row limits and response size caps referenced by connectors. This is the single place where "how many queries can an editor run per hour?" is decided.

## Architecture decisions (ADRs)

| Decision | Rationale |
|---|---|
| Fail-fast validation | A misconfigured production deployment fails immediately at boot with a clear diagnostic, rather than misbehaving at runtime hours later |
| Single `Config` struct passed by value | Immutable after construction; no race conditions, no "set a config key at runtime" footguns |
| No config file | 12-factor compliance; secrets live in the environment, not in checked-in files |
| `APP_ENV` guards production defaults | Prevents accidentally running with development-safe defaults in production |

## Scope and responsibilities

- Load all configuration from environment variables.
- Apply defaults for optional settings.
- Validate all required and security-critical settings.
- Expose a single typed `Config` struct to the rest of the application.

## Limitations and todos

- [ ] No hot-reload; changing a config value requires a process restart.
- [ ] No config schema documentation generated automatically (e.g., to an `.env.example` file).
- [ ] Encryption key rotation is a manual process; there is no automatic re-encryption tooling.
- [ ] `GuardrailsConfig` role quotas require a process restart to take effect.
