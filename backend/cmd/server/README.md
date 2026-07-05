# cmd/server

## What this package does

`cmd/server` is the application entrypoint. It is the **only** place allowed to import every other package and wire them together. It initialises the dependency graph in the correct order, starts the HTTP server and the in-process scheduler, and handles OS signals for graceful shutdown.

## Architecture

```
main()
  │
  ├─ config.Load()                    ← reads env vars, validates required keys
  ├─ platform/dbx.NewPool()           ← pgx connection pool
  ├─ platform/migrator.Run()          ← applies embedded SQL migrations
  ├─ platform/logger.New()            ← slog JSON logger
  │
  ├─ build service graph (DI, no reflection)
  │    auth.Service, audit.Service, connections.Service,
  │    workflow.Service, catalog.Service, users.Service
  │
  ├─ api.NewRouter(services…)         ← chi router + middleware chain
  ├─ scheduler.New(workflow.Service)  ← in-process cron poll loop
  │
  ├─ http.Server.ListenAndServe()
  └─ os.Signal (SIGINT/SIGTERM) → graceful shutdown (drain in-flight requests + stop scheduler)
```

## Design decisions (ADRs)

| Decision | Rationale |
|---|---|
| Single binary, no separate worker | Simplicity; synchronous execution bounded by `MaxExecutionDuration`. One process to deploy, monitor, and restart. |
| No `init()`, no package-level state | All mutable state is constructor-injected. This makes the wiring explicit and keeps every package individually testable. |
| Graceful shutdown with signal handling | Prevents request truncation during rolling deploys; the scheduler is stopped before the HTTP server to avoid orphaned runs. |
| Migrations run on boot | Eliminates the "schema drift" class of production incident; the binary always brings its own schema up to date. |

## Scope and responsibilities

- Parse and validate environment configuration via `internal/config`.
- Construct the `pgx` connection pool.
- Run embedded database migrations.
- Instantiate every service with its dependencies (repository, encryptor, registry, etc.).
- Build and start the HTTP router (middleware chain + route table).
- Start the background scheduler.
- Listen on `HTTP_PORT` (default `8080`).
- Register SIGINT/SIGTERM handlers; drain in-flight requests and stop the scheduler cleanly.

## What it is NOT responsible for

- Any business logic. If you find business logic here, move it to the appropriate service.
- Defining routes or middleware. Those live in `internal/api/`.
- Defining service behaviour. Services live in their own packages.

## Limitations and todos

- [ ] No TLS termination; production deployments are expected to use a reverse proxy (nginx, ALB, Cloudflare) in front.
- [ ] No hot-reload of config; changing an env var requires a process restart.
- [ ] `MaxExecutionDuration` (2 min) is hard-coded; consider making it configurable per-workflow in a future release.
- [ ] The scheduler is single-instance; running multiple replicas requires an external leader-election mechanism or a distributed lock to avoid duplicate cron executions.
