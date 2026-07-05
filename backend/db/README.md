# db

## What this package does

`db` owns the embedded SQL migration files and the `migrations.go` entry point that binds them into a Go `embed.FS`. It has no Go logic of its own beyond the `//go:embed` directive. The actual migration runner lives in `internal/platform/migrator`.

## Migration conventions

| Convention | Detail |
|---|---|
| File naming | `NNNN_descriptive_name.up.sql` / `NNNN_descriptive_name.down.sql` |
| Sequential numbering | `0001`, `0002`, … never skip a number |
| Applied automatically | The migrator runs on every server boot before accepting connections |
| No retroactive edits | Once a migration is applied in production, it must never be modified; create a new `NNNN+1` migration instead |
| Idempotent `up` files | Use `CREATE TABLE IF NOT EXISTS`, `ALTER TABLE … ADD COLUMN IF NOT EXISTS`, etc. |

## Current migration set

| File | Description |
|---|---|
| `0001_initial_schema.up.sql` | Core tables: `users`, `roles`, `permissions`, `user_roles`, `role_permissions`, `refresh_tokens` |
| `0002_seed_rbac.sql` | Seeds the three system roles (`admin`, `editor`, `viewer`) and their permission bundles |
| `0003_connections_workflows.up.sql` | `connections`, `workflow_executions` tables |
| `0004_connection_health.up.sql` | Adds structured health columns to `connections`: `last_error_code`, `last_error_remediation`, `last_check_duration_ms` |
| `0005_workflows_schedule.up.sql` | Adds `schedule_cron`, `schedule_enabled`, `schedule_next_run` to `workflows` and the partial index that makes the scheduler's due-check query cheap |
| `0006_audit_logs.up.sql` | `audit_logs` append-only table |

## Scope and responsibilities

- Store SQL schema definitions as embedded files.
- Expose a single `embed.FS` (`Migrations`) consumed by `internal/platform/migrator`.

## Architecture note

Embedding migrations in the binary ensures schema and code are always in sync: you cannot deploy a binary without its migrations. The runner tracks applied files in a `schema_migrations` table so re-running on an already-migrated database is a no-op.

## Limitations and todos

- [ ] No automated generation of down migrations (rollbacks must be hand-authored).
- [ ] No migration locking for multi-replica boot races; the `IF NOT EXISTS` idiom and Postgres advisory locks are worth exploring.
- [ ] Seed data (`0002_seed_rbac.sql`) is also applied by the migrator; a dedicated seed step separate from schema migrations is cleaner and should be considered.
