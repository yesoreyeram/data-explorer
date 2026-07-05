# Developer Guide

This guide covers local setup, day-to-day conventions, and how to extend the
two things most likely to grow: connector types and workflow node types. For
the system design rationale, see [`ARCHITECTURE.md`](ARCHITECTURE.md); for
the threat model, see [`SECURITY.md`](SECURITY.md).

## Prerequisites

- Go 1.25+
- Node.js 20+ and npm
- PostgreSQL 14+ (local install, or via Docker)

## Local setup

### 1. Database

Any local Postgres works. Quick version with a system install:

```bash
sudo -u postgres psql -c "CREATE USER data_explorer WITH PASSWORD 'data_explorer';"
sudo -u postgres psql -c "CREATE DATABASE data_explorer OWNER data_explorer;"
```

### 2. Backend

```bash
cd backend
go mod download

export DATABASE_URL="postgres://data_explorer:data_explorer@localhost:5432/data_explorer?sslmode=disable"
go run ./cmd/server
```

On startup the server applies its own embedded migrations
(`db/migrations/*.sql`) and seeds the `admin`/`editor`/`viewer` roles - there
is nothing else to run. It listens on `:8080` by default.

Run the test suite:

```bash
go test ./...
go vet ./...
```

### 3. Frontend

```bash
cd frontend
npm install
npm run dev
```

Vite serves on `:5173` and proxies `/api/v1`, `/healthz`, `/readyz` to
`localhost:8080` (see `frontend/vite.config.ts`) - so in dev, the frontend
talks to relative URLs and never needs a CORS round trip. In production,
put both services behind a reverse proxy on the same origin, or set
`VITE_API_URL` to the API's absolute URL at build time.

Type-check, lint, build:

```bash
npx tsc -b
npm run lint
npm run build
```

Guardrail UI conventions:

- Render `DataFrame.meta.warnings` as user-visible soft warnings without
  treating them as failed operations.
- Before CSV export, warn when a frame is truncated or above the 10K visible
  row export cap; offer a bounded export and a refine-query escape hatch.
- Workflow node run metadata (`rowsOut`, `rowCap`, `truncated`, `warnings`,
  `timeoutMs`) should be surfaced on the canvas with text labels as well as
  warning/danger styling.
- Use the shared navigation layer (`navigation.ts` + `navigationStore.ts`) for
  breadcrumbs, favorites, recent activity, and the command palette instead of
  page-local implementations.
- Prefer `DataFrameView`'s built-in chart/table/split modes and
  `savedChartsStore` for quick visualizations before introducing a new
  dashboard-specific chart flow.

### 4. First login

Register a user through the UI (or `POST /api/v1/auth/register`). New
accounts start as `viewer`. Promote yourself to `admin` directly in the
database once:

```sql
INSERT INTO user_roles (user_id, role_id)
SELECT u.id, r.id FROM users u, roles r
WHERE u.email = 'you@example.com' AND r.name = 'admin';
```

## Environment variables (backend)

All configuration is environment variables (`internal/config/config.go`) -
with one operator-facing exception: `GUARDRAILS_CONFIG_FILE` can point to a
small JSON document that overrides the platform guardrails and per-role
explore/workflow hourly quotas. Everything else has a safe local-dev default
except in `APP_ENV=production`, where `JWT_SIGNING_KEY` and
`CONNECTION_ENCRYPTION_KEY` are required.

| Variable                     | Default                                   | Notes                                              |
| ----------------------------- | ------------------------------------------ | --------------------------------------------------- |
| `APP_ENV`                     | `development`                             | `production` enables stricter validation + secure cookies |
| `HTTP_ADDR`                    | `:8080`                                   |                                                       |
| `HTTP_ALLOWED_ORIGINS`         | `http://localhost:5173`                   | comma-separated CORS allow-list                     |
| `DATABASE_URL`                 | `postgres://data_explorer:...@localhost/...` | pgx connection string                            |
| `JWT_SIGNING_KEY`               | dev-only insecure default                 | **required**, ≥32 bytes, in production              |
| `CONNECTION_ENCRYPTION_KEY`     | dev-only insecure default                 | **required** in production; `openssl rand -base64 32` |
| `ACCESS_TOKEN_TTL`              | `15m`                                     |                                                       |
| `REFRESH_TOKEN_TTL`             | `168h` (7d)                               |                                                       |
| `LOG_LEVEL` / `LOG_FORMAT`      | `info` / `json`                           | `LOG_FORMAT=text` for readable local dev logs        |
| `GUARDRAILS_CONFIG_FILE`        | unset                                     | optional JSON overrides for body/row/page/timeout/cell/JSON limits and `role_quotas` |

## Code conventions

- **Repository / Service split**: every domain package (`auth`,
  `connections`, `workflow`, `audit`) has a `Repository` (SQL only) and a
  `Service` (business rules + validation, calls the repository). Handlers
  call services only, never repositories or SQL directly.
- **Errors**: repositories return sentinel errors (`ErrNotFound`,
  `ErrConflict`) that handlers translate to HTTP status codes with
  `errors.Is`. Don't leak driver-specific errors (`pgx.ErrNoRows`, SQL state
  codes) past the repository boundary.
- **No comments explaining *what* the code does** - names should do that.
  Comments are reserved for *why* something non-obvious is the way it is
  (see the existing code for the tone to match).
- **Tests** live next to the code they test (`_test.go`). Favor pure,
  dependency-free unit tests (see `internal/workflow/engine_test.go`, which
  exercises the DAG engine with a stub source node instead of a real
  database) over integration tests that need a live Postgres, where
  possible.

## Frontend design system conventions

See [`ARCHITECTURE.md`](ARCHITECTURE.md#frontend) for the full picture, and
[`DESIGN.md`](../DESIGN.md) for the design-token reference and per-primitive
usage guidance. In new UI code:

- **Use `src/components/ui/`** (`Button`, `IconButton`, `Field`, `Input`,
  `Select`, `Textarea`, `Badge`, `Card`/`CardHeader`/`CardBody`, `StatTile`,
  `Kbd`, `Divider`, `PageHeader`, `SectionLabel`, `EmptyState`) instead of
  raw `className="btn"`/`"input"`/`"field"` strings. They render the exact
  same markup/classes, so this is purely about not hand-rolling the same
  boilerplate (label/hint wiring, `type="button"` defaults, disabled
  states) at every call site.
- **Don't introduce a new color.** The palette (`src/styles/tokens.css`) is
  intentionally near-monochrome - every structural color is grayscale, and
  `--success`/`--warning`/`--danger`/`--info` are the only hues, reserved
  for `Badge`'s status dot and the `StatTile` trend delta. A new feature
  needing to convey state should reach for one of those four tones, not a
  new hex/hsl value.
- **Spacing/sizing comes from the token scale** (`--space-1` through
  `--space-8`, `--radius-sm/md/lg/xl/pill`, `--font-size-xs..3xl`) - avoid
  one-off pixel values in new component CSS.
- **Every primitive has a Storybook story.** Run `npm run storybook` inside
  `frontend/` to browse the catalog and try variants live. Add a
  `<Name>.stories.tsx` sibling next to any new primitive.
- Not everything has been migrated to `ui/` primitives - the deepest,
  most-repeated field sets (`AuthTypeFields`, `CloudConnectionFields`,
  `CloudQueryFields`, `PaginationFields`, `NodeConfigPanel`) still use the
  raw classes directly. That's fine: the classes *are* the design system,
  the `ui/` components are just a typed convenience over them, so there's no
  visual or behavioral gap between the two.

## Adding a new connection type (connector)

Connectors implement one interface (`internal/connections/connector.go`),
returning the standalone `pkg/dataframe` type - see
[`ARCHITECTURE.md`](ARCHITECTURE.md#dataframe-the-tabular-data-contract) for
what a `Frame` actually is:

```go
type Connector interface {
    Test(ctx context.Context, config json.RawMessage, secret map[string]string) error
    Execute(ctx context.Context, config json.RawMessage, secret map[string]string, spec QuerySpec) (*dataframe.Frame, error)
}
```

Steps, using the existing connectors as templates
(`internal/connections/connectors/{postgres,mysql,rest,graphql}.go`):

1. Create `internal/connections/connectors/<name>.go`. Define a config
   struct for the non-secret fields (host, base URL, ...) and read secrets
   out of the `secret map[string]string` passed to you (never log it).
2. If your source can execute arbitrary read queries, reuse
   `EnsureReadOnlySQL` from `sqlguard.go` for defense-in-depth, or apply an
   equivalent guard for your query language. If it's HTTP-based, build it on
   `pkg/httpclient` (see the next section) rather than raw `net/http`, to get
   the auth matrix, pagination, and guardrails for free.
3. Respect `connections.EffectiveRowLimit(spec.RowLimit)` and set a context
   timeout - every connector must be bounded.
4. Build the result with `dataframe.New(nil)` + `frame.AppendRow(...)` per
   row (or `dataframe.FromRecords(rows)` if you already have `[]map[string]any`),
   then `frame.SetMeta(dataframe.Metadata{SourceType: "my-source", ...})`.
   Leave `SourceID`/`Name` unset - `connections.Service.Query` stamps those
   from the `Connection` record after your `Execute` returns.
5. Register it in `cmd/server/main.go`:
   ```go
   connectorRegistry.Register("my-source", connectors.NewMySource())
   ```
   HTTP-backed connectors should receive the current `config.GuardrailsConfig`
   so safe JSON decoding, redirect/page/body limits, and decompression-ratio
   protection stay centralized.
6. Add the type to `domain.ConnectionType` (`internal/domain/models.go`), the
   frontend's `ConnectionType` union (`frontend/src/api/types.ts`), and the
   type-specific config fields in
   `frontend/src/pages/connections/ConnectionTypeConfigFields.tsx` - this one
   component backs both `ConnectionFormModal` (persisted) and the Explore
   page's temporary-connection mode (never persisted), so it only needs
   adding once. If the type supports ad-hoc queries, add its query-shape
   fields to `frontend/src/components/QuerySpecFields.tsx` and
   `buildQuerySpec()`/`summarizeQuery()` in `lib/querySpec.ts` - shared the
   same way across the query modal and the Explore page.

### Extending the health-error classification

A connector should just return whatever error its driver/SDK/HTTP client
gave it (wrapped with `fmt.Errorf("%w", ...)` for context, if useful) -
`connections.Classify` is the single place that turns that into a
`HealthError` (stable `Code`, user-facing `Message`, actionable
`Remediation`), applied centrally in `Service.Test`/`Query`/`QueryAdhoc`.
Connectors don't need to call `Classify` themselves.

To improve classification for a case that's currently falling through to
`ErrCodeUnknown`:

1. If the underlying library exposes a typed error (like `*pgconn.PgError`
   or `smithy.APIError`), add/extend a `classify<Thing>` helper in
   `internal/connections/healtherror.go` and an `errors.As` branch in
   `Classify` - see `classifyPostgres`/`classifyAWS` for the pattern of
   mapping a driver-specific code to one of the existing `ErrorCode` values.
2. If it's an untyped error from a library that only returns strings, add a
   `containsAny(msg, ...)` case to `classifyByMessage` instead - keep it as
   the last resort, since a typed check is always more reliable than
   substring matching.
3. If a connector wants to reject bad config before ever dialing out (e.g. a
   missing required field), return `connections.NewConfigError("...")`
   rather than a plain `fmt.Errorf` - it's already a `HealthError` with
   `ErrCodeInvalidConfig`, so `Classify` passes it through unchanged.
4. Add a case to `healtherror_test.go`'s table-driven tests for the new code
   path (see `TestClassifyPostgres`/`TestClassifyByMessageFallback` for the
   shape) - one line per input/expected-`ErrorCode` pair.

Don't invent a new `ErrorCode` value casually - the existing eight are meant
to cover "what should the user actually go check" (credentials vs.
permissions vs. network vs. rate limit vs. ...), not to mirror every
provider's error taxonomy 1:1.

### Adding a new HTTP auth scheme

If you're adding a scheme `pkg/httpclient` doesn't already cover (Basic,
Bearer, API key, Digest, OAuth2 client-credentials/refresh-token, JWT,
workload identity federation, Kerberos):

1. Implement `httpclient.Authenticator` (`Authenticate(ctx, *http.Request)
   error`) in a new `pkg/httpclient/auth_<name>.go`. If the scheme needs to
   inspect a response before it can authenticate (like Digest's
   challenge-response), implement `RoundTripperAuthenticator` instead - see
   `auth_digest.go`.
2. Add a case to `buildAuthenticator` in
   `internal/connections/connectors/httpauth.go`, mapping `AuthConfig`
   fields (non-secret) and named `secret[...]` keys (credentials) to your
   authenticator's constructor. Document which secret keys it reads in the
   `AuthConfig` field comments.
3. On the frontend: add the value to `AuthType` (`frontend/src/api/types.ts`)
   and a case in `AuthTypeFields.tsx` for its config/secret fields.
4. Write a `httptest.Server`-backed test in `pkg/httpclient/auth_test.go`
   (see the existing ones for the pattern - assert on what the server
   actually received, not just that `Authenticate` didn't error).

### Adding a new service to an existing cloud connector

`aws`, `gcp`, and `azure` are each one `Connector`, but each dispatches on a
`config.service` field to one of several sub-implementations (see
[`ARCHITECTURE.md`](ARCHITECTURE.md#cloud-provider-connectors)). To add a new
service to a cloud that's already wired up (e.g. AWS Redshift Data API
alongside Athena):

1. Add the service's non-secret config fields (if any) to that cloud's config
   struct (`AWSConfig`/`GCPConfig`/`AzureConfig` in `connectors/{aws,gcp,azure}.go`)
   and its `service` to the validation switch in `parseConfig`.
2. Create `internal/connections/connectors/<cloud>_<service>.go` with
   `test<Service>`/`execute<Service>` functions, following the shape of the
   existing ones (e.g. `aws_athena.go`). Reuse the cloud's shared credential
   helper (`awsConfig`/`gcpClientOptions`/`azureCredential`) rather than
   building a client from scratch - that's what keeps the ambient-credential
   fallback *and* the alternative auth methods (AWS AssumeRole, GCP service
   account impersonation, Azure client certificate) working for every
   service under that cloud, without each service file re-implementing them.
3. If the service's underlying API is async (start-then-poll, like Athena and
   CloudWatch Logs Insights), poll with `AsyncQueryPollInterval`/
   `AsyncQueryMaxWait` from `cloudguardrails.go` rather than a bespoke loop.
   If it reads objects (like S3/GCS/Blob Storage), reuse
   `objectparse.go`'s `InferObjectFormat`/`ParseObjectRows` and cap reads at
   `MaxObjectBytes`.
4. Wire the new `service` value into the cloud's `Test`/`Execute` dispatch
   switch (`connectors/{aws,gcp,azure}.go`).
5. On the frontend: add the service to the relevant `AWSService`/
   `GCPService`/`AzureService` union (`frontend/src/api/types.ts`), its option
   in `CloudConnectionFields.tsx` (connection-level config) and, if it needs
   query-time parameters beyond what `CloudQuerySpec` already covers, extend
   `CloudQuerySpec` and `CloudQueryFields.tsx`.
6. Add a `parseConfig`/credential-selection test in
   `connectors/<cloud>_test.go` (see the existing ones - actual SDK calls
   aren't exercised in CI, so these tests cover config validation and which
   credential path gets chosen, not live cloud behavior).

## Adding an integration catalog entry

The "Browse catalog" picker on the Connections page (`internal/catalog`,
see [`ARCHITECTURE.md`](ARCHITECTURE.md#integration-catalog-prefilling-not-proxying))
is a static, hand-curated list - there's no ingestion pipeline to run. To add
one:

1. Add an `Entry{}` to the slice in `internal/catalog/seed.go`. `AuthType`
   must be one of the values `connectors/httpauth.go`'s `AuthConfig.AuthType`
   already supports (see [`ARCHITECTURE.md`](ARCHITECTURE.md#httpclient-the-outbound-http-layer));
   `AuthConfig` should only ever carry non-secret fields (a header name, a
   token URL) - never a placeholder credential.
2. If the service needs a per-tenant subdomain/workspace id in its URL
   (Shopify, Salesforce, Zendesk, ...), use a `{placeholder}` in
   `BaseURL`/`Endpoint` - the form prefills it as-is and the user edits it
   before saving, same as any other prefilled field.
3. Add a case to `internal/catalog/service_test.go` if the new entry changes
   an existing test's expected count (e.g. a new entry in the "Email"
   category).

There's no frontend change needed - `CatalogBrowserModal.tsx` and
`ConnectionFormModal.tsx`'s prefill logic are entirely data-driven off
whatever `GET /api/v1/catalog` returns.

## Adding a new workflow node type

Node executors implement `nodes.Executor` (`internal/workflow/nodes/types.go`),
operating entirely on `*dataframe.Frame`:

```go
type Executor interface {
    Execute(ctx context.Context, deps Deps, in ExecInput) (*dataframe.Frame, error)
}
```

Steps, using `internal/workflow/nodes/{transform,filter,join,aggregate}.go`
as templates:

1. Create `internal/workflow/nodes/<name>.go`. Read your node's config from
   `in.Config` (JSON), and its upstream data from `in.Inputs` -
   `in.SingleInput()` for the common one-input case, or
   `in.Inputs["left"]`/`in.Inputs["right"]`-style named handles if your node
   needs multiple distinguishable inputs (see `join.go`). Prefer an
   operation already on `dataframe.Frame` (`Select`, `Rename`, `Filter`,
   `Join`, `GroupBy`, `Describe`) over writing new row-shuffling logic in the
   node itself - `join.go`/`aggregate.go` are ~30-line adapters for exactly
   this reason.
2. Register it in `nodes.DefaultRegistry()` (`internal/workflow/nodes/types.go`).
3. Add the type to `workflow.NodeType` (`internal/workflow/definition.go`)
   so it passes `Definition.Validate()`.
4. On the frontend: add it to `NodeType` (`frontend/src/api/types.ts`), the
   palette (`WorkflowBuilderPage.tsx`'s `PALETTE`/`DEFAULT_CONFIG`), an icon
   in `pages/workflow/workflowIcons.tsx` + `FlowNode.tsx`'s `META` map, and a
   config form in `pages/workflow/NodeConfigPanel.tsx`.
5. Write a table-driven test in `internal/workflow/nodes/<name>_test.go` and,
   if it changes how the engine wires inputs, extend
   `internal/workflow/engine_test.go`.

## Scheduled workflow execution

See [`ARCHITECTURE.md`](ARCHITECTURE.md#scheduled-workflow-execution) for
the design. There's nothing to wire up to add this to a new workflow - it's
already a first-class field on every workflow (`PUT /workflows/{id}/schedule`,
the "Schedule" button in the builder). Things worth knowing when touching
this code:

- `internal/scheduler.PollInterval` (15s) is the effective floor on
  schedule precision, not the cron expression's own granularity - a
  "run every minute" workflow can fire up to 15s late. Lowering it trades
  precision for more frequent `DueSchedules` queries; there's a partial
  index (`idx_workflows_schedule_due`) so that query stays cheap regardless.
- Cron expressions are standard 5-field (`minute hour dom month dow`, no
  seconds) via `github.com/robfig/cron/v3`'s `ParseStandard` - see
  `internal/workflow/schedule.go`. If you need seconds-level scheduling,
  that's `cron.ParseStandard` → a 6-field parser away, but nothing in this
  codebase needs that precision today.
- A scheduled run is indistinguishable from a manual one in the execution
  history except for `triggeredBy` (`"scheduler"` vs. a user id) - it goes
  through the exact same `workflow.Service.Execute`, so it gets the same
  `MaxExecutionDuration` bound and the same guardrails as any other run.

## Database migrations

Migrations are plain `.sql` files in `backend/db/migrations/`, embedded into
the binary and applied automatically on startup by
`internal/platform/migrator`, tracked in a `schema_migrations` table. To add
one: create `NNNN_description.sql` with the next number (lexical ordering =
apply order), forward-only (no down migrations by design - roll forward with
a new corrective migration instead).

## API surface

All routes are under `/api/v1`, JSON in/out, bearer-token authenticated
(except `/auth/login|register|refresh`). See `internal/api/router.go` for
the definitive route table and the permission required per route; the
handler for each lives in `internal/api/handlers/`.

Two infrastructure endpoints sit outside the API version prefix:
`/healthz` (liveness), `/readyz` (readiness), `/metrics` (Prometheus).

## Troubleshooting

- **"JWT_SIGNING_KEY must be set..." on boot**: you're running with
  `APP_ENV=production` without setting the required secrets. Set them, or
  unset `APP_ENV`/use `development` locally.
- **Frontend gets 401s in the browser console on first load**: expected -
  the app always attempts a silent `/auth/refresh` on mount to restore a
  session from the cookie; with no prior login there's no cookie yet, so it
  401s once and falls back to the login page.
- **`only SELECT queries are allowed`**: the SQL guard rejected your query;
  see [`SECURITY.md`](SECURITY.md#data-source-access-sql-connectors). Only
  single `SELECT`/`WITH` statements are permitted through the API.
