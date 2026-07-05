# internal/domain

## What this package does

`internal/domain` contains the **core entity structs** shared across all application layers. It is deliberately free of any dependency on transport (HTTP), storage (SQL), auth, or business logic — it can be imported by any layer without creating import cycles.

## Entities

| Type | Description |
|---|---|
| `User` | Registered user: ID, email, display name, status (`active`/`suspended`), roles |
| `Role` | Named permission bundle: ID, name, description, `isSystem`, permissions |
| `Permission` | Fine-grained access code: ID, `code` (e.g. `connections:write`), description |
| `Connection` | Stored data source link: ID, name, type, config (JSONB), status, health fields |
| `ConnectionType` | Enum: `postgres`, `mysql`, `rest`, `graphql`, `aws`, `gcp`, `azure` |
| `ConnectionStatus` | Enum: `unverified`, `healthy`, `unhealthy` |
| `Workflow` | Saved pipeline: ID, name, description, definition (JSONB DAG), schedule fields |
| `WorkflowExecution` | Run record: ID, workflow ID, status, duration, per-node results, triggered-by |
| `AuditLog` | Immutable event: ID, actor, action, resource type/ID, outcome, IP, user-agent, request ID, timestamp |

## Key design choices

### `Connection.Config` and `Connection.SecretEncrypted`

The `domain.Connection` type carries only the non-secret `Config` JSONB. There is **no** `Secret` or `Password` field — the encrypted secret lives in `secret_encrypted` in the database and is never surfaced through the domain type. This makes it structurally impossible to accidentally include a decrypted secret in an API response.

### `WorkflowExecution.TriggeredBy` is a `string`, not a foreign key

Scheduled runs have no acting user. Inventing a synthetic `"scheduler"` user to satisfy a FK would pollute every user list. Using a plain string (`"scheduler"` or a user UUID) is intentional; it is documented in `ARCHITECTURE.md`.

### No behaviour in domain types

Methods on domain types are limited to marshalling helpers. No validation, no business rules, no database access. This keeps the package testable without any infrastructure.

## Scope and responsibilities

- Define the canonical in-memory shape of every application entity.
- Define type-safe enumerations (connection types, status values, user status).
- Provide `json` tags for the JSON wire format.
- Nothing else.

## Limitations and todos

- [ ] No OpenAPI schema generation from these types; the API schema is maintained separately.
- [ ] `Config json.RawMessage` means the config shape for each connection type is not enforced at the domain layer — it is validated in the connector layer.
- [ ] `AuditLog.Metadata` is an unstructured map; consider typed per-action metadata in a future iteration.
