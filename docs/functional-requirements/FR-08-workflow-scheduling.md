# FR-08 — Workflow Scheduling & Automation

## Overview

Workflow **scheduling** allows a saved workflow to run automatically on a
recurring cadence expressed as a standard 5-field cron expression
(minute, hour, day-of-month, month, day-of-week). Data Explorer runs the
scheduler in-process — no external Airflow / Argo / cron server — so a
single deployment of the platform is complete out of the box. Scheduled
runs execute using the same engine, guardrails, and audit trail as
manual runs, so any pipeline the user has proved works on **Run now**
will behave identically at 03:00.

Scheduling is a low-friction "set and forget" surface: users pick a
cadence from a list of common presets (every 5 min, hourly, daily,
weekly) or paste an exact cron expression, see the next N run times
computed live, and toggle the schedule on/off. When the scheduler
picks up the workflow, it invokes the workflow engine on a background
worker; the resulting `workflow_executions` row is indistinguishable
from a manual run except that `initiated_by = "scheduler"`.

## Product goals

- Turn a working, hand-run pipeline into an automated one in **≤2
  clicks** — no scripting, no external scheduler, no infrastructure
  change.
- Make schedule syntax **standard cron** so users can copy expressions
  from other systems and paste them in unchanged.
- Show the user *when* the next runs will actually fire — no guessing.
- Fail loudly if the schedule becomes invalid (e.g. a referenced
  connection is deleted) rather than silently skipping executions.
- Preserve one uniform observability, audit, and guardrail surface
  across manual and scheduled runs — a scheduled run cannot bypass any
  safety rail.

## User personas

| Persona          | Description                                                                                        |
| ---------------- | -------------------------------------------------------------------------------------------------- |
| **Analyst**      | Wants a "run this dashboard-feeder every morning" cadence with zero infrastructure.                |
| **Editor**       | Owns a set of workflows and needs to schedule them without asking Ops for cron jobs on a VM.       |
| **Ops engineer**| Runs periodic health/metrics rollups every 5 minutes.                                              |
| **Admin**        | Wants to see every scheduled workflow and disable / re-enable one when needed.                     |
| **Auditor**      | Verifies that automated runs are recorded with the same fidelity as manual ones.                   |

## User stories

- **US-08.1** As an editor, I want to attach a cron schedule to a saved
  workflow, so it runs on a cadence without a cron job on a server.
- **US-08.2** As an analyst, I want to pick from common presets ("every
  hour", "every day at 06:00") rather than remember cron syntax, so I
  don't fumble the schedule.
- **US-08.3** As an editor, I want to paste an exact cron expression
  for less common cadences ("every weekday at 09:15"), so I don't have
  to bend my needs to a preset list.
- **US-08.4** As an editor, I want to see the next 5 firing times of a
  schedule before I save it, so I can catch a mistake immediately.
- **US-08.5** As an admin, I want a page listing every scheduled
  workflow with its cadence and last-fired time, so I can spot idle
  or misbehaving pipelines.
- **US-08.6** As an editor, I want to pause a schedule with a toggle
  without editing the cron expression, so I can suspend a pipeline
  during a maintenance window.
- **US-08.7** As an auditor, I want a scheduled run to leave the same
  audit record as a manual run, so my log is uniform.
- **US-08.8** As an operator, I want the platform to skip (not queue)
  overlapping runs of the same workflow, so long-running pipelines
  don't stack up.

## Functional requirements

### FR-08.1 — Cron schedule field

Every workflow SHALL have a nullable `schedule` string field. When
non-null, it SHALL be a valid **standard 5-field cron expression**
(minute, hour, day-of-month, month, day-of-week) as parsed by
`ParseStandard` from `robfig/cron`.

### FR-08.2 — Preset options

The UI SHALL offer a preset list. At a minimum:

- Every 5 minutes (`*/5 * * * *`)
- Every 15 minutes (`*/15 * * * *`)
- Every hour (`0 * * * *`)
- Every day at 06:00 (`0 6 * * *`)
- Every weekday at 09:00 (`0 9 * * 1-5`)
- Every Sunday at 00:00 (`0 0 * * 0`)
- Custom (free-form cron input)

Presets SHALL be labelled in human terms; the corresponding cron
expression SHALL be visible for transparency.

### FR-08.3 — Validation and preview

When the user enters or selects an expression:

- The UI SHALL validate the expression client-side and again
  server-side.
- The UI SHALL show the **next 5 firing times** computed from now,
  formatted in the user's timezone with an ISO 8601 tooltip.
- Invalid expressions SHALL show an inline error naming the failing
  field.

### FR-08.4 — Time zone

Cron expressions SHALL be evaluated in **UTC** on the server. The UI
SHALL show next-run times converted to the browser's local timezone
with the UTC value in a tooltip. This SHALL be clearly documented on
the schedule modal.

### FR-08.5 — Scheduler poll loop

The scheduler SHALL run inside the same server process:

- On startup, it loads every scheduled workflow.
- On a periodic tick (default 30 seconds), it computes each schedule's
  next-run time and, if the next-run time has elapsed since the last
  dispatch, dispatches the workflow to the engine.
- The scheduler SHALL persist per-workflow `last_scheduled_run_at` to
  prevent duplicate dispatches after restarts.

### FR-08.6 — Overlap policy

If a workflow's previous run is still in flight when the next fire
time arrives, the scheduler SHALL **skip** (not queue) the second
run and SHALL record a skipped-schedule event with reason
`overlap` on the workflow's history.

### FR-08.7 — Pause / resume

The UI SHALL expose a **Pause schedule** toggle. Pausing SHALL clear
the schedule (set to null) without deleting the workflow; resuming
SHALL restore the previously-saved cron expression from an audit
event.

### FR-08.8 — Ownership and permissions

- `workflows:write` gates setting or clearing a schedule.
- `workflows:execute` is *not* required to schedule a workflow — a
  user who can write a workflow can schedule it, and the scheduler
  runs with a system principal.
- The scheduler's identity SHALL be recorded as `initiated_by =
  "scheduler"` on every scheduled run.

### FR-08.9 — Scheduled-run audit trail

Each scheduled run SHALL emit the same audit events as a manual run
(`workflows.run_started`, `workflows.run_completed`) plus a
`workflows.scheduled_dispatch` event with `workflow_id`, `dispatched_at`,
and `next_run_at`. Skipped runs (overlap) SHALL emit
`workflows.scheduled_skipped` with reason.

### FR-08.10 — Visual indicators

The Workflows list SHALL show, for every scheduled workflow, a
"Scheduled" badge with the cadence description ("every day at
06:00 UTC") and the last outcome (success / failure) with the
timestamp.

### FR-08.11 — Guardrails apply uniformly

All the execution guardrails from FR-07 (2 min run timeout, 60s node
timeout, 100K row cap per node, 200 nodes / 500 edges, per-connection
rate limits) SHALL apply identically to scheduled runs. Scheduled
runs SHALL NOT bypass any safety rail.

## UI/UX requirements

- The schedule modal is opened from the workflow builder ("Schedule…"
  button) or the workflows list row action.
- Modal contents (see [`docs/screenshots/25-workflow-schedule-modal.png`](../screenshots/25-workflow-schedule-modal.png)):
  - Radio list of presets + "Custom" option.
  - Free-form cron text input, disabled unless "Custom" is chosen.
  - Live-computed cron expression preview.
  - "Next 5 runs" list.
  - Save / Cancel buttons.
- A workflow that has a schedule attached shows a **Scheduled** badge
  on its builder page and on the Workflows list — see
  [`docs/screenshots/26-workflow-scheduled-badge.png`](../screenshots/26-workflow-scheduled-badge.png)
  and [`docs/screenshots/27-workflows-list-scheduled.png`](../screenshots/27-workflows-list-scheduled.png).
- Pausing a schedule updates the badge to show "Paused" with a
  neutral color.
- Errors during scheduled runs surface on the workflow detail page's
  run-history table exactly as manual failures do.

## Acceptance criteria

- [ ] Saving a workflow with the "Every hour" preset persists the
  schedule as `0 * * * *`.
- [ ] Saving a workflow with an invalid cron expression returns 400
  from the API and shows an inline error in the modal.
- [ ] The "Next 5 runs" preview updates within 200ms of the
  expression changing.
- [ ] A scheduled workflow whose next-run time has elapsed produces a
  `workflow_executions` row with `initiated_by = "scheduler"`.
- [ ] A workflow whose previous run is still in flight when the next
  fire time arrives records a `workflows.scheduled_skipped` audit event
  with reason `overlap` and does not create a second execution row.
- [ ] Clearing the schedule via the toggle sets the workflow's
  `schedule` column to NULL and stops future dispatches.
- [ ] A user without `workflows:write` cannot modify the schedule of a
  workflow (form is read-only).
- [ ] After a server restart the scheduler resumes without
  double-firing any workflow whose fire time was already handled.
- [ ] Presets show their cron expression next to the label to keep
  users honest.
- [ ] The scheduler evaluates cron expressions in UTC and the UI
  clearly notes that in the modal.

## Edge cases & error handling

- **Clock drift**: The scheduler tolerates a bounded drift by using
  the server clock; if the process pauses for longer than one tick,
  it dispatches at most one "catch-up" run per schedule, not one per
  missed tick.
- **DST transitions**: Because schedules are stored and evaluated in
  UTC, DST does not affect fire time; the UI documents this on the
  modal.
- **Feb 29 / rare dates**: Standard cron semantics apply; e.g.
  `0 0 29 2 *` fires only in leap years.
- **Very short cadence**: A `*/1 * * * *` cadence is accepted but a
  banner on the modal warns "This will run every minute — ensure
  your source respects rate limits."
- **Deleted workflow**: Deleting a scheduled workflow immediately
  removes it from the scheduler's in-memory table so no orphan
  dispatch happens.
- **Deleted connection referenced by a scheduled workflow**: The
  next scheduled run fails with `invalid_config`; the audit log
  records the failure; the schedule is not automatically paused —
  the user must decide whether to fix or pause.
- **Overlapping runs beyond overlap policy**: A pathological
  workflow that always exceeds its own cadence emits `overlap`
  skips every fire; the UI surfaces the last-skipped reason on the
  workflow detail page.
- **Server crash mid-run**: A run that was in progress at crash
  time SHALL be marked `failed` with `error_code = timeout` on
  reboot; the scheduler resumes normally.
- **Concurrent schedule edits**: The workflow's `updated_at`
  optimistic lock (see FR-07) applies; a second save with a stale
  timestamp is rejected with 409.

## Non-functional requirements

- **Latency**: Scheduler tick latency (fire → engine dispatch) SHALL
  be under 1 second at p95.
- **Reliability**: The scheduler SHALL NOT drop scheduled runs due to
  transient database errors — it retries with exponential backoff.
- **Concurrency**: The scheduler SHALL support at least 500
  concurrently scheduled workflows without missed dispatches.
- **Persistence**: Scheduler state (`last_scheduled_run_at`) SHALL be
  persisted so restart safety holds.
- **Observability**: Scheduler SHALL emit metrics
  `scheduler_dispatch_total`, `scheduler_skip_total{reason}`,
  `scheduler_lag_seconds` (measured as `now - fire_time` at dispatch).
- **Isolation**: A single misbehaving scheduled workflow SHALL NOT
  starve the scheduler; runs execute on a bounded worker pool.

## Market context & differentiation

| Product              | Scheduling model                                    | Notes                                                                              |
| -------------------- | --------------------------------------------------- | ---------------------------------------------------------------------------------- |
| **Airflow**          | Cron + external Python DAG                          | Powerful; requires standing up a scheduler / worker cluster.                       |
| **n8n**              | Cron trigger node                                   | In-process; excellent UX; SaaS or self-host.                                       |
| **Zapier**           | Every-N-minutes / cron                              | SaaS-only; opaque runtime.                                                         |
| **GitHub Actions**   | Cron schedule in YAML                               | Cron-based but ties to code repo, not to data resources.                           |
| **AWS EventBridge**  | Cron / rate expressions triggering AWS targets       | Cloud-native; AWS-only; boilerplate to wire targets.                               |
| **Retool Workflows** | Cron schedule attached to a workflow                | In-product; SaaS-first.                                                            |
| **Kestra**           | Cron + declarative YAML                             | Powerful; requires learning Kestra's YAML DSL.                                     |
| **Metabase Pulses**  | Question-schedule with email/slack delivery         | Focused on report delivery, not on generic pipeline scheduling.                    |

Data Explorer's differentiators for scheduling:

- **In-process, zero-infrastructure.** Cron schedules run on the same
  process that authored them — no external scheduler service.
- **Standard cron everywhere.** No product-specific DSL — copy an
  expression from anywhere and it works.
- **Live "next 5 runs" preview.** Users see what their expression
  actually does before saving.
- **Overlap-safe by default.** No queue backpressure surprises when a
  workflow slows down.
- **Guardrails apply identically to scheduled and manual runs.**
- **Uniform audit trail.** Scheduled runs are indistinguishable from
  manual runs in the audit log except for one attribution field.

## Future enhancements (out of scope)

- Time-zone-aware schedules (per-workflow tz override).
- Fire-once-at-a-specific-timestamp schedules.
- Per-schedule retry policy on failure.
- Backfill mode (compute past missed runs).
- Calendar-driven exclusions (holiday skipping).
- Distributed scheduling across multiple server processes with lease
  election.
- Alerting hooks on failed scheduled runs (email/Slack).
- Schedule "cool-off" after N consecutive failures.

## Cross-references

- [FR-07 Visual Workflow Builder](./FR-07-visual-workflow-builder.md) —
  the workflow resource being scheduled.
- [FR-10 Audit Logging & Compliance](./FR-10-audit-logging.md) —
  the audit stream scheduled runs write to.
- [FR-11 Observability, Guardrails & Reliability](./FR-11-observability-and-guardrails.md) —
  the scheduler's metrics and safety rails.
