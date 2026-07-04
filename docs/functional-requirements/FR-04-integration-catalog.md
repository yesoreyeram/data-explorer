# FR-04: Integration Catalog

## Overview

The **Integration Catalog** is a first-party, static registry of ~20
well-known third-party APIs (GitHub, Stripe, Slack, Twilio, HubSpot,
Notion, Airtable, Salesforce, Zendesk, PagerDuty, Linear, Contentful,
Hasura, Discord, OpenAI, Mailgun, Shopify, and SendGrid, among others).
Picking an entry prefills a new REST or GraphQL connection's type, base
URL / endpoint, auth scheme, and any non-secret auth configuration —
saving the "what's the base URL and auth type for X?" lookup. The
catalog is **read-only** and does **not** proxy any external service.

## Product goals

- **Reduce time-to-first-connection.** The most tedious part of adding
  a new API integration is finding the base URL and figuring out which
  of ten auth schemes it uses. The catalog gets that to a single click.
- **Reduce configuration errors.** A prefilled base URL is one fewer
  place a typo turns into a "network_unreachable" error the user has
  to debug.
- **Never leak credentials into the catalog.** The catalog carries no
  credentials, ever. It supplies the *shape* of a connection (type +
  URL + auth type + non-secret auth params), not the *identity* to
  use.
- **Never depend on a live third party.** The catalog is authored by
  hand in this repository; there is no external directory to fetch or
  cache, so a network outage of any third-party registry cannot break
  connection creation.
- **Extensible by contribution.** New entries land as pull requests
  against `internal/catalog/seed.go`.

## User personas

| Persona     | Description                                                                                                    |
| ----------- | -------------------------------------------------------------------------------------------------------------- |
| Editor      | Uses the catalog to bootstrap a new REST/GraphQL connection quickly.                                           |
| Analyst     | Browses the catalog to see which integrations are known to the platform (e.g. "does GitHub work out of the box?"). |
| Contributor | Adds an entry for a widely-used integration through a PR.                                                     |

## User stories

- **US-04.1** As an editor, I want to search a list of well-known
  integrations by name so that I don't have to look up the vendor's
  base URL myself.
- **US-04.2** As an editor, I want the catalog to filter by category
  (Developer tools / Payments / CRM / Messaging / Email / etc.) so
  that I can find integrations for a specific use case even when I
  don't remember the exact product name.
- **US-04.3** As an editor, I want to filter by connection type (REST
  vs GraphQL) so that I can quickly find the GraphQL entries when I
  need one.
- **US-04.4** As an editor, I want picking a catalog entry to leave
  every credential field blank so that I still have to explicitly type
  or paste the credential — the catalog cannot silently plant one on
  me.
- **US-04.5** As an editor, I want a "documentation" link on each
  entry that opens the vendor's auth docs so that I know exactly where
  to go to fetch the credential the entry expects.
- **US-04.6** As a contributor, I want a documented, small file to add
  a new entry to so that expanding the catalog is a targeted, easily
  reviewed change.

## Functional requirements

### FR-04.1 — Catalog data shape

Each catalog **entry** SHALL contain, at minimum:

- `id` — stable identifier (kebab-case, unique across the catalog).
- `name` — human-readable product name.
- `description` — one-line description of what the entry is for.
- `category` — one of: Developer tools, Payments, Messaging,
  Communications, Email, CRM, Commerce, Productivity, Customer support,
  Incident management, Project management, CMS, Database/API, Social,
  AI. (New categories are added to the enum as needed.)
- `type` — either `rest` or `graphql` (must match the FR-03.1
  connection-type vocabulary).
- `baseUrl` (for REST) or `endpoint` (for GraphQL). The value MAY
  contain `{placeholder}` tokens for values that vary per tenant
  (e.g. `{shop}`, `{subdomain}`, `{instance}`).
- `authType` — one of the FR-03.8 auth types.
- `authConfig` — non-secret auth parameters (e.g. `apiKeyHeader:
  "X-Shopify-Access-Token"` for Shopify).
- `docsUrl` — a link to the vendor's authentication docs.

The catalog SHALL NEVER contain a credential of any kind.

### FR-04.2 — Read-only, static, in-process

The catalog data SHALL live in `internal/catalog/seed.go` (authored by
hand). At runtime it SHALL be filtered entirely in memory — no database
lookup, no external HTTP call.

- `GET /api/v1/catalog` SHALL return the full catalog list, optionally
  filtered by:
  - `q` (case-insensitive substring match against `name` and
    `description`)
  - `category`
  - `type`
- The endpoint SHALL reuse the `connections:read` permission rather
  than a new one, since the catalog is a convenience layered on
  connection creation.

### FR-04.3 — Prefill new connection

The **Browse catalog** button on the Connections page SHALL open a
searchable catalog browser modal. Clicking an entry:

1. Closes the catalog browser modal.
2. Opens the `ConnectionFormModal` in **create** mode with:
   - `type` set to the entry's `type`.
   - `baseUrl` / `endpoint` set to the entry's URL, verbatim,
     including any `{placeholder}` tokens (the user is expected to
     replace them).
   - `authType` set to the entry's `authType`.
   - `authConfig` fields (e.g. token URL, API-key header name)
     populated from the entry.
   - **All secret fields left blank.**
   - The form's help area SHOULD surface a link labelled **"Get
     credentials from `<vendor>`'s auth docs"** pointing at
     `docsUrl`.
3. From here the user proceeds through the standard "create
   connection" flow (see [FR-03.4](./FR-03-connection-management.md)).

### FR-04.4 — Placeholder handling

Base URLs containing `{placeholder}` tokens SHALL NOT be validated
until the user submits. Placeholder detection is a client-side
convenience:

- The form MAY highlight a base URL containing an unreplaced
  `{placeholder}` and show an inline hint "**Replace the highlighted
  placeholder(s) with your tenant-specific value.**"
- The server's URL validator SHALL reject any URL still containing an
  unreplaced `{placeholder}` at submit time with a clear message.

### FR-04.5 — Adding a new entry

A contributor SHALL be able to add a new catalog entry by:

1. Appending a new `Entry{...}` literal to the `seed` slice in
   `internal/catalog/seed.go`.
2. Verifying the `type` and `authType` match the existing vocabulary
   (else the connection form will not know how to render the
   prefilled connection).
3. Adding a test if the entry uses a new placeholder pattern or
   auth-type combination.

There SHALL be no admin UI for adding catalog entries at runtime.
Catalog changes are code-reviewed, not runtime configuration — this is
deliberate and matches the "no external registry dependency" design
constraint.

## UI/UX requirements

### Catalog browser modal

Reference screenshot: [`docs/screenshots/15-catalog-browser.png`](../screenshots/15-catalog-browser.png).

- Opens from a **Browse catalog** button on the Connections page (see
  FR-03).
- Modal title: **"Browse integration catalog"**.
- Top toolbar (in order): a **search input** (placeholder "**Search
  integrations…**"), a **Type** drop-down (`All types` / `REST` /
  `GraphQL`), and a **Category** drop-down (`All categories` + every
  category seen in the seed data).
- Body: a list of catalog rows. Each row is a full-width clickable
  button showing:
  - Product **name** (large, near-monochrome ink accent).
  - Category badge (neutral).
  - **Type** pill (REST / GraphQL).
  - **Description** (subdued).
- Empty state (spans full body): **"No integrations match your
  search."**
- The catalog SHOULD be alphabetically sorted by `name` within a
  category, and categories SHOULD be rendered in a stable order.
- The modal SHALL be dismissable with **Escape**, the **X** button, or
  by clicking outside its card.

### Prefilled form

Reference screenshot: [`docs/screenshots/17-catalog-prefilled-form.png`](../screenshots/17-catalog-prefilled-form.png).

- After a catalog pick, the connection form MAY display a subtle
  banner near the top: **"Prefilled from the `<name>` catalog
  entry."** with a link to `docsUrl`.
- The **Type** drop-down SHALL be disabled to prevent an accidental
  change that would invalidate the prefill (the user would need to
  cancel and open the form again for a different type).

## Acceptance criteria

- [ ] `GET /api/v1/catalog` (with `connections:read`) returns at least
  10 entries, sorted deterministically.
- [ ] `GET /api/v1/catalog?q=github` returns only entries whose name or
  description matches, in a case-insensitive substring compare.
- [ ] `GET /api/v1/catalog?type=graphql` returns only GraphQL entries.
- [ ] `GET /api/v1/catalog?category=CRM` returns only CRM entries.
- [ ] `GET /api/v1/catalog` requires `connections:read`; without it
  the API returns `403`.
- [ ] Clicking a catalog entry closes the browser modal, opens the
  connection form, and populates: `type`, `baseUrl` (or `endpoint`),
  `authType`, and any non-secret `authConfig` fields.
- [ ] Clicking a catalog entry NEVER populates a secret field.
- [ ] Attempting to save a connection whose URL still contains a
  `{placeholder}` fails with a clear inline message.
- [ ] The Type drop-down is disabled after a catalog prefill.
- [ ] No entry in `internal/catalog/seed.go` contains anything that
  looks like a credential (verified by a static test that scans for
  fields named `apiKey`, `token`, `password`, `secret`).
- [ ] Every catalog entry has a `docsUrl` that is a well-formed
  `https://` URL.
- [ ] Adding a new catalog entry through a PR requires no schema
  migration and no external dependency.

## Edge cases & error handling

- **Catalog entry with an obsolete base URL.** The catalog is
  hand-authored; if a vendor changes their base URL, the entry needs
  updating in a follow-up PR. The prefilled URL is a hint, not a
  contract — the user can still change it in the connection form.
- **Duplicate `id`.** SHALL be caught by a unit test in
  `internal/catalog/service_test.go`.
- **User without `connections:write` who somehow reaches the browser
  modal.** The **Browse catalog** button is gated behind
  `connections:write` (see FR-03.7), so this should not happen; if it
  does, picking an entry still opens the connection form which itself
  gates its Save button on `connections:write`.
- **Entry whose auth type requires a special sub-form the frontend
  doesn't render.** SHALL be prevented by a compile-time test that
  every entry's `authType` matches an entry in `AuthTypeFields.tsx`'s
  registry.
- **Case sensitivity in search.** The search SHALL be
  case-insensitive on both `name` and `description`.

## Non-functional requirements

- **Latency.** The catalog endpoint SHALL respond in < 20 ms — it is
  in-memory data.
- **Offline safety.** The system SHALL be able to serve the catalog
  even when the server has no outbound network at all.
- **Correctness maintenance cost.** The seed is expected to drift over
  time (vendors change auth). Corrections land as PRs; no runtime
  correctness dependency on any external service.

## Market context & differentiation

| Product   | Catalog / template model                                                                                             |
| --------- | -------------------------------------------------------------------------------------------------------------------- |
| Postman   | "Public API Network" — a massive live registry, tightly integrated with cloud-only workspace.                        |
| n8n       | 400+ built-in "nodes" for specific integrations; new nodes require Node.js code.                                     |
| Retool    | Dozens of pre-built integrations with a rich UI; new integrations require Retool platform work.                      |
| Zapier    | Enormous curated app directory; each integration is a hand-built Zapier app.                                         |
| Airbyte   | Registry of hundreds of connector Docker images; each connector is its own repository.                               |

**Where Data Explorer is intentionally different.** Rather than build
per-vendor code (a la Zapier or n8n), Data Explorer uses a *shape-only*
catalog on top of a *generic* REST/GraphQL connector. That is:

- **Much smaller surface area.** No per-vendor code, no per-vendor
  security review, no per-vendor breakage when a vendor changes an
  endpoint.
- **Any REST or GraphQL API works.** If the entry doesn't exist yet,
  the user just creates a plain REST/GraphQL connection. The catalog
  is an accelerator, not a gate.
- **Zero external runtime dependency.** No live registry to be down,
  no version-negotiation with a remote schema — the catalog is
  compiled into the binary.

The trade-off is that Data Explorer will not, for example, natively
know how to paginate GitHub's issues API. That's a workflow-node
concern (see [FR-07](./FR-07-visual-workflow-builder.md)) and a
pagination-config concern (`PaginationFields.tsx`), not a
catalog concern.

## Future enhancements (out of scope for this FR)

- **Vendor-specific query snippets.** Each catalog entry could carry
  starter queries (e.g. GitHub "list repos I own", Stripe "list
  charges in the last 30 days") that the connection form or Explore
  page could offer as one-click starting points.
- **Community-maintained catalog file.** A separate `catalog.yaml`
  loaded at boot for private forks to add local entries without
  editing Go code, while the built-in seed remains code-reviewed.
- **Health checks per entry template.** For each entry, a "known
  good" health-check probe (e.g. `GET /v1/account` for Stripe) that
  the connection Test button uses when no custom path is configured.
- **Popularity / suggested order.** Order entries by how many
  connections in this deployment use each entry.

## Cross-references

- Implementation: `backend/internal/catalog/`,
  `backend/internal/api/handlers/catalog.go`,
  `frontend/src/pages/connections/CatalogBrowserModal.tsx`.
- Related FRs: [FR-03 Connection Management](./FR-03-connection-management.md).
- Architecture: [`../ARCHITECTURE.md`](../ARCHITECTURE.md) section
  "Integration catalog: prefilling, not proxying".
