# Functional Requirements

This directory contains the functional requirement documents (FRDs) for
Data Explorer. Each document describes **what** the product does from the
user's perspective — not **how** it is built — and captures the intent,
user stories, acceptance criteria, edge cases, and product-value rationale
behind every major feature area.

FRDs live alongside the architecture and security documents in
`docs/` and are meant to be:

- **Authoritative on scope**: the source of truth when discussing whether
  a proposed change fits an existing feature or introduces a new one.
- **Onboarding-friendly**: a new engineer, product manager, designer, or
  customer-facing team member should be able to understand *what a feature
  is* and *why users care* from these documents alone, without reading
  code.
- **Actionable for QA**: every FRD ends with concrete acceptance criteria
  that can be verified by manual test, automated test, or a screenshot
  audit.
- **Evergreen**: they describe user-visible behavior, not implementation
  details, so they stay valid across refactors.

## Related documents

- [`../ARCHITECTURE.md`](../ARCHITECTURE.md) — how the product is built.
- [`../DEVELOPER_GUIDE.md`](../DEVELOPER_GUIDE.md) — local setup and
  contribution workflow.
- [`../SECURITY.md`](../SECURITY.md) — the security model and controls
  that constrain many of the FRs below.
- [`../screenshots/`](../screenshots/) — the visual reference each FRD
  points at when describing a UI surface.

## Index

| ID    | Title                                                                                          | Primary user personas               |
| ----- | ---------------------------------------------------------------------------------------------- | ----------------------------------- |
| FR-01 | [Authentication & Session Management](./FR-01-authentication-and-sessions.md)                   | All users, Admin                    |
| FR-02 | [Role-Based Access Control (RBAC)](./FR-02-role-based-access-control.md)                        | Admin, Security officer             |
| FR-03 | [Data Source Connection Management](./FR-03-connection-management.md)                           | Editor, Admin                       |
| FR-04 | [Integration Catalog](./FR-04-integration-catalog.md)                                           | Editor, Viewer (browse only)        |
| FR-05 | [Connection Health Monitoring](./FR-05-connection-health-monitoring.md)                         | Editor, Admin, SRE                  |
| FR-06 | [Ad-Hoc Data Exploration](./FR-06-ad-hoc-exploration.md)                                        | Analyst, Editor, Viewer             |
| FR-07 | [Visual Workflow Builder (Pipelines)](./FR-07-visual-workflow-builder.md)                       | Analyst, Editor                     |
| FR-08 | [Workflow Scheduling & Automation](./FR-08-workflow-scheduling.md)                              | Editor, Admin, Operations           |
| FR-09 | [Query Result Export & Sharing](./FR-09-query-result-export.md)                                 | Analyst, Viewer                     |
| FR-10 | [Audit Logging & Compliance](./FR-10-audit-logging.md)                                          | Admin, Compliance officer, Auditor  |
| FR-11 | [Observability, Guardrails & Reliability](./FR-11-observability-and-guardrails.md)              | SRE, Operator, Admin                |
| FR-12 | [User Interface, Theming & Accessibility](./FR-12-user-interface-and-accessibility.md)          | All users                           |

## How to read an FRD

Each document follows the same structure so you can jump to the section
you need:

1. **Overview** — one-paragraph summary of the feature.
2. **Product goals** — why this feature exists and what user problem it
   solves.
3. **User personas** — who uses it.
4. **User stories** — "As an X, I want Y, so that Z" statements.
5. **Functional requirements** — numbered `FR-XX.N` requirements, each a
   testable statement of behavior.
6. **UI/UX requirements** — what the user sees and how they interact
   with it.
7. **Acceptance criteria** — measurable, verifiable pass/fail conditions.
8. **Edge cases & error handling** — the non-happy paths.
9. **Non-functional requirements** — performance, security, and
   reliability constraints specific to this feature.
10. **Market context & differentiation** — how comparable products
    (Retool, n8n, Grafana, Metabase, Hex, Postman, Airbyte, ...) solve
    the same problem and where Data Explorer is intentionally different.
11. **Future enhancements** — non-binding ideas that came up during
    research but are out of scope for the current implementation.

## How to change an FRD

FRDs are updated the same way as any other doc in the repository — via a
pull request. When a change to code changes user-visible behavior, the
corresponding FRD should be updated in the same PR so the two stay in
sync.

When adding a new feature area, create a new `FR-NN-slug.md` file using
the numbering above and add it to the index table.
