# Design: Serverless Mode & Deployment-Profile Architecture

**Status:** Proposal (v2 — expanded security architecture, greenfield system redesign)
**Author:** —
**Related:** [FR-06 Ad-Hoc Data Exploration](../functional-requirements/FR-06-ad-hoc-exploration.md),
[ARCHITECTURE.md](../ARCHITECTURE.md), [SECURITY.md](../SECURITY.md)

---

## 1. Summary

Serverless mode is a second way to run Data Explorer: a **static frontend**
(Explore only) plus a **stateless query executor** deployable to serverless
platforms (AWS Lambda, Vercel Functions, Azure Functions, Cloudflare,
Google Cloud Run). It serves **unauthenticated users** who want quick,
throwaway data exploration — paste a DSN or an API URL, run a query, look
at a dataframe, leave. The frontend and executor are hosted and deployed
**independently**; the frontend can be configured with a **cluster of
executor URLs** and either **round-robins** across them or lets the user
**manually pin** one during execution.

Because the project is greenfield, this document does not bolt serverless
mode onto the current monolith. It proposes a **deployment-profile
architecture**: a single execution kernel behind explicit ports, composed
into profiles (*enterprise server*, *serverless executor*, and later
*hybrid*) by swapping adapters — so the unauthenticated profile is a
**subtraction of adapters, never an `if mode == serverless` branch inside
security-relevant code**. Security is the organizing concern throughout:
removing authentication from a product whose core feature is "dial an
arbitrary network target with caller-supplied credentials" is only safe if
the remaining controls are structural.

---

## 2. Goals and non-goals

### Goals

- **G1 — Zero-friction exploration.** No login, no registration, no
  database, no setup: open a URL, run a query, see a dataframe.
- **G2 — Explore only.** Exactly the temporary-connection half of FR-06.
  No saved connections, workflows, scheduling, users, or audit UI.
- **G3 — Independent hosting.** Static frontend on any static host
  (Vercel, Cloudflare Pages, S3+CloudFront, GitHub Pages); executor on any
  serverless compute. No shared infrastructure, different lifecycles.
- **G4 — Executor clusters.** The frontend accepts a configured list of
  executor endpoints with two selection strategies: automatic round-robin,
  or manual selection in the Explore toolbar at execution time.
- **G5 — One codebase, profile composition.** Connectors, dataframe, and
  guardrails are written once. Profiles differ only in which adapters are
  wired at the entrypoint.
- **G6 — Enterprise-grade posture in both profiles.** The redesign must
  leave the authenticated product *stronger* (pluggable identity, policy
  engine, distributed limits), not merely make the unauthenticated one
  possible.

### Non-goals

- Persistence of any kind in the serverless profile — no saved
  connections, workflows, users, or server-side history. Recent queries
  live in the browser (`localStorage`) only, and never contain secrets.
- Authentication/authorization in the serverless profile (an optional
  shared-secret gate exists — §8.6 — but is not an identity system).
- Making the serverless profile a safe **open proxy for the public
  internet**. It is designed for personal deployments, demos, docs
  playgrounds, and team sandboxes. §8 exists because some deployments
  will be public anyway; the defaults assume hostile traffic.
- Executor-side load balancing or service discovery. The cluster is a
  static, operator-controlled client-side list (§7.4 explains why this is
  a security decision, not a shortcut).

---

## 3. System redesign: the deployment-profile architecture

### 3.1 The problem with the current shape

Today `cmd/server` wires everything concretely: handlers construct services
that assume a pgx pool, an audit repository, JWT auth, and RBAC middleware.
A serverless variant carved out of that with feature flags would scatter
`if serverless` branches through security-critical paths — the classic way
authorization bypasses are born. Greenfield freedom lets us fix the cause:
**the kernel must not know which profile it is running in.**

### 3.2 Kernel and ports (hexagonal)

The backend is reorganized into an execution kernel with explicit ports.
Everything inside the kernel is deterministic, stateless, and
profile-agnostic; everything environmental enters through an interface.

```
backend/
  pkg/                      # standalone libs (unchanged philosophy)
    dataframe/
    httpclient/             # + EgressGuard hook (dial-time policy, §8.2)
  internal/
    kernel/                 # profile-agnostic core
      connectors/           # postgres, mysql, rest, graphql, aws, gcp, azure
      queryexec/            # QuerySpec validation, guardrail application,
                            #   connector dispatch (today: connections.Service.QueryAdhoc)
      catalog/              # static integration catalog (no DB)
      guardrails/           # row caps, size caps, sqlguard, cloudguardrails
    ports/                  # interfaces + shared types ONLY
      identity.go           #   IdentityProvider, Principal
      policy.go             #   PolicyEngine (authorization decisions)
      store.go              #   MetadataStore (connections/workflows/users CRUD)
      secrets.go            #   SecretStore (encrypt-at-rest boundary)
      audit.go              #   AuditSink
      limits.go             #   RateLimiter, QuotaStore
      egress.go             #   EgressGuard (network policy, §8.2)
      telemetry.go          #   Metrics/Tracing
    adapters/
      identity/ (jwtlocal, oidc*, anonymous)
      policy/   (rbac, static)          # static = fixed allow/deny table
      store/    (postgres, none)        # none = every call -> ErrNotSupported
      secrets/  (aesgcm+pg, ephemeral)  # ephemeral = in-request only, no persistence
      audit/    (postgres, slog)        # slog = structured stdout for log drains
      limits/   (memory, redis*)        # redis for horizontally-scaled full mode
      egress/   (unrestricted, publiconly, allowlist)
      telemetry/(prometheus, otel*)
    profiles/
      server/               # enterprise: jwtlocal|oidc + rbac + postgres + aesgcm + postgres-audit
      serverless/           # anonymous + static-policy + none-store + ephemeral + slog-audit
    transport/
      httpapi/              # handlers + middleware, built against ports only
  cmd/
    server/                 # profiles/server + net/http
    serverless/             # profiles/serverless + platform adapters (§9)
```

(* = later phase, enabled by the port existing now.)

Key properties:

- **Handlers depend on ports, never adapters.** `ExploreQuery` asks
  `PolicyEngine.Authorize(principal, ActionExploreAdhoc)`; it cannot tell
  whether the answer came from RBAC-over-JWT or from the serverless static
  table. There is no "skip auth" code path to get wrong — the anonymous
  identity adapter *always* produces a Principal, and the static policy
  *always* evaluates it.
- **Absent capabilities are structurally absent.** The serverless profile
  registers only four routes (§4.1). `/auth/*` and `/connections` are not
  "disabled routes returning 403"; the handlers for them are never
  constructed, and the store adapter behind them is `none`. A request to
  them is a 404 from the router, and there is no latent code reachable by
  a middleware-ordering mistake.
- **The enterprise profile gains from the same refactor**: OIDC/SAML SSO
  becomes an identity adapter (closing a known limitation in SECURITY.md),
  Redis-backed limits fix the "per-instance rate limiting" caveat, and the
  policy port is where per-connection ACLs or data-masking policy can land
  later without touching handlers.

### 3.3 Deployment profiles

| | **Enterprise server** | **Serverless executor** | **Hybrid** (later) |
| --- | --- | --- | --- |
| Identity | Local JWT / OIDC | Anonymous (+ optional shared secret §8.6) | OIDC, tokens verified statelessly |
| Policy | RBAC | Static table (explore + catalog only) | RBAC claims in token |
| Store | PostgreSQL | none | read-only replica / API |
| Secrets | AES-256-GCM in PG | Ephemeral (request-scoped) | central secrets service |
| Audit | PG append-only | Structured stdout → platform log drain | central sink |
| Egress | `unrestricted` (operator-trusted) | `public-only` default | per-tenant policy |
| State | PG is system of record | **None. Zero.** | split |

The hybrid column is not designed here; it is listed to show that the port
set was chosen so an enterprise team can later run *authenticated* elastic
query executors next to a stateful control plane without another redesign.

### 3.4 Frontend redesign: capability-driven UI

Symmetrically, the SPA stops hardcoding its surface. A single
**AppCapabilities** object — derived from runtime config plus the selected
executor's manifest (§5.3) — drives routing, navigation, and feature
visibility:

```ts
interface AppCapabilities {
  mode: "server" | "serverless";
  auth: boolean;                    // login/register/refresh flows exist
  savedConnections: boolean;
  workflows: boolean;
  audit: boolean;
  users: boolean;
  connectorTypes: ConnectionType[]; // per selected executor in serverless
  limits: ExecutorLimits;           // surfaced in the UI, not just enforced
}
```

Routes, the sidebar, and the Explore mode-toggle all render from this
object. In serverless mode the auth interceptor, refresh logic, and
`withCredentials` in `api/client.ts` are **not installed at all** — no
`Authorization` header is ever attached, so nothing sensitive can leak to a
misconfigured executor via headers. This is the client-side twin of "absent,
not disabled": UI hiding remains a UX nicety, but here it also shrinks the
shipped bundle (route-level code splitting: React Flow, admin pages, and
the workflow builder never load in serverless builds).

---

## 4. The serverless query executor

### 4.1 Surface

`cmd/serverless` registers exactly:

```
GET  /api/v1/healthz              liveness
GET  /api/v1/serverless/info      capability manifest (§4.3) — doubles as readiness
GET  /api/v1/catalog              static integration catalog (flag-gated)
POST /api/v1/explore/query        ad-hoc execution — inline connection ONLY
```

Middleware chain (same primitives as full mode, composed differently):
RequestID → Recover → SecurityHeaders → CORS → AccessLog →
ClientIP-resolution (§8.5) → RateLimit/Quota → optional SharedSecret (§8.6)
→ handler. There is no Authenticate/RequirePermission pair; authorization
is the static policy adapter, which permits exactly the four actions above.

`ExploreQuery` behavior deltas vs. full mode:

- `connectionId` is **rejected** (`400 saved connections are not available
  in serverless mode`) — the `none` store adapter has nothing to load, but
  we reject explicitly for a clear message rather than surfacing a store
  error.
- The inline-connection permission check maps to the static policy (always
  allowed if the connector type is enabled); the compensating control is
  the egress policy plus connector allowlist (§8.2–8.3).
- The identical `queryexec` path runs, so every existing guardrail (row
  cap, 25MB response cap, redirect cap, SQL read-only guard, pagination
  caps, cell truncation, timeouts) applies unchanged — they live in the
  kernel, not in the profile.

### 4.2 Configuration

Twelve-factor env vars (read only by the serverless profile):

| Variable | Default | Purpose |
| --- | --- | --- |
| `SERVERLESS_ALLOWED_CONNECTOR_TYPES` | `postgres,mysql,rest,graphql` | Connector allowlist. **Cloud types excluded by default** (§8.3). |
| `SERVERLESS_ALLOW_AMBIENT_CREDENTIALS` | `false` | Cloud connectors may use platform identity. Must stay `false` on unauthenticated deployments (§8.3). |
| `SERVERLESS_EGRESS_POLICY` | `public-only` | `public-only` \| `allowlist` \| `unrestricted` (§8.2). |
| `SERVERLESS_EGRESS_ALLOWLIST` | — | host[:port] patterns for `allowlist`. |
| `SERVERLESS_RATE_LIMIT_RPS` / `_BURST` | `2` / `10` | Per-client-IP limit on `/explore/query`. |
| `SERVERLESS_DAILY_QUERY_QUOTA` | `500` | Per-IP rolling daily cap (§8.5). `0` disables. |
| `SERVERLESS_MAX_CONCURRENCY` | `8` | In-process concurrent execution cap (excess → `429` + `Retry-After`). |
| `SERVERLESS_MAX_ROWS` | `2000` | Row cap; clamped to ≤ kernel hard cap (10K). Lower than full mode: egress is billed (§10.4). |
| `SERVERLESS_MAX_RESPONSE_BYTES` | `5242880` (5MB) | Upstream response cap; clamped to ≤ kernel 25MB. |
| `SERVERLESS_QUERY_TIMEOUT` | `25s` | Per-query timeout; keep under the platform invocation timeout for clean errors instead of platform 502s. |
| `SERVERLESS_ACCESS_TOKEN` | — | Optional shared-secret gate (§8.6). |
| `SERVERLESS_ENABLE_CATALOG` | `true` | Serve `/catalog`. |
| `SERVERLESS_REGION` / `SERVERLESS_LABEL` | — | Advertised in the manifest for the cluster picker. |
| `SERVERLESS_KILL_SWITCH` | `false` | Refuse all `/explore/query` with a static 503 message (§8.11). |
| `HTTP_ALLOWED_ORIGINS` | *(required)* | CORS allowlist — must name the frontend origin(s). |

**Fail-closed startup validation:** the executor refuses to boot if
`DATABASE_URL`, `JWT_SIGNING_KEY`, or `CONNECTION_ENCRYPTION_KEY` is set
(catches copy-pasted full-mode env files — a loud failure beats a silently
carried credential); if the egress policy is `allowlist` with an empty
allowlist; or if ambient credentials are enabled while the shared-secret
gate is absent **and** `SERVERLESS_I_UNDERSTAND_PUBLIC_AMBIENT_RISK=true`
is not also set. Dangerous combinations must be typed deliberately.

### 4.3 Capability manifest — `GET /api/v1/serverless/info`

Executors in a cluster are legitimately heterogeneous (regions, connector
sets, versions, egress policies). Each self-describes:

```json
{
  "mode": "serverless",
  "apiVersion": "v1",
  "version": "1.4.0",
  "region": "eu-west-1",
  "label": "EU (Ireland)",
  "connectorTypes": ["postgres", "mysql", "rest", "graphql"],
  "egressPolicy": "public-only",
  "authRequired": false,
  "limits": { "maxRows": 2000, "maxResponseBytes": 5242880, "queryTimeoutMs": 25000,
              "rateLimitRps": 2, "dailyQuota": 500 },
  "catalog": true
}
```

This is the health probe for the cluster picker, the source of the
connection-type filter, and the way the UI shows users the limits and
egress posture of the executor **before** they paste credentials into it.
`apiVersion` is the SPA↔executor compatibility contract: the frontend
refuses (with a clear message) executors whose major version it does not
speak, so a mixed-version cluster degrades explicably rather than
mysteriously. The manifest never echoes secrets or internal hostnames.

### 4.4 Databases from serverless runtimes

Postgres/MySQL connectors need raw TCP. Native on Lambda, Azure Functions,
Cloud Run, Vercel Functions; on Cloudflare Workers the Go story is
WASM-only with heavy stdlib constraints (§9). Users' databases must be
reachable *from the executor* (public endpoint, VPC attachment, or a
pooler). Serverless concurrency can exhaust naive Postgres connection
limits — deployment docs push pgbouncer/RDS Proxy for anything beyond
demos, and the executor opens at most one upstream connection per request
and closes it before responding (no cross-invocation pooling to leak state
between anonymous users; see §8.8).

---

## 5. Frontend: serverless profile

### 5.1 Runtime configuration

Resolution order for `AppCapabilities.mode` and the cluster definition:

1. `/config.json` fetched at boot from the frontend's **own origin** (one
   static build reconfigurable per environment — integrity implications in
   §8.7);
2. `VITE_APP_MODE` / `VITE_EXECUTORS` baked at build time;
3. default: `server` mode.

```jsonc
// /config.json — served next to index.html, same origin, no third-party host
{
  "appMode": "serverless",
  "executors": {
    "strategy": "round-robin",          // "round-robin" | "manual"
    "endpoints": [
      { "id": "us-east", "label": "US East (Lambda)", "url": "https://abc123.lambda-url.us-east-1.on.aws" },
      { "id": "eu",      "label": "EU (Azure Fn)",    "url": "https://dx-eu.azurewebsites.net" },
      { "id": "edge",    "label": "Edge (CF)",        "url": "https://dx.example.workers.dev" }
    ]
  }
}
```

`label` is a fallback; a reachable endpoint's own manifest wins. A
single-endpoint list is valid and hides the picker. URLs are origins; the
client appends `/api/v1/...`. **`https://` is mandatory** — the config
loader rejects `http://` endpoints except `localhost` (§8.1).

### 5.2 Endpoint selection semantics

A Zustand `executorStore` owns: `endpoints` + per-endpoint health
(`unknown | healthy | unhealthy`, probed via `/info` at boot and lazily on
failure), `strategy`, `selectedId` (manual), `cursor` (round-robin).

- **Round-robin:** each `POST /explore/query` advances the cursor over the
  **healthy** subset. Non-execution calls (`/catalog`, `/info`) target the
  endpoint the *next* execution will use, so form prefill and type
  filtering match where the query actually runs. Every result panel shows
  which executor served the run (manifest label + `X-DX-Executor` response
  header) — essential for debugging heterogeneous clusters.
- **Manual:** all requests go to the pinned endpoint until changed. The
  toolbar dropdown lists "Automatic (round-robin)" first, then each
  endpoint with label, region, health, and connector set — satisfying
  "round robin or manually choose the cluster during execution".
- **Failover:** in round-robin, on network error / 5xx / timeout the
  client marks the endpoint unhealthy and retries **once** on the next
  healthy endpoint; both attempts appear in the error surface if all fail.
  In manual mode there is **no silent failover** — the user pinned the
  endpoint deliberately (often because only that region can reach their
  database, or because they only trust that operator with these
  credentials — §8.7); the error suggests switching instead.
- **Inherent asymmetry vs. a load balancer:** the target database/API must
  be reachable *from the executor*, so round-robin across regions can make
  the same query alternately succeed and fail. Per-run attribution and
  manual pinning are the mitigations; docs must call it out.

### 5.3 Explore page in serverless mode

- Temporary-connection form only (saved/temporary toggle hidden via
  capabilities); connection-type dropdown filtered to the selected
  executor's `connectorTypes`.
- Recent queries persist to `localStorage` **excluding credentials**
  (identical discipline to full mode's US-06.8); an explicit "clear
  history" control and a note that history is device-local.
- CSV/JSON export stays fully client-side over the returned dataframe.
- The executor's limits (rows, bytes, timeout) from the manifest are
  displayed near the Run button, so truncation never surprises.

---

## 6. Trust model

Naming the parties before enumerating threats:

| Party | Trusts | Must NOT need to trust |
| --- | --- | --- |
| **End user** | The frontend operator (serves the JS) and the **selected** executor operator (receives pasted credentials in-flight). | Other executors in the cluster they haven't selected/used; other users of the same executor. |
| **Frontend operator** | Their static host; the executor list they configure. | The executors' *availability* (cluster + failover exists for this). |
| **Executor operator** | Their serverless platform. | End users (assumed hostile), targets users point the executor at (assumed hostile), the frontend (executor revalidates everything server-side). |
| **Target systems** | — | The executor is an *unauthenticated relay* toward them; egress policy + guardrails bound what it can be made to do (§8.2). |

Trust boundaries: browser ↔ executor (TLS, CORS, no cookies);
executor ↔ target (egress policy); browser ↔ static host (CSP/SRI/config
integrity); executor ↔ its own cloud platform (ambient-identity lockout).
The single most important sentence for documentation and UI copy: **users
paste live credentials into a request that goes to whichever executor is
selected — the picker must always make that destination visible, and
executor operators are trusted by their users.**

---

## 7. Threat model

Assets: (A1) user-pasted credentials in flight; (A2) the executor's own
platform identity and network position; (A3) target systems reachable from
the executor; (A4) the operator's bill/quota; (A5) integrity of the
frontend code and config; (A6) result data in the browser.

STRIDE over the serverless profile:

| Threat | Vector | Controls |
| --- | --- | --- |
| **Spoofing** | Rogue executor added to config; DNS takeover of an executor host; phishing clone of the frontend | Operator-controlled static config only, no user-supplied executor URLs (§7.4); HTTPS-only endpoints; optional manifest signing (§8.7); custom domains + CAA records in deployment docs |
| **Tampering** | Modified JS bundle or `config.json` redirecting credentials | CSP, SRI, same-origin config, signed releases, immutable static hosting (§8.7, §8.9) |
| **Repudiation** | Anonymous abuse with no trail | Structured security event log to platform drain: request id, client IP, connector type, target host, outcome — never credentials or query text (§8.10) |
| **Information disclosure** | SSRF reads internal services/metadata; error messages leak internals; credentials logged | Dial-time egress guard incl. DNS-rebinding defense (§8.2); classified errors only (existing `Classify` discipline); log redaction (§8.10); no persistence at all |
| **Denial of service / economic DoS** | Query floods; slow-loris targets; amplification via pagination; unbounded platform bills | Rate limits + daily quotas + concurrency caps (§8.5); kernel guardrails (existing); platform budget alarms + reserved-concurrency caps in templates; kill switch (§8.11) |
| **Elevation of privilege** | Ambient cloud identity used via ad-hoc cloud queries; executor's VPC position used to reach private systems | Ambient-credential lockout + zero-permission execution roles (§8.3); egress `public-only` default (§8.2); cloud connectors off by default |

Two deliberate accepted risks, stated plainly:

1. **The executor is an outbound-request machine by design.** With
   `public-only` egress it can still be used to probe *public* third-party
   endpoints anonymously (attribution laundering). Mitigations: rate
   limits/quotas make it a poor attack proxy; an outbound `User-Agent:
   DataExplorer-Serverless/<version> (+docs-url)` header identifies the
   relay honestly to targets; `allowlist` mode exists for curated
   deployments; abuse-contact guidance ships in the templates. This risk
   is why the default limits are conservative.
2. **A malicious executor operator sees user-pasted credentials.** This is
   irreducible in the requested design (credentials must reach the thing
   that dials out). Mitigations are transparency, not cryptography: the
   picker always shows the destination, `/info` exposes the executor's
   posture, and docs tell users to use scoped, revocable, read-only
   credentials for exploration.

### 7.4 Why the cluster list is operator-controlled only

An earlier idea — letting end users type executor URLs into the UI — is
rejected: it would turn the frontend into a credential-phishing kit ("try
my executor!") and make the trust model unexplainable. The cluster is
defined solely by the frontend operator's config; users choose *among*
vetted endpoints, never add new ones. (A power user who wants their own
executor changes the config of their own frontend deployment — that is the
independence G3 already grants.)

---

## 8. Security architecture

### 8.1 Transport

- Executor endpoints and the frontend are HTTPS-only; the config loader
  rejects `http://` (localhost excepted for development). Serverless
  platforms terminate TLS; the executor additionally sets HSTS when it can
  see it is serving TLS-terminated traffic.
- CORS on the executor is an explicit origin allowlist
  (`HTTP_ALLOWED_ORIGINS`). Requests are cookie-less and
  credential-less, so `*` is not catastrophic — but defaults and docs use
  explicit origins; wildcard requires an explicit opt-in flag, keeping
  operators in the habit that protects full mode.
- `POST /explore/query` responses: `Cache-Control: no-store` (they can
  contain data pulled with private credentials); `Vary: Origin`.
- Existing security-headers middleware applies unchanged (`nosniff`,
  `X-Frame-Options: DENY`, strict CSP on API responses).

### 8.2 Egress control (SSRF defense in depth)

The headline risk. The guard is a new `ports.EgressGuard` enforced **at
dial time inside `pkg/httpclient` and the SQL connectors' dialers** — i.e.
below every connector, on every connection, not as a URL pre-check:

- **Checked per resolved IP, post-DNS.** The custom `DialContext` resolves
  the hostname, validates **every** returned address against policy, and
  then dials **only the validated IP** (pinning), so a DNS answer cannot
  change between check and connect (TOCTOU / DNS-rebinding defense).
  Re-run on every redirect hop and on every pagination request.
- **`public-only` (default) denies:** loopback (`127.0.0.0/8`, `::1`),
  RFC1918, CGNAT (`100.64.0.0/10`), link-local `169.254.0.0/16` (covers
  AWS/Azure/GCP metadata IPs) and `fe80::/10`, unique-local `fc00::/7`,
  unspecified (`0.0.0.0`, `::`), broadcast/multicast, IPv4-mapped IPv6
  forms of all of the above (`::ffff:10.0.0.1` must not bypass the v4
  rules), NAT64 (`64:ff9b::/96`), and the metadata *hostnames*
  (`metadata.google.internal`, etc.) by name as well as by IP.
- **Scheme and port hygiene:** `http`/`https` only for HTTP connectors
  (already enforced); SQL DSNs are parsed and rebuilt from validated
  components — host, port, database, user, password, `sslmode` — rather
  than passed through verbatim, which neutralizes DSN smuggling (e.g.
  `host=/var/run/postgresql` Unix sockets, multi-host DSNs, or
  parameter-injection via unescaped values). Unix-socket and file-ish
  targets are rejected outright in the serverless profile.
- **`allowlist`:** only named host[:port] patterns; evaluated on the
  original hostname *and* the resolved IPs (a listed name that resolves
  privately still needs `unrestricted` — the operator must mean it).
- **`unrestricted`:** for executors deployed inside a private network on
  purpose (e.g. Lambda-in-VPC exploring internal services). Deployment
  templates pair it with network-level access control on the function URL
  and it is prominently flagged in `/info.egressPolicy` so the UI can show
  a "private-network executor" badge.
- **Platform backstop:** templates additionally configure platform-level
  egress control where available (Lambda-in-VPC security groups with no
  route to sensitive subnets; Cloudflare Workers' lack of raw
  IP-addressed fetch works in our favor) — application policy is the
  portable control, platform policy is the belt-and-suspenders.

### 8.3 Cloud identity and privilege containment

In full mode, ambient identity (IAM role / ADC / managed identity) is a
feature. In an unauthenticated executor it is privilege escalation: any
visitor could run Athena/S3/BigQuery **as the executor**. Layered lockout:

1. Cloud connector types (`aws`, `gcp`, `azure`) are **excluded from the
   default allowlist**.
2. Even when enabled, ambient-credential fallback is disabled unless
   `SERVERLESS_ALLOW_AMBIENT_CREDENTIALS=true`; connectors require
   explicit user-supplied static credentials or fail with a clear error.
   This is enforced in the kernel's credential-resolution step, not in
   each connector.
3. Deployment templates ship **zero-permission execution roles** (Lambda
   role with no policies beyond log writing; Function App with managed
   identity disabled; Cloud Run service account with no bindings) — so
   even a bypass of (1)+(2) finds an identity that can do nothing.
4. Templates disable/deny instance metadata where the platform allows, and
   the egress guard blocks metadata endpoints regardless (§8.2).
5. The startup fail-closed rule (§4.2) makes the dangerous combination —
   ambient credentials on, no access gate — impossible to reach without a
   third, explicitly named override variable.

### 8.4 Input validation and request hardening

Unchanged kernel disciplines, now load-bearing without an authn front
door: 1MB request body cap with `DisallowUnknownFields`; strict typed
`QuerySpec` validation; parameterized SQL only; `EnsureReadOnlySQL`
defense-in-depth (still explicitly *not* the primary control — the docs
tell users to paste read-only credentials); response/object/page caps;
cell truncation. Additions for the serverless profile:

- Per-request header allowlist for the REST connector: user-supplied
  header names are validated against RFC 7230 token syntax, and
  hop-by-hop / platform-reserved headers (`Host`, `Connection`,
  `X-Forwarded-*`, platform IP headers) cannot be set — prevents
  request-smuggling games through the relay.
- The executor never forwards the *browser's* headers upstream (no cookie,
  origin, or IP leakage to targets); outbound requests are built from the
  QuerySpec alone plus the honest `User-Agent` (§7).

### 8.5 Anti-abuse: rate, quota, concurrency, economics

- **Client IP resolution is platform-pinned.** Each platform adapter
  declares the *one* trustworthy source (API Gateway/Function URL source
  IP, `CF-Connecting-IP`, `x-vercel-forwarded-for` leftmost-trusted, Azure
  `X-Forwarded-For` per Front Door config). A bare user-supplied
  `X-Forwarded-For` is never trusted; if the adapter cannot determine a
  trustworthy source it falls back to the socket peer and logs a warning.
  Everything below keys off this resolved IP.
- **Token-bucket per-IP rate limit** (default 2 rps / burst 10) on
  `/explore/query` — the strict auth-limiter posture, because every call
  dials out.
- **Daily per-IP quota** (default 500) — rate limits stop bursts, quotas
  stop patient abuse. In-memory per instance by default (honest
  limitation: resets on cold start); the `limits` port allows a shared
  store later without code changes. Deployment templates therefore also
  set **platform-level caps**: Lambda reserved concurrency, API Gateway
  throttling + AWS Budgets alarms, Cloudflare rate-limiting rules, Vercel
  spend limits — the platform is the durable enforcement point for
  economic DoS, the app limiter is the fast one.
- **In-process concurrency cap** (default 8) returning `429` +
  `Retry-After` — protects a single instance from slow-target pile-ups
  (a deliberately slow upstream is attacker-controlled wait time).
- **Optional human gate:** a `SERVERLESS_TURNSTILE_SECRET`-style hook
  (Cloudflare Turnstile / hCaptcha verification before the first query of
  a session) is specified as an optional middleware for genuinely public
  playgrounds. Off by default; not a substitute for limits.

### 8.6 Optional shared-secret gate

`SERVERLESS_ACCESS_TOKEN`: when set, `/explore/query` (and optionally
`/catalog`) require `Authorization: Bearer <token>`, compared in constant
time. The frontend prompts for it once and keeps it **in memory only**
(mirroring the existing access-token discipline — never `localStorage`).
This is deliberately *not* an identity system: one static credential,
no users, no sessions. It converts "public to the internet" into
"unauthenticated but gated" for team sandboxes, and it is the knob that
unlocks riskier server config combinations (§4.2, §8.3). `/info` reports
`authRequired: true` so the picker can show a lock icon.

### 8.7 Frontend integrity and the credential path

The SPA is the thing users type credentials into, so its integrity chain
matters as much as the executor's:

- **CSP for the static frontend** (meta tag + host headers in every
  deployment template): `default-src 'self'; connect-src 'self' <executor
  origins>; script-src 'self'; object-src 'none'; base-uri 'none';
  frame-ancestors 'none'`. No third-party CDN scripts, fonts, or
  analytics in the serverless build — the bundle is fully self-contained,
  so `connect-src` listing only the executors is actually achievable, and
  it doubles as an exfiltration barrier: injected script cannot POST
  credentials anywhere but the vetted executors.
- **SRI:** Vite build emits `integrity` attributes for its own
  entry chunks; deployment docs cover immutable, versioned asset paths.
- **`config.json` is same-origin only** and constrained by CSP; whoever
  can modify it can already modify `index.html`, so it adds no new trust
  root. Its schema is strictly validated; unknown keys rejected; a config
  that fails validation renders an error page, never a permissive default.
- **Optional manifest signing** for high-assurance deployments: config may
  pin a per-endpoint Ed25519 public key; the executor signs its `/info`
  body (`X-DX-Manifest-Signature`). This detects DNS/hosting takeover of
  an executor host *before* a user sends credentials to it. Off by
  default (key distribution is real friction); designed now so the header
  and config slot exist.
- **Credential UX rules:** credential inputs are `type="password"` with
  reveal toggles; never placed in URLs or query strings; excluded from
  recent-query history; cleared from component state on page leave;
  autocomplete disabled. The Run button area always shows the destination
  executor label (§5.2) — the human confirmation of where secrets go.

### 8.8 Runtime and data hygiene

- **Statelessness as a control:** no database, no disk writes, no
  cross-request caches keyed by user data. Upstream connections are opened
  per request and closed before the response returns — no pooled
  connection authenticated with user A's credentials can serve user B.
  OAuth2/token caches in `pkg/httpclient` are request-scoped in the
  serverless profile (the in-memory cross-request cache is a full-mode
  optimization tied to saved connections; adhoc runs must not share one).
- **Per-invocation isolation where the platform gives it** (Lambda,
  Vercel): one request per sandbox at a time is a meaningful memory-safety
  backstop. On Cloud Run, templates default to modest per-instance
  concurrency; `concurrency=1` documented for paranoid deployments.
- **Memory zeroization is explicitly not claimed.** Go's GC gives no
  guarantee; we do not pretend otherwise. The honest controls are: short
  invocation lifetimes, no swap in these sandboxes, no core dumps
  (templates disable them where configurable), and secrets never reaching
  logs or errors (below).
- **Error discipline:** the existing `Classify` machinery returns
  stable codes + remediation text; raw driver/SDK detail is logged
  server-side at debug level, never returned to anonymous callers in
  production (`APP_ENV=production` suppresses `Detail()` in responses).
  Secrets are never part of an error value (existing invariant, kept).

### 8.9 Supply chain and build integrity

Enterprise-grade for both profiles, but critical here because the
executor binary is what strangers send credentials through:

- Pinned, `go.sum`/lockfile-verified dependencies; `govulncheck` and npm
  audit gates in CI; automated update PRs.
- Reproducible static Go builds (`CGO_ENABLED=0`, `-trimpath`); container
  images distroless/scratch, run as non-root, read-only root filesystem.
- **SBOM (SPDX) published per release; artifacts and images signed
  (cosign/Sigstore); SLSA provenance attestation** from the release
  workflow. Deployment templates verify signatures before deploy.
- CI hardening: OIDC-federated cloud deploy credentials (no long-lived
  keys in CI), least-privilege workflow tokens, environment protection on
  release jobs.
- The frontend build pipeline emits a manifest of chunk hashes usable for
  SRI and for out-of-band verification of what a given static host serves.

### 8.10 Security observability

Without an audit table, the platform log drain is the evidentiary record:

- One structured **security event** per query to stdout: timestamp,
  request id, resolved client IP, connector type, target host (host only —
  never full URL with paths/params, never DSN, never credentials, never
  query text at info level), egress-policy decision, outcome class,
  duration, rows returned, and rate-limit/quota state. This mirrors the
  full-mode audit fields so downstream SIEM parsing is shared.
- Prometheus/OTel metrics via the telemetry port: query counts by
  connector/outcome, egress denials (an SSRF-probing spike is the #1
  detection signal), rate-limit rejections, quota exhaustions,
  concurrency saturation, upstream latency histograms.
- Deployment templates include alarm examples: egress-denial spikes,
  sustained 429s, invocation-count budget alarms, error-rate SLO burn.
- Log retention guidance per platform, with a privacy floor (§10.3).

### 8.11 Incident response and kill switches

- `SERVERLESS_KILL_SWITCH=true` → all `/explore/query` return a static
  503 with a documented code; `/info` reports `"disabled": true` so the
  cluster picker greys the endpoint with a reason. One env flip, no
  redeploy, per executor.
- Frontend-side: removing an endpoint from `config.json` (or an entire
  frontend rollback) is a static-host operation, seconds not minutes.
- Because executors are stateless and versioned, rollback = redeploy of
  the previous signed artifact; there is no schema or state to unwind.
- The templates document an abuse-response runbook: identify via security
  events (request id ↔ platform trace), block at platform WAF/rate rules,
  tighten `SERVERLESS_EGRESS_POLICY` to `allowlist`, kill switch as last
  resort.

---

## 9. Platform adapters

The executor remains a plain `http.Handler`; each platform gets a thin
adapter in `cmd/serverless/` plus a hardened template in
`deploy/serverless/<platform>/`. Kernel and guardrail code is identical
everywhere; only the client-IP source (§8.5) and packaging differ.

| Platform | Adapter | Security notes baked into the template |
| --- | --- | --- |
| **AWS Lambda** | `algnhsa` wrapping the router; Function URL (simplest) or API Gateway. `provided.al2023`, arm64, static binary. | Zero-permission execution role (logs only); reserved concurrency cap; Budgets alarm; optional VPC attachment *only* for deliberate `unrestricted` private-explorer deployments, with SGs that cannot reach anything sensitive. |
| **Vercel Functions** | Go runtime `func Handler(w,r)` → router; one catch-all `api/[...path].go`. | Two-project layout (frontend + executor) is the reference for independent hosting; spend limits; `x-vercel-forwarded-for` as IP source. |
| **Azure Functions** | Custom handler — the binary already listens on `FUNCTIONS_CUSTOMHANDLER_PORT`; `host.json` routes everything to it. | Managed identity disabled by default; Front Door/App Gateway rate rules; daily memory-time quota as the billing backstop. |
| **Cloudflare** | (a) Workers via TinyGo→WASM (`syumai/workers`): honest constraints — TinyGo's `reflect`/`net` gaps exclude `pgx`, the cloud SDKs, and `jsonata-go`, so the WASM profile is **rest/graphql only**, advertised via `/info.connectorTypes`. (b) **Recommended:** Cloudflare Containers (or any container host) running the normal binary. Frontend on **Pages** either way. | `CF-Connecting-IP`; Cloudflare rate-limiting rules and Turnstile integrate naturally (§8.5); Workers' fetch-only egress is an extra SSRF backstop in profile (a). |
| **Google Cloud Run** | None — runs the container as-is. The least-friction serverless target for a plain Go binary and the fallback whenever another platform's constraints bite. | Dedicated no-role service account; `concurrency` set low; ingress + Cloud Armor rate policies; request-based billing alarms. |

The capability manifest (§4.3) is what makes heterogeneity tolerable: a
cluster can mix a full-connector Lambda with a REST-only Workers WASM
deployment, and the UI filters per selected executor.

---

## 10. Enterprise operational concerns

### 10.1 Reliability targets and behavior under load

- Suggested SLOs per executor: 99.5% availability on `/info`; p95 query
  overhead (executor time minus upstream time) < 300ms warm.
- Cold starts: static Go binaries keep them modest (~100–300ms on
  Lambda arm64); `/info` probing from the picker keeps commonly-used
  executors warm as a side effect but the design does not depend on
  warmth.
- Overload behavior is defined, not emergent: concurrency cap → 429 +
  `Retry-After`; the frontend backs off and (round-robin mode) tries the
  next healthy executor; quotas exhaust with a clear, user-visible code.

### 10.2 Versioning and compatibility

- `apiVersion` in the manifest is the SPA↔executor contract (§4.3). The
  request/response shapes for `/explore/query` are shared, generated types
  (single source in the repo) — the same `{schema, rows, meta}` dataframe
  wire format as full mode.
- Compatibility policy: an SPA speaks executors of the same major
  version; minor-version skew must be tolerated in both directions
  (unknown manifest fields ignored; new optional QuerySpec fields must not
  be load-bearing for correctness).

### 10.3 Privacy and compliance

- **Data in results is never stored server-side** — the executor is a
  conduit. This makes the GDPR story short but not empty: **client IPs in
  security logs are personal data.** Templates default log retention to
  30 days, document the legitimate-interest basis (abuse prevention), and
  keep IPs out of any metric labels (metrics are aggregate; IPs live only
  in logs).
- Data residency: manual endpoint pinning *is* the residency control —
  a user who must keep queries in the EU pins the EU executor; the picker
  shows regions precisely to make that decision possible. Round-robin
  mode documentation states plainly that queries may execute in any
  configured region.
- Public deployments ship with a template acceptable-use page (linked
  from the shell) and an abuse contact — required hygiene for anything
  internet-facing that makes outbound requests.
- SOC 2-minded operators get: change management via signed releases +
  provenance (§8.9), logical access via the shared-secret gate or
  platform IAM on the deploy path, monitoring via §8.10, and availability
  via §10.1. The serverless profile intentionally has no user data at
  rest to bring into audit scope.

### 10.4 Cost containment as a security property

Economic DoS is a first-class threat (A4). Defaults are chosen for billed
egress: 2K rows / 5MB / 25s (vs. the kernel's 10K / 25MB ceilings), all
advertised in `/info` so the UI can explain truncation. Platform-level
budget alarms and hard concurrency caps appear in **every** template —
the application limiter degrades gracefully across cold starts; the
platform cap is the guarantee.

---

## 11. Testing strategy

- **Egress guard unit suite** as a table-driven test over the full corpus:
  v4/v6 private ranges, IPv4-mapped IPv6, NAT64, metadata IPs and
  hostnames, multi-A-record answers (one public + one private must be
  rejected), redirect-hop re-validation, DSN smuggling cases (unix
  sockets, multi-host, parameter injection). This suite is the security
  regression floor and runs on every commit.
- **Profile-isolation tests:** boot the serverless profile and assert the
  route table is exactly the four routes; assert startup fails on each
  forbidden env var; fuzz `/explore/query` for the `connectionId`
  rejection.
- **Kernel parity tests:** the same QuerySpec fixtures run through both
  profiles against local Postgres/httptest fixtures and must produce
  identical dataframes — proving guardrails live in the kernel, not the
  profile.
- **Frontend:** capability-driven routing snapshots per mode; cluster
  store tests for round-robin advancement over healthy subsets, single
  failover retry, manual-pin no-failover, and manifest-driven type
  filtering; a Playwright run against a local executor covering the
  paste-credentials → run → attribution-visible flow.
- **Platform smoke tests in CI** where emulators exist (SAM local, Azure
  Functions Core Tools, wrangler dev, Cloud Run container); the rest via
  a tagged release pipeline against real scratch accounts.
- Security checks in CI: `govulncheck`, dependency audit, container
  image scan, and a CSP/SRI assertion on the built frontend.

---

## 12. Phasing

1. **Phase 0 — Ports refactor.** Extract kernel + ports; re-wire
   `cmd/server` through `profiles/server` with today's concrete adapters.
   Pure refactor, no behavior change; full test suite is the safety net.
   This phase is what "greenfield redesign" buys: everything after it is
   additive.
2. **Phase 1 — Executor core.** `profiles/serverless`, `cmd/serverless`,
   egress guard (+ its regression suite), ambient-credential lockout,
   fail-closed config, `/info`, security event log. Runs as a plain
   container; testable end-to-end with the existing frontend pointed at it
   via `VITE_API_URL`.
3. **Phase 2 — Frontend profile.** `AppCapabilities`, `/config.json`
   loader, reduced shell/routes, limits surfacing, CSP/SRI in the build,
   single-endpoint support.
4. **Phase 3 — Clusters.** `executorStore`, `/info` probing, round-robin +
   manual picker, failover semantics, per-run executor attribution.
5. **Phase 4 — Platform adapters & hardened templates.** Lambda, Vercel,
   Azure Functions, Cloudflare (containers first; WASM rest/graphql
   profile as stretch), Cloud Run — each with zero-permission identity,
   budget alarms, IP-source pinning, and the abuse runbook.
6. **Phase 5 — Enterprise follow-through.** Shared-secret gate, Turnstile
   hook, manifest signing, release signing/SLSA, FR-17 (user-facing FRD
   with acceptance criteria and screenshots per FRD conventions), and the
   full-mode dividends: OIDC identity adapter, Redis limits adapter.

---

## 13. Open questions

1. **First-party hosted playground?** Allowlist egress + sample datasets
   is the best onboarding funnel, but makes us the operator of §7's
   accepted risks. Needs a product decision plus an abuse-ops commitment,
   not just engineering.
2. **Turnstile default-on for the public templates?** Friction vs. abuse
   posture. Current lean: off by default, one-line enable, and the docs'
   "public deployment checklist" recommends it.
3. **Manifest signing scope.** Ship the header + config slot in Phase 3
   but the signing tooling in Phase 5, or land both together? Current
   lean: slot early, tooling later (§8.7).
4. **Round-robin granularity.** Per-execution (specified) vs. sticky
   per-session with failover. Per-execution matches the request wording;
   a sticky "Automatic" variant can be added later without API changes.
5. **WASM profile investment.** Maintain a TinyGo build (rest/graphql
   only) or document Cloudflare = Pages + Containers and revisit on
   demand? Current lean: the latter for v1; the capability manifest means
   adding it later is invisible to the frontend.
6. **Shared quota store.** Is per-instance quota + platform caps enough
   for v1 (current lean: yes), or do public playgrounds justify a Redis/
   Durable Object quota adapter sooner? The `limits` port keeps this a
   deployment decision rather than a design one.
