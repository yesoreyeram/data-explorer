# Design: Serverless Mode

**Status:** Proposal
**Author:** —
**Related:** [FR-06 Ad-Hoc Data Exploration](../functional-requirements/FR-06-ad-hoc-exploration.md),
[ARCHITECTURE.md](../ARCHITECTURE.md), [SECURITY.md](../SECURITY.md)

## 1. Summary

Serverless mode is a second, radically stripped-down way to run Data
Explorer: a **static frontend** (Explore page only) plus a **stateless query
executor** deployable to serverless platforms (AWS Lambda, Vercel Functions,
Azure Functions, Cloudflare Workers/Containers, Google Cloud Run). It serves
**unauthenticated users** who want quick, throwaway data exploration —
paste a DSN or an API URL, run a query, look at a dataframe, leave.

The frontend and the executor are hosted and deployed **independently**. The
frontend can be pointed at a **cluster of executor URLs** and either
**round-robins** across them or lets the user **manually pin** one for the
current session/execution.

Nothing in full mode changes. Serverless mode is a different entrypoint and
a different frontend build profile of the same codebase — not a fork.

## 2. Goals

- **G1 — Zero-friction exploration.** No login, no registration, no
  PostgreSQL, no setup: open the page, run a query, see a dataframe.
- **G2 — Explore only.** Ship exactly the temporary-connection half of the
  Explore surface (FR-06). No saved connections, no workflows, no
  scheduling, no user management, no audit UI.
- **G3 — Independent hosting.** The static frontend and the query executor
  are separate deployables with no shared infrastructure. Frontend on any
  static host (Vercel, Cloudflare Pages, GitHub Pages, S3+CloudFront);
  executor on any serverless compute.
- **G4 — Executor clusters.** The frontend accepts a configured list of
  executor endpoints and supports two selection strategies: automatic
  round-robin, or manual selection (a picker in the Explore toolbar, chosen
  before/during execution).
- **G5 — One codebase.** The executor is the existing Go backend minus the
  stateful parts; the frontend is the existing SPA with a mode switch. All
  connector, dataframe, and guardrail code is reused as-is.

## 3. Non-goals

- Persistence of any kind in serverless mode: no saved connections, no
  workflows, no users, no server-side query history. (Recent queries live
  in the browser's `localStorage` only.)
- Authentication/authorization in serverless mode. If you need RBAC or an
  audit trail, run full mode.
- Multi-tenant SaaS hardening. Serverless mode is intended for personal
  deployments, demos, docs playgrounds, and team sandboxes — not as a
  public query proxy for the internet at large (see §8, which exists
  precisely because people will do this anyway).
- Executor-side load balancing or service discovery. The "cluster" is a
  static client-side list; anything smarter belongs in a real LB.

## 4. Background: what we can reuse

The existing architecture already isolates almost everything serverless
mode needs:

- The **ad-hoc explore path** (`POST /api/v1/explore/query` with an inline
  `connection` object) never touches PostgreSQL: credentials arrive in the
  request body, are used in-memory, and are never persisted
  (`internal/api/handlers/explore.go`, `connections.Service.QueryAdhoc`).
- **Connectors** (`internal/connections/connectors/*`) are stateless given
  a config + secret map, and already enforce guardrails (row caps,
  response-size caps, redirect limits, SQL guard, cloud guardrails).
- `pkg/dataframe` and `pkg/httpclient` are standalone libraries with no
  `internal/*` imports.
- The **integration catalog** (`internal/catalog`) is static in-process
  data — no DB — so it can ship in serverless mode to prefill the
  temporary-connection form.
- The frontend Explore page already supports the temporary-connection
  mode (FR-06.1) with per-type query editors and the dataframe grid.

What serverless mode must *remove*: the pgx pool, migrations, auth/JWT,
RBAC middleware, audit repository, workflows, and the scheduler poll loop.
What it must *add*: an unauthenticated request path with its own guardrail
posture (§8), platform adapters (§7), and the frontend cluster/config layer
(§6).

## 5. Backend: the serverless query executor

### 5.1 Shape

A new entrypoint, `backend/cmd/serverless`, that wires a reduced router:

```
GET  /api/v1/healthz              liveness (already DB-free in this path)
GET  /api/v1/serverless/info      capability manifest (see 5.3)
GET  /api/v1/catalog              static integration catalog (optional, flag-gated)
POST /api/v1/explore/query        ad-hoc query execution — inline connection ONLY
```

Everything else from `internal/api/router.go` is absent — not disabled,
absent. There is no `/auth/*`, no `/connections`, no `/workflows`, no
`/audit-logs`, so the attack surface is exactly the four routes above.

Rather than forking the router, `internal/api/router.go` gains a second
constructor (e.g. `NewServerlessRouter(cfg, h)`) that composes the same
middleware primitives (RequestID, Recover, SecurityHeaders, CORS,
AccessLog, rate limiting) minus `Authenticate`/`RequirePermission`, and a
`Handlers` construction path that takes **no** DB pool, no auth service,
no audit repository. The audit writer is replaced by a stdout structured-log
sink (same fields, `slog` instead of an `audit_logs` row) so platform log
drains (CloudWatch, Vercel logs, Workers tail) still capture who-ran-what
at the IP/request-id level.

`ExploreQuery` in serverless mode:

- **Rejects `connectionId`** with `400 saved connections are not available
  in serverless mode` — there is no store to load one from.
- Skips the `connections:test` permission check (there is no principal);
  the equivalent risk control is the egress policy in §8.
- Runs the identical `QueryAdhoc` path, so every existing guardrail
  (10K-row cap, 25MB response cap, redirect limit, SQL guard, per-request
  timeout) applies unchanged.

### 5.2 Configuration

Twelve-factor env vars, extending `internal/config` with a
`ServerlessConfig` block (only read by `cmd/serverless`):

| Variable | Default | Purpose |
| --- | --- | --- |
| `SERVERLESS_ALLOWED_CONNECTOR_TYPES` | `postgres,mysql,rest,graphql` | Comma-separated allowlist of connector types the executor will run. Cloud connectors are **excluded by default** (see §8.2). |
| `SERVERLESS_ALLOW_AMBIENT_CREDENTIALS` | `false` | Whether cloud connectors may fall back to the platform's ambient identity (Lambda role, managed identity). **Must stay `false` on unauthenticated deployments.** |
| `SERVERLESS_EGRESS_POLICY` | `public-only` | `public-only` (deny RFC1918/link-local/loopback/metadata targets), `allowlist` (only `SERVERLESS_EGRESS_ALLOWLIST` hosts), or `unrestricted` (trusted private deployments only). |
| `SERVERLESS_EGRESS_ALLOWLIST` | — | Comma-separated host[:port] patterns when policy is `allowlist`. |
| `SERVERLESS_RATE_LIMIT_RPS` / `_BURST` | `2` / `10` | Per-IP limiter on `/explore/query` (stricter than full mode's general limiter, matching the auth-limiter posture since every call dials out). |
| `SERVERLESS_MAX_ROWS` | `10000` | Row cap, clamped to ≤ full-mode cap. |
| `SERVERLESS_QUERY_TIMEOUT` | `25s` | Per-query timeout; keep below the platform's own invocation timeout so we return a clean error instead of a platform 502. |
| `SERVERLESS_ENABLE_CATALOG` | `true` | Serve `/catalog` for form prefill. |
| `HTTP_ALLOWED_ORIGINS` | *(required)* | CORS allowlist — must name the frontend origin(s) since frontend and executor are on different hosts by design. |

No `DATABASE_URL`, `JWT_SIGNING_KEY`, or `CONNECTION_ENCRYPTION_KEY` is
required or read; `cmd/serverless` must fail fast if `DATABASE_URL` is set,
to catch copy-pasted full-mode env files (a loud failure beats a silently
ignored credential).

### 5.3 Capability manifest: `GET /api/v1/serverless/info`

The frontend cannot assume every executor in a cluster is configured
identically (different regions, different egress policies, different
versions). Each executor self-describes:

```json
{
  "mode": "serverless",
  "version": "1.4.0",
  "region": "eu-west-1",
  "label": "EU (Ireland)",
  "connectorTypes": ["postgres", "mysql", "rest", "graphql"],
  "limits": { "maxRows": 10000, "maxResponseBytes": 26214400, "queryTimeoutMs": 25000 },
  "catalog": true
}
```

This endpoint doubles as the **health check** the frontend uses for the
cluster picker (a reachable executor that answers `/info` is selectable;
one that doesn't is greyed out). `region`/`label` come from
`SERVERLESS_REGION` / `SERVERLESS_LABEL` env vars and give the manual
picker something human to display.

### 5.4 What about the DB-backed connectors from serverless runtimes?

Postgres/MySQL connectors open raw TCP connections. This works natively on
Lambda, Azure Functions, Cloud Run, and Vercel Functions. Cloudflare
Workers historically could not do raw TCP; `connect()` (TCP sockets API)
now covers most cases, but the Go story there is WASM-only (see §7.4).
Regardless of platform, users' databases must be reachable from the
executor (public endpoint, VPC peering for Lambda-in-VPC, or a pooler like
pgbouncer/RDS Proxy — serverless concurrency can exhaust naive Postgres
connection limits, so docs should push poolers for anything beyond demos).

## 6. Frontend: serverless build profile

### 6.1 Mode switch

A single runtime flag drives everything: `appMode: "full" | "serverless"`.
It is resolved in this order:

1. `window.__APP_CONFIG__` from an optional `/config.json` fetched at boot
   (lets one static build be reconfigured per environment without
   rebuilding — important because "host frontend independently" implies
   ops may not control the build pipeline);
2. `VITE_APP_MODE` baked at build time;
3. default `"full"`.

The same config object carries the cluster definition (§6.3).

### 6.2 Routing and chrome in serverless mode

`App.tsx` branches on the mode:

- Routes: `/explore` (also the index redirect target) and `*` → NotFound.
  No `/login`, `/register`, `ProtectedRoute`, or auth-store bootstrap —
  the auth interceptor and refresh logic in `api/client.ts` are not
  installed at all (no `Authorization` header, no `withCredentials`, no
  401-refresh dance).
- `AppShell` renders a reduced shell: logo, theme switcher, the executor
  picker (§6.4), and a "running in serverless mode" hint linking to docs.
  No sidebar entries for connections/workflows/audit/users.
- `ExplorePage` hides the saved/temporary mode toggle (FR-06.1) and
  renders the temporary-connection form only, with the connection-type
  dropdown filtered to the intersection of the *selected executor's*
  `connectorTypes` (from `/info`).
- Recent queries (US-06.5) persist to `localStorage` (they already
  contain no secrets by design — FR-06/US-06.8; **credentials are
  explicitly excluded** from the stored entry, exactly as the full-mode
  page treats temporary credentials).
- CSV/JSON export is already client-side over the returned dataframe —
  unchanged.

Code-splitting note: workflow builder, admin pages, and React Flow are
already route-level components; the serverless bundle should lazy-load
routes so the shipped JS for serverless mode is meaningfully smaller, but
this is an optimization, not a correctness requirement.

### 6.3 Cluster configuration

```jsonc
// /config.json (served next to index.html) — or the VITE_ equivalent at build time
{
  "appMode": "serverless",
  "executors": {
    "strategy": "round-robin",          // "round-robin" | "manual"
    "endpoints": [
      { "id": "us-east", "label": "US East (Lambda)",   "url": "https://abc123.lambda-url.us-east-1.on.aws" },
      { "id": "eu",      "label": "EU (Azure Fn)",      "url": "https://dx-eu.azurewebsites.net" },
      { "id": "edge",    "label": "Edge (CF Worker)",   "url": "https://dx.example.workers.dev" }
    ]
  }
}
```

- `label` is a fallback; once an endpoint's `/info` responds, its
  self-reported `label`/`region`/`connectorTypes` take precedence.
- A single-endpoint list is valid and hides the picker complexity
  (strategy is irrelevant with one endpoint).
- URLs are origins; the frontend appends `/api/v1/...`.

### 6.4 Endpoint selection semantics

A small `executorStore` (Zustand, like the existing stores) owns:

- `endpoints` + per-endpoint health (`unknown | healthy | unhealthy`),
  refreshed by probing `/info` on boot and lazily on failure;
- `strategy` (initialized from config, user-overridable in the picker);
- `selectedId` (manual mode) and `cursor` (round-robin mode).

Behavior:

- **Round-robin** (`strategy: "round-robin"`): each *query execution*
  (`POST /explore/query`) advances the cursor over the **healthy** subset.
  Non-execution calls (`/catalog`, `/info`) use the same current endpoint
  as the most recent/next execution so the form prefill and type filter
  are consistent with where the query will actually run. The result panel
  shows which executor served the run (from `/info` label + response
  header `X-DX-Executor`, added by the executor) so behavior differences
  across a heterogeneous cluster are debuggable.
- **Manual** (`strategy: "manual"` or user picks a specific endpoint in
  the toolbar dropdown): all requests go to the pinned endpoint until the
  user changes it. The dropdown always offers "Automatic (round-robin)"
  as the first entry plus one entry per configured endpoint, satisfying
  "option to either round robin or manually choose the cluster during
  execution".
- **Failover:** on network error / 5xx / timeout in round-robin mode, the
  client marks the endpoint unhealthy, retries **once** on the next
  healthy endpoint, and surfaces both attempts in the error toast if all
  fail. In manual mode there is **no silent failover** — the user pinned
  the endpoint for a reason (e.g. only that region can reach their DB);
  the error suggests switching.
- **Important asymmetry with a load balancer:** because the target
  database/API must be reachable *from the executor*, round-robin across
  regions can make the same query alternately succeed and fail. This is
  inherent to the feature as requested; the per-run executor attribution
  and manual pinning are the mitigations, and docs must call it out.

### 6.5 CORS, not proxying

In full mode the SPA and API share an origin (nginx proxy). In serverless
mode they deliberately do not, so the executor's CORS allowlist
(`HTTP_ALLOWED_ORIGINS`) must include the frontend origin. `*` is
acceptable here — every request is unauthenticated and carries no
cookies — but the default should still be explicit origins to keep
operators in the habit.

## 7. Platform adapters

The executor stays a plain `http.Handler` (chi router). Each platform gets
a thin adapter in `backend/cmd/serverless/`; connector/guardrail code is
untouched.

| Platform | Adapter | Notes |
| --- | --- | --- |
| **AWS Lambda** | `algnhsa` (or `aws-lambda-go-api-proxy`) wrapping the chi handler; deploy behind a **Function URL** (simplest) or API Gateway. `provided.al2023` runtime, single static binary. | First-class: raw TCP OK, VPC attach for private DBs, IAM role exists → ship with `SERVERLESS_ALLOW_AMBIENT_CREDENTIALS=false` and an **execution role with no data permissions** (§8.2). |
| **Vercel Functions** | Vercel's Go runtime exports `func Handler(w http.ResponseWriter, r *http.Request)` — pass straight to the chi router. One catch-all function `api/[...path].go`. | Frontend and executor can be one Vercel project (rewrites) or two independent projects — both supported; the two-project layout is the reference for "hosted independently". |
| **Azure Functions** | [Custom handler](https://learn.microsoft.com/azure/azure-functions/functions-custom-handlers): the Go binary is an HTTP server; `host.json` routes all traffic to it. | Effectively zero adapter code — `cmd/serverless` already listens on `FUNCTIONS_CUSTOMHANDLER_PORT` when present. Disable managed identity on the Function App or keep ambient creds off. |
| **Cloudflare Workers** | Two honest options: (a) **Workers with Go compiled to WASM** (`syumai/workers`, TinyGo) — real constraints: TinyGo drops parts of `reflect`/`net`, which `pgx`, the AWS SDK, and `jsonata-go` lean on, so the WASM build would support **rest/graphql only** (via `fetch`); (b) **Cloudflare Containers / a container host** running the normal binary. Recommend (b) for full connector support; offer (a) as a REST/GraphQL-only edge profile advertised via `/info.connectorTypes`. | Frontend on **Cloudflare Pages** is fully supported either way. |
| **Google Cloud Run** | None — it runs the container as-is. | Included because it's the least-friction "serverless" for a plain Go HTTP binary; also the fallback recommendation whenever a platform's constraints bite. |

The capability manifest (§5.3) is what makes this heterogeneity tolerable:
a cluster can mix a full-connector Lambda with a REST-only Workers WASM
deployment, and the frontend filters the connection-type dropdown per
selected executor.

Deliverables per platform: a `deploy/serverless/<platform>/` directory with
IaC/config (SAM or plain `Makefile` for Lambda, `vercel.json`,
`host.json` + `local.settings.json` template, `wrangler.toml`) and a
README section each.

## 8. Security model (the hard part)

Full mode's security story leans on authentication, RBAC, and audit.
Serverless mode removes all three **and** hands anonymous users a service
whose core feature is "dial an arbitrary network target with credentials I
supply". That is an SSRF-and-abuse machine unless constrained. These
controls are **requirements, not suggestions**:

### 8.1 Egress policy (SSRF)

A new shared guardrail in `pkg/httpclient` + the SQL connectors' dialers,
enforced at **dial time on every resolved IP** (not just on the hostname,
to defeat DNS rebinding), governed by `SERVERLESS_EGRESS_POLICY`:

- `public-only` (default): reject loopback, RFC1918, link-local
  (169.254.0.0/16 — which covers cloud metadata endpoints — plus
  IPv6 equivalents: `::1`, `fc00::/7`, `fe80::/10`), and the executor's
  own address. Re-checked on every redirect hop.
- `allowlist`: only explicitly listed hosts, for curated playground
  deployments (e.g. a docs demo that only lets you query a sample DB).
- `unrestricted`: opt-in escape hatch for executors deployed *inside* a
  private network on purpose (e.g. Lambda in a VPC exploring internal
  services, protected by network-level access to the Function URL).

### 8.2 Ambient cloud identity

In full mode, "no static keys, use the runtime's IAM role / managed
identity" is a feature. In unauthenticated serverless mode it is a
privilege-escalation hole: any visitor could run Athena/S3/BigQuery
queries **as the executor's own identity**. Therefore:

- Cloud connector types are **off by default**
  (`SERVERLESS_ALLOWED_CONNECTOR_TYPES` default excludes `aws,gcp,azure`).
- Even when enabled, ambient-credential resolution is disabled unless
  `SERVERLESS_ALLOW_AMBIENT_CREDENTIALS=true`; the connector must receive
  explicit user-supplied static credentials or fail with a clear error.
- Deployment templates ship with **zero-privilege execution roles** so a
  misconfiguration has nothing to steal.

### 8.3 Abuse and resource controls

- Per-IP rate limiting (§5.2) using the existing `NewIPRateLimiter`,
  behind the platform's forwarded-for handling (trust the platform's
  canonical client-IP header — `CF-Connecting-IP`,
  `x-vercel-forwarded-for`, API Gateway source IP — never a bare
  user-supplied `X-Forwarded-For`).
- All existing per-request guardrails (row cap, 25MB body cap, redirect
  cap, SQL guard, timeout) remain; serverless config can only tighten
  them, never exceed full-mode caps.
- Security headers middleware unchanged; `/explore/query` responses are
  `Cache-Control: no-store`.
- Structured access + query logs to stdout (platform log drain) with
  request id, client IP, connector type, target host (never credentials,
  never query text at info level) — the serverless stand-in for the audit
  trail.

### 8.4 Credentials in flight

Unchanged from FR-06/US-06.8: temporary credentials travel in the request
body over TLS, live only for the request, and are never logged or stored.
Serverless mode adds one nuance: the browser now sends them to whichever
executor is selected, so the picker UI must always make the target visible
(§6.4) — a user pasting production credentials should be able to see, at
a glance, which deployment is about to receive them. The docs must state
that operators of the executor are trusted by its users.

## 9. Build, packaging, repo layout

```
backend/
  cmd/server/          full mode (unchanged)
  cmd/serverless/      serverless entrypoint + platform adapters
  internal/api/        + NewServerlessRouter, serverless Handlers wiring
  internal/config/     + ServerlessConfig
  pkg/httpclient/      + egress policy dialer (also usable by full mode later)
deploy/serverless/
  lambda/  vercel/  azure-functions/  cloudflare/  cloudrun/
frontend/
  (same app; /config.json runtime config + appMode branching)
docs/
  design/serverless-mode.md   (this document)
  functional-requirements/FR-17-serverless-mode.md  (follow-up, user-facing)
```

CI additions: build `cmd/serverless` for `linux/amd64` +
`linux/arm64`, plus the TinyGo/WASM profile if/when the Workers-native
option ships; a smoke test that boots `cmd/serverless` with no
`DATABASE_URL` and exercises `/info` + a REST ad-hoc query against a local
fixture server; a frontend test that boots the SPA with a serverless
`config.json` and asserts route/UI reduction and round-robin/pinning
behavior.

## 10. Phasing

1. **Phase 1 — Executor core.** `cmd/serverless`, `NewServerlessRouter`,
   `ServerlessConfig`, egress policy, ambient-credential lockout, `/info`.
   Runs as a plain container (Cloud Run/anything). This alone is testable
   end-to-end with the existing frontend pointed at it via `VITE_API_URL`.
2. **Phase 2 — Frontend serverless profile.** `appMode`, reduced
   routes/shell, `config.json` loader, single-endpoint support.
3. **Phase 3 — Clusters.** `executorStore`, `/info` probing, round-robin +
   manual picker, failover, per-run executor attribution.
4. **Phase 4 — Platform adapters & templates.** Lambda, Vercel, Azure
   Functions, Cloudflare (containers first, WASM profile as stretch),
   with `deploy/serverless/*` templates and docs.
5. **Phase 5 — FR-17.** Promote the user-facing behavior into a proper
   FRD with acceptance criteria and screenshots, per the FRD conventions.

## 11. Open questions

1. **Public-demo posture:** do we want a first-party hosted playground
   (allowlist egress + sample datasets)? It's the best onboarding funnel
   but makes us the operator of §8's risks.
2. **Optional shared-secret gate:** a single static bearer token
   (`SERVERLESS_ACCESS_TOKEN`) would let teams deploy "unauthenticated but
   not public" executors with one env var. Cheap to add; slightly muddies
   the "no auth" story. Leaning yes, off by default.
3. **Workers-native (WASM) profile:** worth the TinyGo maintenance burden,
   or do we document Cloudflare = Pages(frontend) + Containers(executor)
   and revisit when demand shows up? Leaning the latter for v1.
4. **Result-size economics:** serverless egress is billed; do defaults
   need to be lower than full mode's 25MB/10K rows (e.g. 5MB/2K) with the
   caps advertised via `/info`? Leaning yes for the shipped templates.
5. **Round-robin scope:** current design advances per *execution*. An
   alternative is per *session* (sticky until failure), which behaves
   better against heterogeneous clusters. The picker's "Automatic" mode
   could later grow a sticky variant without an API change.
