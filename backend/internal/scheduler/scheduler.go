// Package scheduler drives cron-scheduled workflow execution. It is a
// single in-process polling loop, not a separate worker process or queue -
// consistent with the rest of the app's "one Go binary" simplicity
// principle (see ARCHITECTURE.md's "Scaling beyond a single request"): a
// workflow run is already bounded (workflow.MaxExecutionDuration) and cheap
// enough that a 15s poll tick comfortably keeps up with any realistic
// number of scheduled workflows.
package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/yesoreyeram/data-explorer/backend/internal/domain"
	"github.com/yesoreyeram/data-explorer/backend/internal/workflow"
)

// PollInterval is how often the scheduler checks for due workflows. It, not
// cron expression granularity, is the effective floor on how precisely a
// schedule fires - a "* * * * *" (every minute) workflow can fire up to
// PollInterval late.
const PollInterval = 15 * time.Second

// TriggeredBy is the sentinel WorkflowExecution.TriggeredBy value for
// scheduler-initiated runs, distinguishing them from a real user id in the
// execution history and audit trail.
const TriggeredBy = "scheduler"

type Scheduler struct {
	workflows *workflow.Service
	log       *slog.Logger
}

func New(workflows *workflow.Service, log *slog.Logger) *Scheduler {
	return &Scheduler{workflows: workflows, log: log}
}

// Run blocks, polling every PollInterval until ctx is cancelled. Intended
// to be started in its own goroutine from main.
func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

func (s *Scheduler) tick(ctx context.Context) {
	due, err := s.workflows.DueSchedules(ctx)
	if err != nil {
		s.log.Error("scheduler: failed to list due workflows", "error", err)
		return
	}
	for _, wf := range due {
		s.runOne(ctx, wf)
	}
}

func (s *Scheduler) runOne(ctx context.Context, wf domain.Workflow) {
	log := s.log.With("workflow_id", wf.ID, "workflow_name", wf.Name)

	_, _, err := s.workflows.Execute(ctx, wf.ID, TriggeredBy)
	if err != nil {
		// A failed *workflow* run is still a recorded execution (see
		// workflow.Service.Execute) - this log line is only for failures the
		// execution history won't otherwise capture, e.g. the workflow having
		// been deleted between the due-list query and now.
		log.Warn("scheduler: workflow run did not complete cleanly", "error", err)
	}

	now := time.Now()
	next, err := workflow.NextRun(wf.ScheduleCron, now)
	if err != nil {
		// The cron expression was valid when saved (SetSchedule validates it)
		// so this shouldn't happen - but if it does, disable rather than
		// tick forever on a schedule we can no longer compute.
		log.Error("scheduler: could not compute next run, disabling schedule", "error", err)
		if _, disableErr := s.workflows.SetSchedule(ctx, wf.ID, wf.ScheduleCron, false); disableErr != nil {
			log.Error("scheduler: failed to disable broken schedule", "error", disableErr)
		}
		return
	}

	if err := s.workflows.MarkScheduleRun(ctx, wf.ID, now, &next); err != nil {
		log.Error("scheduler: failed to advance schedule", "error", err)
	}
}
