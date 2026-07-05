# internal/scheduler

## What this package does

`internal/scheduler` is a **single in-process polling loop** that automatically executes cron-scheduled workflows. It runs inside the same Go binary as the API server — no separate worker process, no queue, no external dependency.

## How it works

```
Every 15 seconds (PollInterval):
  SELECT id FROM workflows
  WHERE schedule_enabled = true
    AND schedule_next_run <= now()
  ORDER BY schedule_next_run ASC

For each due workflow:
  workflow.Service.Execute(ctx, workflowID, triggeredBy="scheduler")
  UPDATE workflows SET schedule_next_run = <next cron occurrence>
```

The due-check query hits a **partial index** on `(schedule_next_run) WHERE schedule_enabled = true` — it is cheap regardless of the total number of workflows. `schedule_next_run` is pre-computed by `workflow.Service.SetSchedule` using `robfig/cron/v3`'s next-occurrence calculation, so the scheduler never evaluates a cron expression at runtime.

## Execution path

A scheduled workflow runs through the **exact same `workflow.Service.Execute` path** as a manual "Run" click or API call:
- Same DAG engine
- Same `MaxExecutionDuration` (2-minute context timeout)
- Same `workflow_executions` row written
- Same per-node guardrails (row caps, timeouts)

The only difference: `TriggeredBy = "scheduler"` (a sentinel string, not a user ID).

## Lifecycle

- `scheduler.New(workflowService)` — constructs the scheduler
- `scheduler.Start(ctx)` — starts the poll loop in a goroutine; respects context cancellation
- `scheduler.Stop()` — signals the loop to stop; called during graceful shutdown before the HTTP server drains

## Architecture decisions (ADRs)

| Decision | Rationale |
|---|---|
| In-process, not a separate worker | "One binary to deploy" simplicity; consistent with the overall architecture principle |
| Poll loop, not push/event | Simplest mechanism; no external queue, lock service, or coordination needed |
| Pre-computed `schedule_next_run` | Cheap due-check query with a partial index; no live cron parsing on every tick |
| `workflow.Service.Execute` reused | Identical execution semantics for scheduled and manual runs; no separate code path to diverge |
| `TriggeredBy = "scheduler"` | No synthetic system user; the execution history clearly shows the trigger |

## Scope and responsibilities

- Poll for due workflows at a configurable interval.
- Invoke `workflow.Service.Execute` for each due workflow.
- Update `schedule_next_run` after each execution.
- Start and stop cleanly with the application lifecycle.

## Limitations and todos

- [ ] In-process state means running multiple replicas will cause duplicate executions for the same due workflow; a distributed lock (e.g. `pg_try_advisory_lock`) or a leader-election mechanism is needed before horizontal scaling.
- [ ] Failed scheduled executions are not retried automatically; the next scheduled time is still advanced.
- [ ] No alerting or notification when a scheduled workflow consistently fails.
- [ ] `PollInterval` (15s) is fixed; high-frequency schedules (sub-minute cron) cannot be supported.
- [ ] No schedule history or last-run tracking beyond the `workflow_executions` table.
