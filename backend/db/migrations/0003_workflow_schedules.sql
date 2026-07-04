-- Cron-based recurring workflow execution.

ALTER TABLE workflows ADD COLUMN schedule_cron    TEXT NOT NULL DEFAULT '';
ALTER TABLE workflows ADD COLUMN schedule_enabled BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE workflows ADD COLUMN schedule_next_run TIMESTAMPTZ;
ALTER TABLE workflows ADD COLUMN schedule_last_run TIMESTAMPTZ;

-- The scheduler polls for due workflows; a partial index keeps that lookup
-- cheap regardless of how many workflows exist, since most won't be scheduled.
CREATE INDEX idx_workflows_schedule_due ON workflows(schedule_next_run) WHERE schedule_enabled;

-- triggered_by was a FK to users(id), which can't represent a scheduler-
-- initiated run (there's no acting user). Relax it to a plain string, same
-- as audit_logs.actor_id already is - "scheduler" is a valid value alongside
-- a real user id, with no synthetic user row required.
ALTER TABLE workflow_executions DROP CONSTRAINT workflow_executions_triggered_by_fkey;
ALTER TABLE workflow_executions ALTER COLUMN triggered_by TYPE TEXT;
ALTER TABLE workflow_executions ALTER COLUMN triggered_by SET DEFAULT '';
