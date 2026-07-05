# internal/connections/connectors

## What this package does

`internal/connections/connectors` contains the **concrete connector implementations** for every supported data source type. Each connector implements the `connections.Connector` interface (`Test` + `Execute`), returning a `*dataframe.Frame`. The registry (`cmd/server/main.go`) maps `domain.ConnectionType` → a connector instance.

## Connectors

### postgres (`postgres.go`)

- Uses `pgx/v5` directly (no ORM).
- Query passed through `sqlguard.EnsureReadOnlySQL` — only `SELECT`/`WITH` are permitted; no data modification, DDL, or multiple statements.
- Rows streamed into a `dataframe.Frame` with automatic type inference.
- Reads `DATABASE_URL`-style DSN from the encrypted secret.

### mysql (`mysql.go`)

- Uses `go-sql-driver/mysql`.
- Same `sqlguard.EnsureReadOnlySQL` guard as Postgres.
- Egress guard applied to the dialer.

### rest (`rest.go`)

- Built on `pkg/httpclient.Client`.
- Supports all `httpclient` auth schemes (Basic, Bearer, API key, Digest, OAuth2, JWT, workload identity, Kerberos).
- Supports all five pagination strategies via `pkg/httpclient.Paginator`.
- JSON/array response parsed into rows via `objectparse.go`.
- Non-secret config (base URL, auth type, token URL, etc.) comes from `connection.Config`; credentials from the encrypted secret.

### graphql (`graphql.go`)

- Built on `pkg/httpclient.Client`.
- Only `GraphQLRelayPaginator` (Relay Cursor Connections: `edges { node }` + `pageInfo`).
- Response data extracted from `data.<rootField>` via configurable JSON path.

### aws (`aws.go` + sub-files)

Wraps `aws-sdk-go-v2`. Sub-services selected by `config.service`:

| Service | File | Query shape |
|---|---|---|
| Athena | `aws_athena.go` | SQL string; start-then-poll (`StartQueryExecution` / `GetQueryExecution`) |
| CloudWatch Logs Insights | `aws_cloudwatchlogs.go` | Log group names + time range + query string; start-then-poll |
| DynamoDB | `aws_dynamodb.go` | Key/filter expressions (Query or Scan) |
| S3 | `aws_s3.go` | Bucket + key/prefix; list or read single object (`objectparse.go`) |

Credentials: ambient IAM role by default; optional static credentials (access key/secret/session token) from encrypted secret; optional `RoleArn` for STS AssumeRole.

### gcp (`gcp.go` + sub-files)

Wraps `cloud.google.com/go/bigquery` and `cloud.google.com/go/storage`. Sub-services:

| Service | File | Query shape |
|---|---|---|
| BigQuery | `gcp_bigquery.go` | SQL string (synchronous) |
| Cloud Storage | `gcp_gcs.go` | Bucket + object path/prefix; list or read single object |

Credentials: Application Default Credentials by default; optional service account key JSON from encrypted secret; optional `ImpersonateServiceAccount`.

### azure (`azure.go` + sub-files)

Wraps `azure-sdk-for-go`. Sub-services:

| Service | File | Query shape |
|---|---|---|
| Log Analytics | `azure_loganalytics.go` | KQL query string (synchronous) |
| Blob Storage | `azure_blobstorage.go` | Container + blob path/prefix; list or read single object |

Credentials: `DefaultAzureCredential` by default; optional service principal (tenant/client/secret) or client certificate from encrypted secret.

## Shared connector utilities

### sqlguard (`sqlguard.go`)

`EnsureReadOnlySQL(query)` — keyword-based guard that rejects any query containing `INSERT`, `UPDATE`, `DELETE`, `DROP`, `CREATE`, `ALTER`, `TRUNCATE`, `GRANT`, `REVOKE` or multiple statements (`;` in the middle). This is defense-in-depth, not a full SQL parser.

### httpauth (`httpauth.go`)

Maps `AuthType` → `pkg/httpclient.Authenticator`. Reads credentials from the `secret map[string]string` using stable key names (`bearer_token`, `client_secret`, `username`, `password`, etc.). The canonical mapping from connection secret keys to auth scheme parameters is defined here.

### objectparse (`objectparse.go`)

Infers CSV/JSON/NDJSON format from object key suffix (or explicit `format` override) and parses object bytes into a `*dataframe.Frame`. Used by S3, GCS, and Azure Blob Storage object-read paths.

### paginationspec (`paginationspec.go`)

Maps a `REST.PaginationSpec` (from `connection.Config`) to the right `pkg/httpclient.Paginator` implementation.

### cloudguardrails (`cloudguardrails.go`)

`AsyncQueryPollInterval` (500ms) and `AsyncQueryMaxWait` (55s) constants for Athena and CloudWatch Logs Insights polling loops.

### projection (`projection.go`)

Applies a field-path projection to JSON objects before they become dataframe rows, so a deeply nested API response can be flattened to the columns of interest.

### options (`options.go`)

Common `ConnectorOptions` struct injected into every connector at construction time, carrying the `egress.Guard`, row limit, response size cap, and other global settings.

## Adding a new connector

1. Create `connectors/<type>.go` implementing `connections.Connector`.
2. Add a new `ConnectionType` constant to `internal/domain/models.go`.
3. Register the connector in `cmd/server/main.go`.
4. Add config-validation tests in `connectors/<type>_test.go`.
5. Update `internal/catalog/seed.go` if the type maps to any catalog entry.

See the Developer Guide for a step-by-step walkthrough.

## Limitations and todos

- [ ] `EnsureReadOnlySQL` is a keyword guard, not a full SQL parser — sophisticated SQL injection via comment tricks is possible; it is defense-in-depth, not the primary access control.
- [ ] Object storage connectors cap single-object reads at `MaxObjectBytes` (50 MB); no streaming path for larger objects.
- [ ] Async polling for Athena/CloudWatch is blocking inside the `Execute` call; a very slow query blocks the request goroutine for up to 55s.
- [ ] No connector-level connection pooling; each `Execute` call creates a fresh SDK client or database connection.
- [ ] DynamoDB `Scan` on large tables with no filter will consume substantial read capacity; no server-side safeguard beyond the row limit.
