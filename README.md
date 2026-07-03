# Data Explorer

An advanced, self-hosted data exploration platform: connect to databases and
APIs, stitch data together in a visual pipeline builder, and explore the
results — with enterprise-grade RBAC, audit logging, and observability built
in from day one.

- **Connect** to PostgreSQL, MySQL, REST, and GraphQL APIs through
  centrally-managed, encrypted-at-rest connections, authenticating with
  whatever the upstream actually requires — Basic, Bearer, API key, Digest,
  OAuth2 (client credentials or refresh token), self-signed JWT, workload
  identity federation, or Kerberos/SPNEGO.
- **Build** Postman/n8n-style pipelines: drag source, filter, transform
  ([JSONata](https://jsonata.org)), join, and aggregate nodes onto a canvas
  and wire them together. Every node speaks the same typed, metadata-rich
  **dataframe** contract, so nodes compose regardless of what produced their
  input.
- **Paginate** REST APIs (offset/limit, page number, cursor, Link header) and
  GraphQL APIs (Relay cursor connections) automatically, with hard caps so a
  misconfigured "next page" can't loop forever.
- **Explore** results in a dense, keyboard-friendly UI with a light/dark/system
  theme switcher.
- **Govern** access with role-based permissions and a full audit trail of who
  did what, from where, and whether it succeeded — backed by guardrails at
  every layer (row limits, response size caps, rate limits, execution
  timeouts) so no single misbehaving pipeline can take the service down.

## Screenshots

| | |
| --- | --- |
| ![Dashboard](docs/screenshots/02-dashboard-light.png) Dashboard overview | ![Workflow builder](docs/screenshots/08-workflow-builder-dark.png) Workflow builder |
| ![Connection auth](docs/screenshots/05-connection-form-graphql-oauth2.png) Connection auth (GraphQL + OAuth2) | ![Dataframe result](docs/screenshots/06-dataframe-query-result.png) Query result with dataframe metadata |
| ![Run output & lineage](docs/screenshots/09-workflow-run-output.png) Run output with lineage | ![Audit log](docs/screenshots/10-audit-log.png) Audit log |

More in [`docs/screenshots/`](docs/screenshots/), including the login page,
connections list, workflows list, and user/role administration.

## Stack

| Layer     | Technology                                                            |
| --------- | ---------------------------------------------------------------------- |
| Backend   | Go 1.25, chi router, pgx (PostgreSQL driver), JWT auth, Prometheus     |
| Frontend  | React 19, TypeScript, Vite, React Flow, TanStack Query, Zustand        |
| Database  | PostgreSQL (system of record: users, connections, workflows, audit)   |
| Transform | [JSONata](https://jsonata.org) (via `blues/jsonata-go`)               |
| Tabular data | `backend/pkg/dataframe` — standalone pandas-style Frame/Schema/Metadata library |
| HTTP client  | `backend/pkg/httpclient` — standalone client with pluggable auth + pagination |

Two of the backend packages are deliberately standalone, dependency-free
libraries rather than being woven into the app:

- **`pkg/dataframe`** — the tabular data contract every connector and every
  workflow node produces and consumes: typed columns, schema inference,
  join/group-by/describe, and rich per-frame metadata (source, lineage,
  timing, truncation, warnings). See [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md#dataframe-the-tabular-data-contract).
- **`pkg/httpclient`** — a from-scratch HTTP client supporting Basic,
  Bearer, API key, Digest (RFC 7616 challenge-response), OAuth2 (via
  `golang.org/x/oauth2`), self-signed JWT, RFC 8693 workload identity
  federation, and Kerberos/SPNEGO (via `gokrb5`), plus five pagination
  strategies including GraphQL Relay cursors. See
  [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md#httpclient-the-outbound-http-layer).

See [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) for how the pieces fit
together, [`docs/DEVELOPER_GUIDE.md`](docs/DEVELOPER_GUIDE.md) for local setup
and contribution workflow, and [`docs/SECURITY.md`](docs/SECURITY.md) for the
security model, guardrails, and threat considerations.

## Quick start

### With Docker Compose (recommended)

```bash
cp deploy/.env.example deploy/.env   # then edit the secrets inside
docker compose -f deploy/docker-compose.yml --env-file deploy/.env up --build
```

The frontend is served at http://localhost:5173, the API at
http://localhost:8080. On first boot the API applies its own database
migrations and seeds the built-in `admin` / `editor` / `viewer` roles - there
is no separate migration step to run.

Register the first account through the UI (or `POST /api/v1/auth/register`);
new accounts start as `viewer`. Promote your own account to `admin` once:

```sql
INSERT INTO user_roles (user_id, role_id)
SELECT u.id, r.id FROM users u, roles r
WHERE u.email = 'you@example.com' AND r.name = 'admin';
```

### Running locally without Docker

```bash
# 1. Postgres (any local instance works)
createuser data_explorer --pwprompt
createdb data_explorer -O data_explorer

# 2. Backend
cd backend
DATABASE_URL="postgres://data_explorer:PASSWORD@localhost:5432/data_explorer?sslmode=disable" \
  go run ./cmd/server

# 3. Frontend (separate shell)
cd frontend
npm install
npm run dev
```

Full details, including required environment variables, are in
[`docs/DEVELOPER_GUIDE.md`](docs/DEVELOPER_GUIDE.md).

## Repository layout

```
backend/
  cmd/server/        entrypoint
  internal/           app-specific packages (auth, connections, workflow, api, ...)
  pkg/dataframe/       standalone tabular data library
  pkg/httpclient/      standalone HTTP client (auth matrix + pagination)
frontend/            React + TypeScript SPA
docs/                Architecture, developer guide, security notes, screenshots
deploy/              docker-compose, Dockerfiles
```

## License

See [LICENSE](LICENSE).
