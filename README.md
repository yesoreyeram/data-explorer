# Data Explorer

An advanced, self-hosted data exploration platform: connect to databases and
APIs, stitch data together in a visual pipeline builder, and explore the
results — with enterprise-grade RBAC, audit logging, and observability built
in from day one.

- **Connect** to PostgreSQL, MySQL, and REST APIs through centrally-managed,
  encrypted-at-rest connections.
- **Build** Postman/n8n-style pipelines: drag source, filter, transform
  ([JSONata](https://jsonata.org)), join, and aggregate nodes onto a canvas
  and wire them together.
- **Explore** results in a dense, keyboard-friendly UI with a light/dark/system
  theme switcher.
- **Govern** access with role-based permissions and a full audit trail of who
  did what, from where, and whether it succeeded.

## Stack

| Layer     | Technology                                                            |
| --------- | ---------------------------------------------------------------------- |
| Backend   | Go 1.25, chi router, pgx (PostgreSQL driver), JWT auth, Prometheus     |
| Frontend  | React 19, TypeScript, Vite, React Flow, TanStack Query, Zustand        |
| Database  | PostgreSQL (system of record: users, connections, workflows, audit)   |
| Transform | [JSONata](https://jsonata.org) (via `blues/jsonata-go`)               |

See [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) for how the pieces fit
together, [`docs/DEVELOPER_GUIDE.md`](docs/DEVELOPER_GUIDE.md) for local setup
and contribution workflow, and [`docs/SECURITY.md`](docs/SECURITY.md) for the
security model and threat considerations.

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
backend/    Go API server (see backend/README via docs/DEVELOPER_GUIDE.md)
frontend/   React + TypeScript SPA
docs/       Architecture, developer guide, security notes
deploy/     docker-compose, Dockerfiles
```

## License

See [LICENSE](LICENSE).
