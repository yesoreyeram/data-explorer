# Data Explorer — Copilot Repository Instructions

## Project overview

Data Explorer is a full-stack, enterprise-grade data exploration and pipeline platform:
- **Backend**: Go 1.25+, `chi` router, PostgreSQL (pgx v5), embedded migrations, chi middleware chain.
- **Frontend**: React 18, TypeScript, Vite, React Flow (workflow builder), Oxlint.
- **Auth/Security**: Argon2id passwords, short-lived HS256 JWT + rotating `httpOnly` refresh tokens, fine-grained RBAC, AES-256-GCM connection-secret encryption, append-only audit log.
- **Connectors**: Postgres, MySQL, REST, GraphQL, AWS (Athena / CloudWatch / DynamoDB / S3), GCP (BigQuery / GCS), Azure (Log Analytics / Blob Storage).
- **Workflow engine**: topologically-sorted DAG with source / filter / transform / join / aggregate / output node types; in-process cron scheduler.
- **Observability**: Prometheus metrics (`internal/observability`), structured `slog` logging with request-id propagation.

## Repository layout

```
backend/
  cmd/server/          entrypoint
  db/migrations/       embedded SQL migrations
  pkg/dataframe/       pandas-style Frame/Schema
  pkg/httpclient/      outbound HTTP client (auth, pagination, retries, caps)
  internal/
    config/            12-factor env config
    domain/            shared entity structs
    platform/          logger, crypto, dbx, migrator, httpx
    auth/              registration, login, JWT + refresh tokens
    rbac/              Principal, permission constants
    audit/             append-only audit writer + query
    connections/       connection CRUD, secret encryption, connector interface, rate limit, health errors
      connectors/      postgres, mysql, rest, graphql, aws, gcp, azure + sqlguard
    catalog/           static integration catalog
    workflow/          DAG definition, engine, scheduler, schedule parsing
    scheduler/         in-process cron poll loop
    observability/     Prometheus registry
    api/               middleware chain + handlers + router
frontend/
  src/
    components/ui/     Button, IconButton, Field, Input, Select, Textarea, Badge, Card*, StatTile
    components/        DataFrameView, DataTable, Modal, PermissionGate, ProtectedRoute, ThemeSwitcher, icons
    pages/             one file per route
    api/               typed fetch wrappers
    state/             Zustand stores
    index.css          design-token variables (near-monochrome)
docs/
  ARCHITECTURE.md
  DEVELOPER_GUIDE.md
  SECURITY.md
  screenshots/         PNG screenshots (named numerically: 01-*, 02-*, ...)
```

## Build and test commands

```bash
# Backend
cd backend && go build ./...
cd backend && go vet ./...
cd backend && go test ./... -race

# Frontend
cd frontend && npm ci
cd frontend && npx tsc -b
cd frontend && npm run lint       # Oxlint
cd frontend && npm run build      # Vite production build
```

CI runs both jobs on every push/PR (`.github/workflows/ci.yml`).

## Code style and conventions

### Go
- Table-driven tests with `t.Run` subtests; use `errors.Is`/`errors.As` for assertions.
- All errors flow through `connections.Classify` for structured `HealthError` taxonomy.
- Handlers are thin — logic lives in `Service` types; repositories handle only persistence.
- Always pass `context.Context` as the first parameter.
- No `init()` functions, no global mutable state outside the DI wiring in `cmd/server`.
- Structured log fields: `slog.String("key", val)`, never `fmt.Sprintf` in log calls.

### TypeScript / React
- `src/components/ui/` exports the design-system primitives — **always** use them instead of raw HTML elements or ad-hoc class strings.
- Design tokens are CSS custom properties in `src/index.css`; use `var(--token-name)`, never hard-coded hex.
- Accent is near-black on light / near-white on dark; `success`/`warning`/`danger` are the **only** hues — confined to status dots only.
- Permission checks: use `<PermissionGate permission="…">` in JSX; never duplicate RBAC logic in components.
- State: `useQuery` (react-query) for server data, Zustand for client-side UI state.

## Security requirements (mandatory for all changes)

1. **Never** log, echo, or return a decrypted secret, password, or token.
2. **Never** concatenate user input into SQL — use parameterized queries.
3. Every new mutating endpoint must be gated by a single RBAC permission at the router.
4. Every mutating action and sensitive read must emit an audit log entry.
5. Run `go vet ./...` and `npx tsc -b` before committing. Zero warnings/errors required.

## Screenshots requirement (mandatory for all PRs)

Every PR that adds or changes any visible UI must:

1. **Capture screenshots** of every new or changed screen using Playwright (`npm run dev` + `npx playwright screenshot`). Store PNGs in `docs/screenshots/` following the existing `NN-kebab-name.png` naming convention.
2. **Update `docs/screenshots/`** — stale screenshots from replaced screens must be deleted or overwritten.
3. **Reference screenshots in the PR description** — embed them with `![alt](docs/screenshots/NN-name.png)` or use a before/after table matching the style already used in this repo's PR descriptions.
4. If the PR is created by an automated agent, post the screenshot gallery as a PR comment if the PR body is already set.

Backend-only PRs with no UI change are exempt from steps 1–4.
